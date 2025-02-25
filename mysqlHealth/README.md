# MySQL Health Check

A comprehensive MySQL/MariaDB health monitoring module that provides various checks and monitoring capabilities for MySQL databases and clusters.

## Features
1. **Basic Connectivity**
   - Verifies database accessibility
   - Performs simple `SELECT NOW()` test

2. **Process Monitoring**
   - Monitors number of active database processes
   - Alerts if process count exceeds configured limit

3. **Cluster Monitoring** (when enabled)
   - Tracks cluster size and status
   - Monitors node synchronization state
   - Checks for inaccessible cluster nodes
   - Creates alerts for cluster size mismatches

4. **Database Integrity**
   - Performs automatic repair checks on configured schedule
   - Generates reports for any repaired tables

5. **PMM Integration**
   - Monitors PMM agent status
   - Verifies PMM service health


## Configuration

The module uses a configuration file with the following structure:
```yaml
mysql:
  process_limit: <number>  # Maximum allowed MySQL processes
  cluster:
    enabled: true/false    # Enable cluster monitoring
    size: <number>        # Expected cluster size
    check_table_day: "Sun" # Day to perform table checks (default: Sunday)
    check_table_hour: "05:00" # Time to perform table checks (default: 05:00)
```

### Database Connection
- Automatically detects MySQL/MariaDB installation
- Parses my.cnf configuration files
- Supports both socket and TCP/IP connections
- Multiple profile support for connection configurations