package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sensu/sensu-go/types"
	"github.com/sensu/sensu-plugin-sdk/sensu"
)

// Config represents the metrics plugin config.
type Config struct {
	sensu.PluginConfig
	Scheme string
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "metrics-process-status",
			Short:    "Sensu metrics for process status counts",
			Keyspace: "sensu.io/plugins/metrics-process-status/config",
		},
	}

	options = []sensu.ConfigOption{
		&sensu.PluginConfigOption[string]{
			Path:      "scheme",
			Argument:  "scheme",
			Shorthand: "s",
			Default:   "",
			Usage:     "Metric naming scheme (default: hostname.processes)",
			Value:     &plugin.Scheme,
		},
	}
)

func main() {
	check := sensu.NewCheck(&plugin.PluginConfig, options, checkArgs, executeCheck, false)
	check.Execute()
}

func checkArgs(_ *types.Event) (int, error) {
	if plugin.Scheme == "" {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "unknown"
		}
		plugin.Scheme = fmt.Sprintf("%s.processes", hostname)
	}
	return sensu.CheckStateOK, nil
}

func executeCheck(_ *types.Event) (int, error) {
	cmd := exec.Command("ps", "aux")
	output, err := cmd.Output()
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("failed to get processes: %v", err)
	}

	// Count processes by state
	stateCounts := make(map[string]int)
	stateCounts["running"] = 0
	stateCounts["sleeping"] = 0
	stateCounts["stopped"] = 0
	stateCounts["zombie"] = 0
	stateCounts["idle"] = 0
	stateCounts["other"] = 0
	
	total := 0
	lines := strings.Split(string(output), "\n")

	for i, line := range lines {
		if i == 0 || line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}

		total++
		stat := fields[7]
		
		// Parse state code (first character)
		if len(stat) > 0 {
			switch stat[0] {
			case 'R':
				stateCounts["running"]++
			case 'S', 'D':
				stateCounts["sleeping"]++
			case 'T':
				stateCounts["stopped"]++
			case 'Z':
				stateCounts["zombie"]++
			case 'I':
				stateCounts["idle"]++
			default:
				stateCounts["other"]++
			}
		}
	}

	timestamp := time.Now().Unix()
	
	fmt.Printf("%s.total %d %d\n", plugin.Scheme, total, timestamp)
	fmt.Printf("%s.running %d %d\n", plugin.Scheme, stateCounts["running"], timestamp)
	fmt.Printf("%s.sleeping %d %d\n", plugin.Scheme, stateCounts["sleeping"], timestamp)
	fmt.Printf("%s.stopped %d %d\n", plugin.Scheme, stateCounts["stopped"], timestamp)
	fmt.Printf("%s.zombie %d %d\n", plugin.Scheme, stateCounts["zombie"], timestamp)
	fmt.Printf("%s.idle %d %d\n", plugin.Scheme, stateCounts["idle"], timestamp)
	fmt.Printf("%s.other %d %d\n", plugin.Scheme, stateCounts["other"], timestamp)

	return sensu.CheckStateOK, nil
}
