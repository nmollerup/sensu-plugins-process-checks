# sensu-plugins-process-checks

Go implementation of process monitoring checks and metrics for Sensu, replacing the Ruby-based [sensu-plugins-process-checks](https://github.com/sensu-plugins/sensu-plugins-process-checks).

## Features

This project provides the following commands:

### Check Commands

- **check-process** - Monitor processes on a system with various filters (name, state, user, resource usage, etc.)
- **check-cmd** - Run a command and check its output or exit status
- **check-process-restart** - Detect when a process has restarted
- **check-threads-count** - Monitor thread count for processes

### Metrics Commands

- **metrics-per-process** - Collect per-process statistics (CPU, memory, VSZ, RSS)
- **metrics-process-status** - Count processes by status (running, sleeping, stopped, zombie, etc.)
- **metrics-process-uptime** - Track process uptime
- **metrics-processes-threads-count** - Collect thread counts per process

## Installation

### From Source

```bash
go install github.com/nmollerup/sensu-plugins-process-checks/cmd/check-process@latest
go install github.com/nmollerup/sensu-plugins-process-checks/cmd/check-cmd@latest
# ... etc
```

### Using GoReleaser

Build all binaries:

```bash
goreleaser build --snapshot --clean
```

## Usage

### check-process

Check if a process is running:

```bash
check-process -p nginx
```

Check with custom thresholds:

```bash
check-process -p nginx -W 2 -C 1
```

Filter by process state:

```bash
check-process -s Z -w 5 -c 10  # Check for zombie processes
```

### check-cmd

Run a command and check its exit status:

```bash
check-cmd -c "curl -s http://localhost:8080/health" -s 0
```

Check command output with regex:

```bash
check-cmd -c "uptime" -o "load average"
```

### check-process-restart

Detect process restarts:

```bash
check-process-restart -p nginx
```

### check-threads-count

Monitor thread count:

```bash
check-threads-count -p java -w 100 -c 200
```

### metrics-per-process

Collect per-process metrics:

```bash
metrics-per-process -p nginx
```

### metrics-process-status

Collect process status metrics:

```bash
metrics-process-status
```

### metrics-process-uptime

Collect process uptime metrics:

```bash
metrics-process-uptime -p nginx
```

### metrics-processes-threads-count

Collect thread count metrics:

```bash
metrics-processes-threads-count -p java
```

## Configuration

All commands support configuration via:
- Command-line flags
- Environment variables
- Sensu annotations

Example Sensu check definition:

```yaml
---
type: CheckConfig
api_version: core/v2
metadata:
  name: check-nginx
spec:
  command: check-process -p nginx -W 1
  subscriptions:
    - system
  interval: 60
  publish: true
```

## Building

```bash
# Build for current platform
go build -o bin/check-process ./cmd/check-process

# Build all binaries
goreleaser build --snapshot --clean
```

## License

MIT License

## Credits

Based on [sensu-plugins-process-checks](https://github.com/sensu-plugins/sensu-plugins-process-checks) and inspired by [sensu-check-entropy](https://github.com/nmollerup/sensu-check-entropy).
