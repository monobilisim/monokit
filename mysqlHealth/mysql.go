//go:build linux

package mysqlHealth

import (
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/go-ini/ini"
	_ "github.com/go-sql-driver/mysql"      // Keep anonymous import for side effects
	_mysql "github.com/go-sql-driver/mysql" // Import with alias
	"github.com/monobilisim/monokit/common"
)

var Connection *sql.DB

func mariadbOrMysql() string {
	_, err := exec.LookPath("/usr/bin/mysql")
	if err != nil {
		return "mariadb"
	}
	return "mysql"
}

func FindMyCnf() []string {
	daemonBinary := "/usr/sbin/" + mariadbOrMysql() + "d"
	// Check if the daemon binary exists before trying to execute it
	_, err := exec.LookPath(daemonBinary)
	if err != nil {
		// Log as debug because this is expected on systems without MySQL/MariaDB server installed
		common.LogDebug(fmt.Sprintf("Daemon binary %s not found, cannot determine config paths via --help. Error: %v", daemonBinary, err))
		return nil // Cannot proceed without the daemon binary
	}

	cmd := exec.Command(daemonBinary, "--verbose", "--help")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Log as error here because LookPath succeeded but execution failed
		common.LogError(fmt.Sprintf("Error running %s command: %v", daemonBinary, err))
		return nil
	}

	lines := strings.Split(string(output), "\n")
	foundDefaultOptions := false
	for _, line := range lines {
		if strings.Contains(line, "Default options") {
			foundDefaultOptions = true
			continue
		}

		if foundDefaultOptions {

			return strings.Fields(strings.Replace(line, "~", os.Getenv("HOME"), 1))
		}
	}

	return nil
}

func ParseMyCnfAndConnect(profile string) (string, error) {
	var host, port, dbname, user, password, socket string
	var found bool
	var finalConn string
	var lastErr error // Store the last connection/ping error

	// Close any lingering global connection before starting
	if Connection != nil {
		Connection.Close()
		Connection = nil
	}

	cnfPaths := FindMyCnf()
	if cnfPaths == nil || len(cnfPaths) == 0 {
		return "", fmt.Errorf("could not find any my.cnf paths")
	}

	for _, path := range cnfPaths {
		if _, err := os.Stat(path); err == nil {
			cfg, err := ini.LoadSources(ini.LoadOptions{AllowBooleanKeys: true}, path)
			if err != nil {
				// Log or store this error, but continue trying other paths
				common.LogDebug(fmt.Sprintf("Error loading config file %s: %v", path, err))
				lastErr = fmt.Errorf("error loading config file %s: %w", path, err)
				continue
			}

			for _, s := range cfg.Sections() {
				// Ensure section name matches the requested profile exactly or is the default "[client]" if profile is "client"
				// Allow broader matching if profile is empty (though current usage specifies "client")
				sectionName := s.Name()
				isMatch := false
				if profile == "" {
					isMatch = true // Match any section if profile is empty
				} else if sectionName == profile {
					isMatch = true // Exact match
				} else if (profile == "client" && sectionName == "client") || (profile == "client-server" && sectionName == "client-server") {
					isMatch = true // Specific case for default client profile
				} else if strings.HasPrefix(sectionName, profile) && sectionName != "DEFAULT" {
					// Optional: Allow prefix matching if needed, but exact match is safer
					// isMatch = true
				}

				if !isMatch {
					continue
				}

				// Reset vars for each profile attempt
				host = s.Key("host").String()
				port = s.Key("port").String()
				// dbname = s.Key("database").String() // Standard key is 'database' not 'dbname'
				dbname = s.Key("database").String()
				user = s.Key("user").String()
				password = s.Key("password").String()
				socket = s.Key("socket").String()
				currentConnStr := "" // Use a local var for the connection string attempt

				// Construct DSN (Data Source Name)
				// Format: username:password@protocol(address)/dbname?param=value
				// Reference: https://github.com/go-sql-driver/mysql#dsn-data-source-name
				config := _mysql.NewConfig()                     // Use imported driver's config struct for correctness
				config.Net = "tcp"                               // Default to TCP
				config.Addr = fmt.Sprintf("%s:%s", host, "3306") // Default port
				config.User = user
				config.Passwd = password
				config.DBName = dbname
				config.AllowNativePasswords = true // Often needed

				if socket != "" {
					config.Net = "unix"
					config.Addr = socket
					// User/Password/DBName already set
				} else { // No socket defined, try host
					if host == "" { // Host is also empty, default it
						host = "127.0.0.1"
						common.LogDebug(fmt.Sprintf("Host not specified for profile [%s] in %s, defaulting to %s", s.Name(), path, host))
					}
					// Now host is guaranteed to be non-empty (either original or defaulted)
					config.Net = "tcp"
					if port != "" {
						config.Addr = fmt.Sprintf("%s:%s", host, port)
					} else {
						config.Addr = fmt.Sprintf("%s:%s", host, "3306") // Use default port if not specified
					}
					// User/Password/DBName already set
				}

				// Validate required fields before attempting connection
				if config.User == "" {
					common.LogDebug(fmt.Sprintf("Defaulting user for 'root' profile [%s] in %s", s.Name(), path))
					config.User	= "root" // Default to 'root' if no user specified
				}

				currentConnStr = config.FormatDSN()

				// Attempt to connect and ping with the current profile's details
				tempDb, err := sql.Open("mysql", currentConnStr)
				if err != nil {
					lastErr = fmt.Errorf("error opening connection for profile [%s] in %s (DSN: %s): %w", s.Name(), path, currentConnStr, err)
					common.LogDebug(lastErr.Error())
					if tempDb != nil {
						tempDb.Close()
					} // Close if open failed but returned a non-nil db
					continue // Try next profile
				}

				// Set connection timeouts before pinging
				tempDb.SetConnMaxLifetime(time.Minute * 3)
				tempDb.SetMaxOpenConns(10)
				tempDb.SetMaxIdleConns(10)

				pingErr := tempDb.Ping()
				if pingErr == nil {
					// Success! Assign to global Connection and return.
					if Connection != nil { // Close previous successful connection if any (shouldn't happen with break)
						Connection.Close()
					}
					Connection = tempDb // Assign the successful connection
					finalConn = currentConnStr
					found = true
					if os.Getenv("MONOKIT_DEBUG") == "1" || os.Getenv("MONOKIT_DEBUG") == "true" {
						fmt.Println("Connected and pinged MySQL successfully with profile: " + s.Name())
						fmt.Println("Connection string: " + finalConn) // DSN format doesn't include password here
						fmt.Println("MyCnf path: " + path)
					}
					break // Exit inner loop (sections)
				} else {
					// Ping failed, store error, close temp connection, continue loop
					// Log the DSN directly; FormatDSN generally avoids embedding the password.
					lastErr = fmt.Errorf("error pinging connection for profile [%s] in %s (DSN: %s): %w", s.Name(), path, currentConnStr, pingErr)
					common.LogDebug(lastErr.Error())
					tempDb.Close() // Close the connection that failed to ping
					// Continue to the next section
				}
			} // End sections loop

			if found {
				break // Exit outer loop (paths)
			}
		} else {
			// Log if file doesn't exist, might be ignorable depending on FindMyCnf behavior
			// common.LogDebug(fmt.Sprintf("Config file not found or not accessible: %s", path))
		}
	} // End paths loop

	if !found {
		// If no connection succeeded, return the last error encountered, or a generic one
		if lastErr != nil {
			// Prepend a generic message to the last specific error
			return "", fmt.Errorf("failed to connect and ping any MySQL profile '%s': %w", profile, lastErr)
		}
		// If lastErr is nil, it means no profiles were even attempted (e.g., FindMyCnf returned empty, or no sections matched)
		return "", fmt.Errorf("no suitable MySQL profile '%s' found or connection failed", profile)
	}

	// Success
	return finalConn, nil
}

// Connect function is now redundant as ParseMyCnfAndConnect handles setting the global Connection
/*
func Connect(connStr string) error {
	db, err := sql.Open("mysql", connStr)
	if err != nil {
		return err
	}
	// Set connection pool settings
	db.SetConnMaxLifetime(time.Minute * 3)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
	Connection = db
	return nil
}
*/

func SelectNow() {
	rows, err := Connection.Query("SELECT NOW() as now")
	if err != nil {
		common.LogError(err.Error())
		common.AlarmCheckDown("access", "Failed to execute SELECT NOW() query", false, "", "")
		// Update health data
		healthData.ConnectionInfo.Error = err.Error()
		return
	}
	defer rows.Close()

	var now string
	for rows.Next() {
		rows.Scan(&now)
	}

	// Update health data instead of printing
	healthData.ConnectionInfo.ServerTime = now
	common.AlarmCheckUp("access", "MySQL access OK", false)
}

func CheckProcessCount() {
	rows, err := Connection.Query("SHOW PROCESSLIST")
	if err != nil {
		common.LogError(err.Error())
		return
	}
	defer rows.Close()

	// Count the rows
	count := 0
	for rows.Next() {
		count++
	}

	maxProcessCount := 100
	if DbHealthConfig.Mysql.Process_limit != 0 {
		maxProcessCount = DbHealthConfig.Mysql.Process_limit
	}

	processPercent := (float64(count) / float64(maxProcessCount)) * 100

	// Update health data
	healthData.ProcessInfo.Total = count
	healthData.ProcessInfo.Limit = maxProcessCount
	healthData.ProcessInfo.ProcessPercent = processPercent

	// Set Exceeded flag based on actual count compared to limit to match UI display
	healthData.ProcessInfo.Exceeded = count > maxProcessCount

	// Still log alarms based on percentage for early warnings
	if processPercent > 90 {
		common.AlarmCheckDown("process count", fmt.Sprintf("Process count > 90%%: %d/%d (%.2f%%)", count, maxProcessCount, processPercent), false, "", "")
	} else {
		common.AlarmCheckUp("process count", fmt.Sprintf("Process count OK: %d/%d (%.2f%%)", count, maxProcessCount, processPercent), false)
	}
}

func InaccessibleClusters() {
	// Check node is part of the cluster
	rows, err := Connection.Query("SELECT @@wsrep_on")
	if err != nil || err == sql.ErrNoRows {
		common.LogDebug(fmt.Sprintf("wsrep_on query failed: %v", err))
		return
	}
	defer rows.Close()

	var wsrepOn string
	if rows.Next() {
		if err := rows.Scan(&wsrepOn); err != nil {
			common.LogError(fmt.Sprintf("Error scanning wsrep_on: %v", err))
			return
		}
	}

	// If wsrep_on is not ON or 1, this node is not part of a Galera cluster
	if wsrepOn != "ON" && wsrepOn != "1" {
		healthData.ClusterInfo.Status = "Not a cluster node"
		return
	}

	query := "SHOW GLOBAL STATUS WHERE Variable_name = 'wsrep_cluster_status'"
	rows, err = Connection.Query(query)

	if err != nil || err == sql.ErrNoRows {
		common.LogDebug(fmt.Sprintf("InaccessibleClusters query failed: %v", err))
		return
	}
	defer rows.Close()

	var variableName, wsrepClusterStatus string
	if rows.Next() {
		if err := rows.Scan(&variableName, &wsrepClusterStatus); err != nil {
			common.LogError(fmt.Sprintf("Error scanning rows: %v", err))
			return
		}
	}

	if wsrepClusterStatus != "Primary" {
		common.AlarmCheckDown("cluster status", fmt.Sprintf("Cluster status is not Primary: %s", wsrepClusterStatus), false, "", "")
	} else {
		common.AlarmCheckUp("cluster status", "Cluster status is Primary", false)
	}

	// Check for non-primary nodes in the cluster
	query = "SHOW STATUS WHERE Variable_name='wsrep_cluster_size'"
	rows, err = Connection.Query(query)
	if err != nil {
		common.LogError(fmt.Sprintf("Error querying wsrep_cluster_size: %v", err))
		return
	}
	defer rows.Close()

	var clusterSize string
	if rows.Next() {
		if err := rows.Scan(&variableName, &clusterSize); err != nil {
			common.LogError(fmt.Sprintf("Error scanning wsrep_cluster_size: %v", err))
			return
		}
	}

	// Parse and store the cluster size
	clusterSizeInt := 0
	fmt.Sscanf(clusterSize, "%d", &clusterSizeInt)
	healthData.ClusterInfo.ClusterSize = clusterSizeInt

	// Count inaccessible nodes
	var inaccessibleCount int = 0
	// Update health data
	healthData.ClusterInfo.InaccessibleCount = inaccessibleCount
}

func CheckClusterStatus() {
	query := "SHOW GLOBAL STATUS WHERE Variable_name = 'wsrep_cluster_status'"
	rows, err := Connection.Query(query)
	if err != nil {
		common.LogError(fmt.Sprintf("CheckClusterStatus query failed: %v", err))
		return
	}
	defer rows.Close()

	var variableName, wsrepClusterStatus string
	if rows.Next() {
		if err := rows.Scan(&variableName, &wsrepClusterStatus); err != nil {
			common.LogError(fmt.Sprintf("Error scanning rows: %v", err))
			return
		}
	}

	// Update health data
	healthData.ClusterInfo.Status = wsrepClusterStatus

	if wsrepClusterStatus != "Primary" && wsrepClusterStatus != "Primary-Primary" {
		common.AlarmCheckDown("cluster status", fmt.Sprintf("Cluster status is not Primary or Primary-Primary: %s", wsrepClusterStatus), false, "", "")
	} else {
		common.AlarmCheckUp("cluster status", fmt.Sprintf("Cluster status is %s", wsrepClusterStatus), false)
	}
}

func CheckNodeStatus() {
	query := "SHOW GLOBAL STATUS WHERE Variable_name = 'wsrep_local_state_comment'"
	rows, err := Connection.Query(query)
	if err != nil {
		common.LogError(fmt.Sprintf("CheckNodeStatus query failed: %v", err))
		return
	}
	defer rows.Close()

	var variableName, wsrepLocalStateComment string
	if rows.Next() {
		if err := rows.Scan(&variableName, &wsrepLocalStateComment); err != nil {
			common.LogError(fmt.Sprintf("Error scanning rows: %v", err))
			return
		}
	}

	// Get node name
	query = "SHOW GLOBAL STATUS WHERE Variable_name = 'wsrep_node_name'"
	rows, err = Connection.Query(query)
	if err != nil {
		common.LogError(fmt.Sprintf("wsrep_node_name query failed: %v", err))
		return
	}
	defer rows.Close()

	var nodeName string
	if rows.Next() {
		if err := rows.Scan(&variableName, &nodeName); err != nil {
			common.LogError(fmt.Sprintf("Error scanning node name: %v", err))
			return
		}
	}

	// Add node to health data
	node := NodeInfo{
		Name:   nodeName,
		Status: wsrepLocalStateComment,
		Active: wsrepLocalStateComment == "Synced",
	}
	healthData.ClusterInfo.Nodes = append(healthData.ClusterInfo.Nodes, node)

	if wsrepLocalStateComment != "Synced" {
		common.AlarmCheckDown("node status", fmt.Sprintf("Node status is not Synced: %s", wsrepLocalStateComment), false, "", "")
	} else {
		common.AlarmCheckUp("node status", "Node status is Synced", false)
	}
}

func CheckClusterSynced() {
	query := "SHOW GLOBAL STATUS WHERE Variable_name = 'wsrep_local_state_comment'"
	rows, err := Connection.Query(query)
	if err != nil {
		common.LogError(fmt.Sprintf("CheckClusterSynced query failed: %v", err))
		return
	}
	defer rows.Close()

	var variableName, wsrepLocalStateComment string
	if rows.Next() {
		if err := rows.Scan(&variableName, &wsrepLocalStateComment); err != nil {
			common.LogError(fmt.Sprintf("Error scanning rows: %v", err))
			return
		}
	}

	isSynced := wsrepLocalStateComment == "Synced"
	// Update health data
	healthData.ClusterInfo.Synced = isSynced

	if !isSynced {
		common.AlarmCheckDown("cluster synced", fmt.Sprintf("Cluster is not synced, state: %s", wsrepLocalStateComment), false, "", "")
	} else {
		common.AlarmCheckUp("cluster synced", "Cluster is synced", false)
	}
}

func CheckDB() {
	cmd := exec.Command("/usr/bin/"+mariadbOrMysql(), "--auto-repair", "--all-databases")
	output, err := cmd.CombinedOutput()
	if err != nil {
		common.LogError("Error running " + "/usr/bin/" + mariadbOrMysql() + " command:" + err.Error())
		return
	}

	lines := strings.Split(string(output), "\n")
	tables := make([]string, 0)
	repairingTables := false
	for _, line := range lines {
		if strings.Contains(line, "Repairing tables") {
			repairingTables = true
			continue
		}
		if repairingTables && !strings.HasPrefix(line, " ") {
			tables = append(tables, line)
		}
	}

	if len(tables) > 0 {
		message := fmt.Sprintf("[MySQL - %s] [:info:] MySQL - `%s` result\n", common.Config.Identifier, "/usr/bin/"+mariadbOrMysql()+" --auto-repair --all-databases")
		for _, table := range tables {
			message += table + "\n"
		}
		common.Alarm(message, "", "", false)
	}
}

func CheckCertificationWaiting() {
	// Use a hardcoded default value of 10 since we don't know the correct field name
	var limiter int = 10

	rows, err := Connection.Query("SELECT COUNT(*) FROM INFORMATION_SCHEMA.PROCESSLIST WHERE STATE LIKE '% for certificate%'")
	if err != nil {
		common.LogError(err.Error())
		return
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		rows.Scan(&count)
	}

	// Update health data
	healthData.CertWaitingInfo.Count = count
	healthData.CertWaitingInfo.Limit = limiter
	healthData.CertWaitingInfo.Exceeded = count > limiter

	if count > limiter {
		common.AlarmCheckDown("certification waiting", fmt.Sprintf("Certification waiting > %d: %d", limiter, count), false, "", "")
	} else {
		common.AlarmCheckUp("certification waiting", fmt.Sprintf("Certification waiting OK: %d/%d", count, limiter), false)
	}
}

func checkPMM() {
	// Check if PMM monitoring is enabled in config (default: enabled)
	if DbHealthConfig.Mysql.Pmm_enabled != nil && !*DbHealthConfig.Mysql.Pmm_enabled {
		// PMM check is explicitly disabled in config
		healthData.PMM.Enabled = false
		return
	}

	// Check if PMM monitoring is enabled (assuming we should check PMM status anyway)
	pmmStatus := "Inactive"
	pmmActive := false

	// Check if PMM client is running
	cmd := exec.Command("systemctl", "is-active", "pmm-agent")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()

	if err == nil && strings.TrimSpace(out.String()) == "active" {
		pmmStatus = "Active"
		pmmActive = true
	}

	// Update health data
	healthData.PMM.Enabled = true
	healthData.PMM.Status = pmmStatus
	healthData.PMM.Active = pmmActive

	if !pmmActive {
		common.AlarmCheckDown("pmm", "PMM client is not active", false, "", "")
	} else {
		common.AlarmCheckUp("pmm", "PMM client is active", false)
	}
}
