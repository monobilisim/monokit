# PMG Health Check

A health monitoring tool for Proxmox Mail Gateway (PMG) services.

## Overview

PMG Health Check is a Linux-based monitoring tool designed to verify the health and operational status of Proxmox Mail Gateway services. It performs various checks on critical components and reports their status.

## Features

- **Service Status Monitoring**: Checks if essential PMG services are running:
  - pmgproxy.service
  - pmg-smtp-filter.service
  - postfix@-.service

- **PostgreSQL Database Check**: Verifies if the PostgreSQL database is operational

- **Mail Queue Monitoring**: Tracks the number of messages in the mail queue and compares against configured limits

- **Version Verification**: Checks the Proxmox Mail Gateway version

## Requirements

- Linux operating system
- Proxmox Mail Gateway installation
- PostgreSQL database
- Access to systemd services

## Usage

The tool is designed to be run as part of the monokit framework. It automatically:

1. Initializes configuration from the mail configuration file
2. Checks service status via API
3. Performs version verification
4. Monitors PMG services
5. Checks PostgreSQL status
6. Analyzes mail queue status

## Configuration

Configuration is managed through the mail configuration file, which includes settings such as:

- Queue limit thresholds for mail monitoring

## Output

The tool provides clear status output for each component:

- Service status (running/not running)
- PostgreSQL availability
- Mail queue statistics
- Version information