package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sensu/sensu-go/types"
	"github.com/sensu/sensu-plugin-sdk/sensu"
)

// Config represents the check plugin config.
type Config struct {
	sensu.PluginConfig
	Pattern    string
	StateDir   string
	MaxAge     int
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "check-process-restart",
			Short:    "Sensu check to detect process restarts",
			Keyspace: "sensu.io/plugins/check-process-restart/config",
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
			Path:      "state-dir",
			Argument:  "state-dir",
			Shorthand: "s",
			Default:   "/tmp",
			Usage:     "Directory to store process state",
			Value:     &plugin.StateDir,
		},
		&sensu.PluginConfigOption[int]{
			Path:      "max-age",
			Argument:  "max-age",
			Shorthand: "m",
			Default:   3600,
			Usage:     "Maximum age in seconds before considering process restarted",
			Value:     &plugin.MaxAge,
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
	// Find matching process
	cmd := exec.Command("pgrep", "-f", plugin.Pattern)
	output, err := cmd.Output()
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("no process matching pattern '%s' found", plugin.Pattern)
	}

	pids := strings.Fields(string(output))
	if len(pids) == 0 {
		return sensu.CheckStateCritical, fmt.Errorf("no process matching pattern '%s' found", plugin.Pattern)
	}

	// Get process start time
	pid := pids[0]
	startTime, err := getProcessStartTime(pid)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("failed to get process start time: %v", err)
	}

	// Check state file
	stateFile := filepath.Join(plugin.StateDir, fmt.Sprintf("check-process-restart-%s.state", sanitizePattern(plugin.Pattern)))
	
	savedTime, err := readStateFile(stateFile)
	if err != nil {
		// First run, save state
		if err := writeStateFile(stateFile, startTime); err != nil {
			return sensu.CheckStateWarning, fmt.Errorf("failed to write state file: %v", err)
		}
		fmt.Printf("CheckProcessRestart OK: Initial check, saved state\n")
		return sensu.CheckStateOK, nil
	}

	// Compare times
	if startTime > savedTime {
		// Process restarted
		age := time.Now().Unix() - startTime
		if err := writeStateFile(stateFile, startTime); err != nil {
			return sensu.CheckStateWarning, fmt.Errorf("process restarted but failed to update state: %v", err)
		}
		fmt.Printf("CheckProcessRestart CRITICAL: Process restarted %d seconds ago\n", age)
		return sensu.CheckStateCritical, nil
	}

	age := time.Now().Unix() - startTime
	fmt.Printf("CheckProcessRestart OK: Process running for %d seconds\n", age)
	return sensu.CheckStateOK, nil
}

func getProcessStartTime(pid string) (int64, error) {
	// Read /proc/<pid>/stat to get start time
	statFile := fmt.Sprintf("/proc/%s/stat", pid)
	data, err := os.ReadFile(statFile)
	if err != nil {
		return 0, err
	}

	// Parse stat file - start time is field 22 (0-indexed: 21)
	fields := strings.Fields(string(data))
	if len(fields) < 22 {
		return 0, fmt.Errorf("unexpected stat file format")
	}

	starttime, err := strconv.ParseInt(fields[21], 10, 64)
	if err != nil {
		return 0, err
	}

	// Convert from clock ticks to unix timestamp
	// Read system boot time from /proc/stat
	bootTime, err := getBootTime()
	if err != nil {
		return 0, err
	}

	clkTck := int64(100) // USER_HZ, typically 100
	startTimeSec := bootTime + (starttime / clkTck)
	
	return startTimeSec, nil
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

func readStateFile(path string) (int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
}

func writeStateFile(path string, startTime int64) error {
	return os.WriteFile(path, []byte(fmt.Sprintf("%d", startTime)), 0644)
}

func sanitizePattern(pattern string) string {
	// Replace non-alphanumeric characters with underscores
	result := ""
	for _, c := range pattern {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			result += string(c)
		} else {
			result += "_"
		}
	}
	return result
}
