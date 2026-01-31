package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
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
			Name:     "metrics-processes-threads-count",
			Short:    "Sensu metrics for process thread counts",
			Keyspace: "sensu.io/plugins/metrics-processes-threads-count/config",
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
			Usage:     "Metric naming scheme (default: hostname.process.threads)",
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
		plugin.Scheme = fmt.Sprintf("%s.process.threads", hostname)
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

		// Get thread count
		threads, err := getThreadCount(pid)
		if err != nil {
			continue
		}

		fmt.Printf("%s.%s.%s %d %d\n", plugin.Scheme, cmdName, pid, threads, timestamp)
	}

	if !found {
		return sensu.CheckStateWarning, fmt.Errorf("no processes matched pattern '%s'", plugin.Pattern)
	}

	return sensu.CheckStateOK, nil
}

func getThreadCount(pid string) (int, error) {
	cmd := exec.Command("ps", "-L", "-p", pid)
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(output), "\n")
	count := 0
	for i, line := range lines {
		if i > 0 && line != "" {
			count++
		}
	}

	return count, nil
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
