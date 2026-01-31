package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/sensu/sensu-go/types"
	"github.com/sensu/sensu-plugin-sdk/sensu"
)

// Config represents the check plugin config.
type Config struct {
	sensu.PluginConfig
	Pattern  string
	Warning  int
	Critical int
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "check-threads-count",
			Short:    "Sensu check to monitor thread count for a process",
			Keyspace: "sensu.io/plugins/check-threads-count/config",
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
		&sensu.PluginConfigOption[int]{
			Path:      "warning",
			Argument:  "warning",
			Shorthand: "w",
			Default:   100,
			Usage:     "Warning threshold for thread count",
			Value:     &plugin.Warning,
		},
		&sensu.PluginConfigOption[int]{
			Path:      "critical",
			Argument:  "critical",
			Shorthand: "c",
			Default:   200,
			Usage:     "Critical threshold for thread count",
			Value:     &plugin.Critical,
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
	return sensu.CheckStateOK, nil
}

func executeCheck(_ *types.Event) (int, error) {
	// Find matching processes
	cmd := exec.Command("ps", "aux")
	output, err := cmd.Output()
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("failed to get processes: %v", err)
	}

	re, err := regexp.Compile(plugin.Pattern)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("invalid pattern: %v", err)
	}

	lines := strings.Split(string(output), "\n")
	var matchedPIDs []string

	for i, line := range lines {
		if i == 0 || line == "" {
			continue
		}

		if re.MatchString(line) {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				matchedPIDs = append(matchedPIDs, fields[1])
			}
		}
	}

	if len(matchedPIDs) == 0 {
		return sensu.CheckStateCritical, fmt.Errorf("no process matching pattern '%s' found", plugin.Pattern)
	}

	// Count threads for matched processes
	totalThreads := 0
	for _, pid := range matchedPIDs {
		threads, err := getThreadCount(pid)
		if err != nil {
			continue
		}
		totalThreads += threads
	}

	// Check thresholds
	if totalThreads >= plugin.Critical {
		fmt.Printf("CheckThreadsCount CRITICAL: %d threads (threshold: %d)\n", totalThreads, plugin.Critical)
		return sensu.CheckStateCritical, nil
	}

	if totalThreads >= plugin.Warning {
		fmt.Printf("CheckThreadsCount WARNING: %d threads (threshold: %d)\n", totalThreads, plugin.Warning)
		return sensu.CheckStateWarning, nil
	}

	fmt.Printf("CheckThreadsCount OK: %d threads\n", totalThreads)
	return sensu.CheckStateOK, nil
}

func getThreadCount(pid string) (int, error) {
	cmd := exec.Command("ps", "-L", "-p", pid)
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(output), "\n")
	// Subtract 1 for header, count remaining non-empty lines
	count := 0
	for i, line := range lines {
		if i > 0 && line != "" {
			count++
		}
	}

	return count, nil
}
