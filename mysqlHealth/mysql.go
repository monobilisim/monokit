//go:build linux

package mysqlHealth

import (
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-ini/ini"
	_ "github.com/go-sql-driver/mysql"      // Keep anonymous import for side effects
	_mysql "github.com/go-sql-driver/mysql" // Import with alias
	"github.com/monobilisim/monokit/common"
	issues "github.com/monobilisim/monokit/common/redmine/issues"
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
				} else if profile == "client" && sectionName == "client" {
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
				} else if host != "" {
					config.Net = "tcp"
					if port != "" {
						config.Addr = fmt.Sprintf("%s:%s", host, port)
					} else {
						config.Addr = fmt.Sprintf("%s:%s", host, "3306") // Use default port if not specified
					}
					// User/Password/DBName already set
				} else {
					// Neither socket nor host defined for this profile section, skip
					common.LogDebug(fmt.Sprintf("Skipping profile [%s] in %s: missing host or socket", s.Name(), path))
					continue
				}

				// Validate required fields before attempting connection
				if config.User == "" {
					common.LogDebug(fmt.Sprintf("Skipping profile [%s] in %s: missing user", s.Name(), path))
					continue
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
	// Simple query to check if the connection is working
	rows, err := Connection.Query("SELECT NOW()")
	if err != nil {
		common.LogError("Error querying database for simple SELECT NOW(): " + err.Error())
		common.AlarmCheckDown("now", "Couldn't run a 'SELECT' statement on MySQL", false, "", "")
		common.PrettyPrintStr("MySQL", false, "accessible")
		return
	}
	defer rows.Close()
	common.AlarmCheckUp("now", "Can run 'SELECT' statements again", false)
	common.PrettyPrintStr("MySQL", true, "accessible")

}

func CheckProcessCount() {
	rows, err := Connection.Query("SHOW PROCESSLIST")
	if err != nil {
		common.LogError("Error querying database for SHOW PROCESSLIST: " + err.Error())
		common.AlarmCheckDown("processlist", "Couldn't run a 'SHOW PROCESSLIST' statement on MySQL", false, "", "")
		common.PrettyPrintStr("Number of Processes", false, "accessible")
		return
	}
	defer rows.Close()
	common.AlarmCheckUp("processlist", "Can run 'SHOW PROCESSLIST' statements again", false)

	// Count the number of processes

	var count int

	for rows.Next() {
		count++
	}

	if count > DbHealthConfig.Mysql.Process_limit {
		common.AlarmCheckDown("processcount", fmt.Sprintf("Number of MySQL processes is over the limit: %d", count), false, "", "")
		common.PrettyPrint("Number of Processes", "", float64(count), false, false, true, float64(DbHealthConfig.Mysql.Process_limit))
	} else {
		common.AlarmCheckUp("processcount", "Number of MySQL processes is under the limit", false)
		common.PrettyPrint("Number of Processes", "", float64(count), false, false, true, float64(DbHealthConfig.Mysql.Process_limit))
	}
}

func InaccessibleClusters() {
	rows := Connection.QueryRow("SHOW STATUS WHERE Variable_name = 'wsrep_incoming_addresses'")

	var ignored string
	var listening_clusters string
	var listening_clusters_array []string

	if err := rows.Scan(&ignored, &listening_clusters); err != nil {
		common.LogError("Error querying database for incoming addresses: " + err.Error())
		return
	}

	listening_clusters_array = strings.Split(listening_clusters, ",")

	if len(listening_clusters_array) == 0 {
		return
	}

	// Check if common.TmpDir + /cluster_nodes exists
	if _, err := os.Stat(common.TmpDir + "/cluster_nodes"); err == nil {
		// If it exists, read the file and compare the contents
		file, err := os.Open(common.TmpDir + "/cluster_nodes")
		if err != nil {
			common.LogError("Error opening file: " + err.Error())
			return
		}
		// Split it and make it into an array
		file_contents := make([]byte, 1024)
		count, err := file.Read(file_contents)
		if err != nil {
			common.LogError("Error reading file: " + err.Error())
			return
		}

		file.Close()

		file_contents_array := strings.Split(string(file_contents[:count]), ",")

		// Compare the two arrays
		for _, cluster := range file_contents_array {
			if common.IsInArray(cluster, listening_clusters_array) {
				common.AlarmCheckUp(cluster, "Node "+cluster+" is back in cluster.", true)
			} else {
				common.AlarmCheckDown(cluster, "Node "+cluster+" is no longer in the cluster.", true, "", "")
			}
		}
	}

	// Create a file with the cluster nodes
	common.WriteToFile(common.TmpDir+"/cluster_nodes", listening_clusters)

}

func CheckClusterStatus() {
	var identifierRedmine string

	// Split the identifier into two parts using a hyphen
	identifierParts := strings.Split(common.Config.Identifier, "-")
	if len(identifierParts) >= 2 {
		identifierRedmine = strings.Join(identifierParts[:2], "-")

		// Check if the identifier is the same as the first two parts
		if common.Config.Identifier == identifierRedmine {
			// Remove all numbers from the end of the string
			re := regexp.MustCompile("[0-9]*$")
			identifierRedmine = re.ReplaceAllString(identifierRedmine, "")
		}

	}

	var varname string
	var cluster_size int

	rows := Connection.QueryRow("SHOW STATUS WHERE Variable_name = 'wsrep_cluster_size'")

	if err := rows.Scan(&varname, &cluster_size); err != nil {
		common.LogError("Error querying database for cluster size: " + err.Error())
		return
	}

	if cluster_size == DbHealthConfig.Mysql.Cluster.Size {
		common.AlarmCheckUp("cluster_size", "Cluster size is accurate: "+fmt.Sprintf("%d", cluster_size)+"/"+fmt.Sprintf("%d", DbHealthConfig.Mysql.Cluster.Size), false)
		issues.CheckUp("cluster-size", "MySQL Cluster boyutu: "+strconv.Itoa(cluster_size)+" - "+common.Config.Identifier+"\n`"+varname+": "+strconv.Itoa(cluster_size)+"`")
		common.PrettyPrint("Cluster Size", "", float64(cluster_size), false, false, true, float64(DbHealthConfig.Mysql.Cluster.Size))
	} else if cluster_size == 0 {
		common.AlarmCheckDown("cluster_size", "Couldn't get cluster size", false, "", "")
		common.PrettyPrintStr("Cluster Size", true, "Unknown")
		issues.Update("cluster-size", "`SHOW STATUS WHERE Variable_name = 'wsrep_cluster_size'` sorgusunda cluster boyutu alınamadı.", true)
	} else {
		common.AlarmCheckDown("cluster_size", "Cluster size is not accurate: "+fmt.Sprintf("%d", cluster_size)+"/"+fmt.Sprintf("%d", DbHealthConfig.Mysql.Cluster.Size), false, "", "")
		issues.Update("cluster-size", "MySQL Cluster boyutu: "+strconv.Itoa(cluster_size)+" - "+common.Config.Identifier+"\n`"+varname+": "+strconv.Itoa(cluster_size)+"`", true)
		common.PrettyPrint("Cluster Size", "", float64(cluster_size), false, false, true, float64(DbHealthConfig.Mysql.Cluster.Size))
	}

	if cluster_size == 1 || cluster_size > DbHealthConfig.Mysql.Cluster.Size {

		issueIdIfExists := issues.Exists("MySQL Cluster boyutu: "+strconv.Itoa(cluster_size)+" - "+identifierRedmine, "", false)

		if _, err := os.Stat(common.TmpDir + "/mysql-cluster-size-redmine.log"); err == nil && issueIdIfExists == "" {
			common.WriteToFile(common.TmpDir+"/mysql-cluster-size-redmine.log", issueIdIfExists)
		}

		issues.CheckDown("cluster-size", "MySQL Cluster boyutu: "+strconv.Itoa(cluster_size)+" - "+identifierRedmine, "MySQL Cluster boyutu: "+strconv.Itoa(cluster_size)+" - "+common.Config.Identifier+"\n`"+varname+": "+strconv.Itoa(cluster_size)+"`", false, 0)
	}
}

func CheckNodeStatus() {
	rows := Connection.QueryRow("SHOW STATUS WHERE Variable_name = 'wsrep_ready'")

	var name string
	var status string

	if err := rows.Scan(&name, &status); err != nil {
		common.LogError("Error querying database for node status: " + err.Error())
		return
	}

	if name == "" && status == "" {
		common.AlarmCheckDown("node_status", "Couldn't get node status", false, "", "")
		common.PrettyPrintStr("Node Status", true, "Unknown")
	} else if status == "ON" {
		common.AlarmCheckUp("node_status", "Node status is 'ON'", false)
		common.PrettyPrintStr("Node Status", true, "ON")
	} else {
		common.AlarmCheckDown("node_status", "Node status is '"+status+"'", false, "", "")
		common.PrettyPrintStr("Node Status", false, "ON")
	}
}

func CheckClusterSynced() {
	rows := Connection.QueryRow("SHOW STATUS WHERE Variable_name = 'wsrep_local_state_comment'")

	var name string
	var status string

	if err := rows.Scan(&name, &status); err != nil {
		common.LogError("Error querying database for local_state_comment: " + err.Error())
		return
	}

	if name == "" && status == "" {
		common.AlarmCheckDown("cluster_synced", "Couldn't get cluster synced status", false, "", "")
		common.PrettyPrintStr("Cluster sync state", true, "Unknown")
	} else if status == "Synced" {
		common.AlarmCheckUp("cluster_synced", "Cluster is synced", false)
		common.PrettyPrintStr("Cluster sync state", true, "Synced")
	} else {
		common.AlarmCheckDown("cluster_synced", "Cluster is not synced, state: "+status, false, "", "")
		common.PrettyPrintStr("Cluster sync state", false, "Synced")
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
	rows, err := Connection.Query("SELECT User, Host, db, State, Info, Time from INFORMATION_SCHEMA.PROCESSLIST where State = 'Waiting for certification' AND Time > 60")
	if err != nil {
		common.LogError("Error querying database for waiting processes: " + err.Error())
		return
	}
	defer rows.Close()

	var waitingCount int
	var waitingProcesses []string

	for rows.Next() {
		var user, host, db, state, info sql.NullString
		var time int

		if err := rows.Scan(&user, &host, &db, &state, &info, &time); err != nil {
			common.LogError("Error scanning row: " + err.Error())
			continue
		}

		waitingCount++
		processInfo := fmt.Sprintf("User: %s, Host: %s, DB: %s, Time: %d seconds",
			user.String, host.String, db.String, time)
		waitingProcesses = append(waitingProcesses, processInfo)
	}

	if waitingCount > 0 {
		message := fmt.Sprintf("Found %d processes waiting for certification for more than 60 seconds", waitingCount)
		common.AlarmCheckDown("certification_waiting", message, false, "", "")

		common.PrettyPrintStr("Long waiting certification processes", false, fmt.Sprintf("%d", waitingCount))

		// Log all waiting processes to the error log
		for _, process := range waitingProcesses {
			common.LogError("Process waiting for certification: " + process)
		}

		// Show first few processes in the output
		for i := 0; i < len(waitingProcesses) && i < 3; i++ {
			common.PrettyPrintStr(fmt.Sprintf("- Process %d", i+1), false, waitingProcesses[i])
		}
	} else {
		common.AlarmCheckUp("certification_waiting", "No processes waiting for certification for more than 60 seconds", false)
		common.PrettyPrintStr("Long waiting certification processes", true, "0")
	}
}

func checkPMM() {
	notInstalled := `
dpkg-query: package 'pmm2-client' is not installed and no information is available
Use dpkg --info (= dpkg-deb --info) to examine archive files.
    `
	dpkgNotFound := `exec: "dpkg": executable file not found in $PATH`
	cmd := exec.Command("dpkg", "-s", "pmm2-client")
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if strings.TrimSpace(stderr.String()) == strings.TrimSpace(notInstalled) || strings.TrimSpace(err.Error()) == strings.TrimSpace(dpkgNotFound) {
			return
		}
		common.LogError(fmt.Sprintf("Error executing dpkg command: %v\n", err))
		return
	}

	output := out.String()
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Status:") {
			status := strings.TrimSpace(strings.Split(line, ":")[1])
			if strings.HasPrefix(status, "install ok installed") {
				common.SplitSection("PMM Status:")
				if common.SystemdUnitActive("pmm-agent.service") {
					common.PrettyPrintStr("Service pmm-agent", true, "active")
					common.AlarmCheckDown("mysql-pmm-agent", "Service pmm-agent", false, "", "")
				} else {
					common.PrettyPrintStr("Service pmm-agent", false, "active")
					common.AlarmCheckUp("mysql-pmm-agent", "Service pmm-agent", false)
				}
			}
			break
		}
	}
}
