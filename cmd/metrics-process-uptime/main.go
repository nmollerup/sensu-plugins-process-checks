package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sensu/sensu-go/types"
	"github.com/sensu/sensu-plugin-sdk/sensu"
)

// Config represents the metrics plugin config.
type Config struct {
	sensu.PluginConfig
	Pattern string
	Scheme  string
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "metrics-process-uptime",
			Short:    "Sensu metrics for process uptime",
			Keyspace: "sensu.io/plugins/metrics-process-uptime/config",
		},
	}

	options = []sensu.ConfigOption{
		&sensu.PluginConfigOption[string]{
			Path:      "pattern",
			Argument:  "pattern",
			Shorthand: "p",
			Default:   "",
			Usage:     "Process pattern to match",
			Value:     &plugin.Pattern,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "scheme",
			Argument:  "scheme",
			Shorthand: "s",
			Default:   "",
			Usage:     "Metric naming scheme (default: hostname.process.uptime)",
			Value:     &plugin.Scheme,
		},
	}
)

func main() {
	check := sensu.NewCheck(&plugin.PluginConfig, options, checkArgs, executeCheck, false)
	check.Execute()
}

func checkArgs(_ *types.Event) (int, error) {
	if plugin.Pattern == "" {
		return sensu.CheckStateCritical, fmt.Errorf("pattern must be specified")
	}
	
	if plugin.Scheme == "" {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "unknown"
		}
		plugin.Scheme = fmt.Sprintf("%s.process.uptime", hostname)
	}
	
	return sensu.CheckStateOK, nil
}

func executeCheck(_ *types.Event) (int, error) {
	cmd := exec.Command("ps", "aux")
	output, err := cmd.Output()
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("failed to get processes: %v", err)
	}

	re, err := regexp.Compile(plugin.Pattern)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("invalid pattern: %v", err)
	}

	timestamp := time.Now().Unix()
	lines := strings.Split(string(output), "\n")
	found := false

	for i, line := range lines {
		if i == 0 || line == "" {
			continue
		}

		if !re.MatchString(line) {
			continue
		}

		found = true
		fields := strings.Fields(line)
		if len(fields) < 11 {
			continue
		}

		pid := fields[1]
		cmdName := sanitizeMetricName(fields[10])

		// Get process uptime
		uptime, err := getProcessUptime(pid)
		if err != nil {
			continue
		}

		fmt.Printf("%s.%s.%s %d %d\n", plugin.Scheme, cmdName, pid, uptime, timestamp)
	}

	if !found {
		return sensu.CheckStateWarning, fmt.Errorf("no processes matched pattern '%s'", plugin.Pattern)
	}

	return sensu.CheckStateOK, nil
}

func getProcessUptime(pid string) (int64, error) {
	statFile := fmt.Sprintf("/proc/%s/stat", pid)
	data, err := os.ReadFile(statFile)
	if err != nil {
		return 0, err
	}

	fields := strings.Fields(string(data))
	if len(fields) < 22 {
		return 0, fmt.Errorf("unexpected stat file format")
	}

	starttime, err := strconv.ParseInt(fields[21], 10, 64)
	if err != nil {
		return 0, err
	}

	bootTime, err := getBootTime()
	if err != nil {
		return 0, err
	}

	clkTck := int64(100)
	startTimeSec := bootTime + (starttime / clkTck)
	uptime := time.Now().Unix() - startTimeSec

	return uptime, nil
}

func getBootTime() (int64, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "btime ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return strconv.ParseInt(fields[1], 10, 64)
			}
		}
	}

	return 0, fmt.Errorf("btime not found in /proc/stat")
}

func sanitizeMetricName(name string) string {
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	
	result := ""
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			result += string(c)
		} else {
			result += "_"
		}
	}
	return result
}
