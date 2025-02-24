# osHealth

osHealth is a system health monitoring package that tracks various system metrics including disk usage, system load, and RAM usage. It's part of the monokit toolkit and provides monitoring with Redmine issue integration.

## Alerts and Monitoring

The package provides two types of alerts:
1. System alerts through the common.AlarmCheck system
2. Redmine issue creation/updates through the issues package

## Features and Monitoring Areas

- **Disk Usage Monitoring**
  - Monitors disk partition usage
  - Configurable filesystem types to monitor
  - Creates alerts when partition usage exceeds defined limits
  - Generates detailed usage tables

- **System Load Monitoring**
  - Tracks system load average
  - Configurable load limits based on CPU count
  - Issues alerts when system load exceeds thresholds

- **RAM Usage Monitoring**
  - Monitors memory usage percentage
  - Configurable RAM usage limits
  - Generates alerts for excessive memory usage

## Configuration

Configuration is done via a configuration file with the following structure:
```yaml
os:
  filesystems:
    - ext4
    - xfs
    # Add other filesystem types to monitor
  
  system_load_and_ram: true
  part_use_limit: 90  # Partition usage limit percentage

  load:
    issue_interval: 15      # Minutes between issue checks
    issue_multiplier: 1     # Load multiplier for issue creation
    limit_multiplier: 1     # Load multiplier for alerts

  ram_limit: 90  # RAM usage limit percentage

  alarm:
    enabled: true
```

## Usage

The package is typically used as part of the monokit toolkit. It can be run using the cobra command framework:

```go
osHealth.Main(cmd, args)
```

## Dependencies

- github.com/shirou/gopsutil/v4
- github.com/monobilisim/monokit/common
- github.com/olekukonko/tablewriter
- github.com/spf13/cobra