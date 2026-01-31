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
			Name:     "metrics-per-process",
			Short:    "Sensu metrics for per-process statistics",
			Keyspace: "sensu.io/plugins/metrics-per-process/config",
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
			Usage:     "Metric naming scheme (default: hostname.process)",
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
		plugin.Scheme = fmt.Sprintf("%s.process", hostname)
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
		cpu, _ := strconv.ParseFloat(fields[2], 64)
		mem, _ := strconv.ParseFloat(fields[3], 64)
		vsz, _ := strconv.Atoi(fields[4])
		rss, _ := strconv.Atoi(fields[5])

		// Sanitize process name for metric path
		cmdName := sanitizeMetricName(fields[10])

		fmt.Printf("%s.%s.%s.cpu %.2f %d\n", plugin.Scheme, cmdName, pid, cpu, timestamp)
		fmt.Printf("%s.%s.%s.mem %.2f %d\n", plugin.Scheme, cmdName, pid, mem, timestamp)
		fmt.Printf("%s.%s.%s.vsz %d %d\n", plugin.Scheme, cmdName, pid, vsz, timestamp)
		fmt.Printf("%s.%s.%s.rss %d %d\n", plugin.Scheme, cmdName, pid, rss, timestamp)
	}

	if !found {
		return sensu.CheckStateWarning, fmt.Errorf("no processes matched pattern '%s'", plugin.Pattern)
	}

	return sensu.CheckStateOK, nil
}

func sanitizeMetricName(name string) string {
	// Remove path and keep only basename
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	
	// Replace non-alphanumeric with underscore
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
