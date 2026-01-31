package main

import (
	"fmt"
	"os"
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
	WarnOver     int
	CritOver     int
	WarnUnder    int
	CritUnder    int
	Pattern      string
	ExcludePattern string
	State        string
	User         string
	VSZ          int
	RSS          int
	PCPU         float64
	ThreadCount  int
	MatchSelf    bool
	MatchParent  bool
	FilePid      string
	Exact        bool
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "check-process",
			Short:    "Sensu check to monitor processes",
			Keyspace: "sensu.io/plugins/check-process/config",
		},
	}

	options = []sensu.ConfigOption{
		&sensu.PluginConfigOption[int]{
			Path:      "warn-over",
			Argument:  "warn-over",
			Shorthand: "w",
			Default:   0,
			Usage:     "Trigger a warning if over a number",
			Value:     &plugin.WarnOver,
		},
		&sensu.PluginConfigOption[int]{
			Path:      "crit-over",
			Argument:  "crit-over",
			Shorthand: "c",
			Default:   0,
			Usage:     "Trigger a critical if over a number",
			Value:     &plugin.CritOver,
		},
		&sensu.PluginConfigOption[int]{
			Path:      "warn-under",
			Argument:  "warn-under",
			Shorthand: "W",
			Default:   1,
			Usage:     "Trigger a warning if under a number",
			Value:     &plugin.WarnUnder,
		},
		&sensu.PluginConfigOption[int]{
			Path:      "crit-under",
			Argument:  "crit-under",
			Shorthand: "C",
			Default:   1,
			Usage:     "Trigger a critical if under a number",
			Value:     &plugin.CritUnder,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "pattern",
			Argument:  "pattern",
			Shorthand: "p",
			Default:   "",
			Usage:     "Match a command against this pattern",
			Value:     &plugin.Pattern,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "exclude-pattern",
			Argument:  "exclude-pattern",
			Shorthand: "x",
			Default:   "",
			Usage:     "Don't match against this pattern",
			Value:     &plugin.ExcludePattern,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "state",
			Argument:  "state",
			Shorthand: "s",
			Default:   "",
			Usage:     "Match process state (S, D, Z, R, T, etc.)",
			Value:     &plugin.State,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "user",
			Argument:  "user",
			Shorthand: "u",
			Default:   "",
			Usage:     "Match processes owned by this user",
			Value:     &plugin.User,
		},
		&sensu.PluginConfigOption[int]{
			Path:      "vsz",
			Argument:  "vsz",
			Shorthand: "z",
			Default:   0,
			Usage:     "Trigger on VSZ size (in KB)",
			Value:     &plugin.VSZ,
		},
		&sensu.PluginConfigOption[int]{
			Path:      "rss",
			Argument:  "rss",
			Shorthand: "r",
			Default:   0,
			Usage:     "Trigger on RSS size (in KB)",
			Value:     &plugin.RSS,
		},
		&sensu.PluginConfigOption[float64]{
			Path:      "pcpu",
			Argument:  "pcpu",
			Shorthand: "P",
			Default:   0.0,
			Usage:     "Trigger on CPU percentage",
			Value:     &plugin.PCPU,
		},
		&sensu.PluginConfigOption[int]{
			Path:      "thread-count",
			Argument:  "thread-count",
			Shorthand: "T",
			Default:   0,
			Usage:     "Trigger on thread count",
			Value:     &plugin.ThreadCount,
		},
		&sensu.PluginConfigOption[bool]{
			Path:      "match-self",
			Argument:  "match-self",
			Shorthand: "m",
			Default:   false,
			Usage:     "Match itself",
			Value:     &plugin.MatchSelf,
		},
		&sensu.PluginConfigOption[bool]{
			Path:      "match-parent",
			Argument:  "match-parent",
			Shorthand: "M",
			Default:   false,
			Usage:     "Match parent process",
			Value:     &plugin.MatchParent,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "file-pid",
			Argument:  "file-pid",
			Shorthand: "f",
			Default:   "",
			Usage:     "Match PID from file",
			Value:     &plugin.FilePid,
		},
		&sensu.PluginConfigOption[bool]{
			Path:      "exact",
			Argument:  "exact",
			Shorthand: "e",
			Default:   false,
			Usage:     "Exact pattern match (not regex)",
			Value:     &plugin.Exact,
		},
	}
)

type Process struct {
	User    string
	PID     string
	PPID    string
	VSZ     int
	RSS     int
	PCPU    float64
	Stat    string
	Threads int
	Command string
}

func main() {
	check := sensu.NewCheck(&plugin.PluginConfig, options, checkArgs, executeCheck, false)
	check.Execute()
}

func checkArgs(_ *types.Event) (int, error) {
	if plugin.Pattern == "" && plugin.FilePid == "" {
		return sensu.CheckStateCritical, fmt.Errorf("pattern or file-pid must be specified")
	}
	return sensu.CheckStateOK, nil
}

func executeCheck(_ *types.Event) (int, error) {
	processes, err := getProcesses()
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("failed to get processes: %v", err)
	}

	matchedProcs := filterProcesses(processes)
	count := len(matchedProcs)

	// Apply thresholds
	if plugin.CritOver > 0 && count > plugin.CritOver {
		fmt.Printf("CheckProcess CRITICAL: Found %d matching processes (> %d)\n", count, plugin.CritOver)
		return sensu.CheckStateCritical, nil
	}

	if plugin.CritUnder > 0 && count < plugin.CritUnder {
		fmt.Printf("CheckProcess CRITICAL: Found %d matching processes (< %d)\n", count, plugin.CritUnder)
		return sensu.CheckStateCritical, nil
	}

	if plugin.WarnOver > 0 && count > plugin.WarnOver {
		fmt.Printf("CheckProcess WARNING: Found %d matching processes (> %d)\n", count, plugin.WarnOver)
		return sensu.CheckStateWarning, nil
	}

	if plugin.WarnUnder > 0 && count < plugin.WarnUnder {
		fmt.Printf("CheckProcess WARNING: Found %d matching processes (< %d)\n", count, plugin.WarnUnder)
		return sensu.CheckStateWarning, nil
	}

	patternStr := plugin.Pattern
	if plugin.FilePid != "" {
		patternStr = fmt.Sprintf("PID from %s", plugin.FilePid)
	}
	fmt.Printf("CheckProcess OK: Found %d matching processes; pattern /%s/\n", count, patternStr)
	return sensu.CheckStateOK, nil
}

func getProcesses() ([]Process, error) {
	// Use ps with portable options
	cmd := exec.Command("ps", "aux")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(output), "\n")
	var processes []Process

	for i, line := range lines {
		if i == 0 || line == "" {
			continue // Skip header
		}

		fields := strings.Fields(line)
		if len(fields) < 11 {
			continue
		}

		vsz, _ := strconv.Atoi(fields[4])
		rss, _ := strconv.Atoi(fields[5])
		pcpu, _ := strconv.ParseFloat(fields[2], 64)

		// Try to get thread count from ps -L if available
		threads := 1
		if pid := fields[1]; pid != "" {
			threadCmd := exec.Command("ps", "-L", "-p", pid)
			threadOut, err := threadCmd.Output()
			if err == nil {
				threads = len(strings.Split(string(threadOut), "\n")) - 2 // Subtract header and empty line
				if threads < 1 {
					threads = 1
				}
			}
		}

		proc := Process{
			User:    fields[0],
			PID:     fields[1],
			PPID:    "", // Not available in aux format
			VSZ:     vsz,
			RSS:     rss,
			PCPU:    pcpu,
			Stat:    fields[7],
			Threads: threads,
			Command: strings.Join(fields[10:], " "),
		}

		processes = append(processes, proc)
	}

	return processes, nil
}

func filterProcesses(processes []Process) []Process {
	var matched []Process
	myPID := os.Getpid()
	myPPID := os.Getppid()

	// Load PID from file if specified
	var filePID int
	if plugin.FilePid != "" {
		data, err := os.ReadFile(plugin.FilePid)
		if err != nil {
			return matched
		}
		filePID, err = strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			return matched
		}
	}

	for _, proc := range processes {
		pid, _ := strconv.Atoi(proc.PID)

		// Skip self unless explicitly matched
		if !plugin.MatchSelf && pid == myPID {
			continue
		}

		// Skip parent unless explicitly matched
		if !plugin.MatchParent && pid == myPPID {
			continue
		}

		// Match by file PID
		if plugin.FilePid != "" {
			if pid == filePID {
				matched = append(matched, proc)
			}
			continue
		}

		// Match by pattern
		if plugin.Pattern != "" {
			var matches bool
			if plugin.Exact {
				matches = strings.Contains(proc.Command, plugin.Pattern)
			} else {
				re, err := regexp.Compile(plugin.Pattern)
				if err == nil {
					matches = re.MatchString(proc.Command)
				}
			}

			if !matches {
				continue
			}
		}

		// Exclude pattern
		if plugin.ExcludePattern != "" {
			re, err := regexp.Compile(plugin.ExcludePattern)
			if err == nil && re.MatchString(proc.Command) {
				continue
			}
		}

		// Filter by state
		if plugin.State != "" && !strings.Contains(proc.Stat, plugin.State) {
			continue
		}

		// Filter by user
		if plugin.User != "" && proc.User != plugin.User {
			continue
		}

		// Filter by VSZ
		if plugin.VSZ > 0 && proc.VSZ < plugin.VSZ {
			continue
		}

		// Filter by RSS
		if plugin.RSS > 0 && proc.RSS < plugin.RSS {
			continue
		}

		// Filter by PCPU
		if plugin.PCPU > 0 && proc.PCPU < plugin.PCPU {
			continue
		}

		// Filter by thread count
		if plugin.ThreadCount > 0 && proc.Threads < plugin.ThreadCount {
			continue
		}

		matched = append(matched, proc)
	}

	return matched
}
