package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/sensu/sensu-go/types"
	"github.com/sensu/sensu-plugin-sdk/sensu"
)

// Config represents the check plugin config.
type Config struct {
	sensu.PluginConfig
	Command      string
	Status       int
	StatusRegex  string
	OutputRegex  string
	Timeout      int
	Shell        string
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "check-cmd",
			Short:    "Sensu check to run a command and check its output or exit status",
			Keyspace: "sensu.io/plugins/check-cmd/config",
		},
	}

	options = []sensu.ConfigOption{
		&sensu.PluginConfigOption[string]{
			Path:      "command",
			Argument:  "command",
			Shorthand: "c",
			Default:   "",
			Usage:     "Command to run",
			Value:     &plugin.Command,
		},
		&sensu.PluginConfigOption[int]{
			Path:      "status",
			Argument:  "status",
			Shorthand: "s",
			Default:   0,
			Usage:     "Expected exit status",
			Value:     &plugin.Status,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "status-regex",
			Argument:  "status-regex",
			Shorthand: "S",
			Default:   "",
			Usage:     "Regex to match expected exit status",
			Value:     &plugin.StatusRegex,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "output-regex",
			Argument:  "output-regex",
			Shorthand: "o",
			Default:   "",
			Usage:     "Regex to match command output",
			Value:     &plugin.OutputRegex,
		},
		&sensu.PluginConfigOption[int]{
			Path:      "timeout",
			Argument:  "timeout",
			Shorthand: "t",
			Default:   30,
			Usage:     "Command timeout in seconds",
			Value:     &plugin.Timeout,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "shell",
			Argument:  "shell",
			Shorthand: "b",
			Default:   "/bin/sh",
			Usage:     "Shell to use for command execution",
			Value:     &plugin.Shell,
		},
	}
)

func main() {
	check := sensu.NewCheck(&plugin.PluginConfig, options, checkArgs, executeCheck, false)
	check.Execute()
}

func checkArgs(_ *types.Event) (int, error) {
	if plugin.Command == "" {
		return sensu.CheckStateCritical, fmt.Errorf("command must be specified")
	}
	return sensu.CheckStateOK, nil
}

func executeCheck(_ *types.Event) (int, error) {
	// Execute command via shell
	cmd := exec.Command(plugin.Shell, "-c", plugin.Command)
	output, err := cmd.CombinedOutput()
	exitCode := 0

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return sensu.CheckStateCritical, fmt.Errorf("failed to execute command: %v", err)
		}
	}

	outputStr := string(output)

	// Check output regex if specified
	if plugin.OutputRegex != "" {
		re, err := regexp.Compile(plugin.OutputRegex)
		if err != nil {
			return sensu.CheckStateCritical, fmt.Errorf("invalid output regex: %v", err)
		}

		if !re.MatchString(outputStr) {
			fmt.Printf("CheckCmd CRITICAL: Output does not match regex /%s/\nOutput: %s\n", 
				plugin.OutputRegex, strings.TrimSpace(outputStr))
			return sensu.CheckStateCritical, nil
		}
	}

	// Check status regex if specified
	if plugin.StatusRegex != "" {
		re, err := regexp.Compile(plugin.StatusRegex)
		if err != nil {
			return sensu.CheckStateCritical, fmt.Errorf("invalid status regex: %v", err)
		}

		exitCodeStr := fmt.Sprintf("%d", exitCode)
		if !re.MatchString(exitCodeStr) {
			fmt.Printf("CheckCmd CRITICAL: Exit status %d does not match regex /%s/\nOutput: %s\n",
				exitCode, plugin.StatusRegex, strings.TrimSpace(outputStr))
			return sensu.CheckStateCritical, nil
		}
	} else {
		// Check exact status if no regex specified
		if exitCode != plugin.Status {
			fmt.Printf("CheckCmd CRITICAL: Exit status %d (expected %d)\nOutput: %s\n",
				exitCode, plugin.Status, strings.TrimSpace(outputStr))
			return sensu.CheckStateCritical, nil
		}
	}

	fmt.Printf("CheckCmd OK: Command executed successfully\nOutput: %s\n", strings.TrimSpace(outputStr))
	return sensu.CheckStateOK, nil
}
