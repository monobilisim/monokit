# PostgreSQL Health Monitoring Tool

A comprehensive PostgreSQL health monitoring tool that checks database and cluster health, focusing on both standalone PostgreSQL instances and Patroni-managed clusters.

## Features

### Database Health Monitoring
- Monitors active database connections and compares against configured limits
- Tracks running queries and their durations
- Logs active connections to rotating daily log files with detailed information including:
  - Process ID (PID)
  - Username
  - Client address
  - Query duration
  - Query text
  - Connection state

### Patroni Cluster Monitoring
- Monitors Patroni cluster health and service status
- Tracks cluster member states and roles
- Detects and alerts on cluster role changes
- Validates cluster size and configuration
- Generates alerts for cluster issues

## Requirements

- Must be run as the postgres user
- Linux operating system
- PostgreSQL database
- Patroni (optional, for cluster monitoring)

## Configuration

The tool uses configuration files loaded via the `common.ConfInit()` function. Database health settings are configured through the `DbHealth` configuration structure.