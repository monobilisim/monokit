//go:build linux

package mysqlHealth

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/go-ini/ini"
	_ "github.com/go-sql-driver/mysql"
	"github.com/monobilisim/monokit/common"
)

// mariadbOrMysql returns "mariadb" if mysql is not found, otherwise it returns "mysql"
func mariadbOrMysql() string {
	_, err := exec.LookPath("/usr/bin/mysql")
	if err != nil {
		return "mariadb"
	}
	return "mysql"
}

// FindMyCnf returns the path to the my.cnf file
func FindMyCnf() []string {
	cmd := exec.Command("/usr/sbin/"+mariadbOrMysql()+"d", "--verbose", "--help")
	output, err := cmd.CombinedOutput()
	if err != nil {
		common.LogError("Error running " + "/usr/sbin/" + mariadbOrMysql() + "d command:" + err.Error())
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

// checkConnection attempts to connect and ping the database, returns true if successful
func checkConnection(s *ini.Section, finalConn string, path string) bool {
	err := Connect(finalConn)
	if err == nil {
		err = Connection.Ping()
		if err == nil {
			if os.Getenv("MONOKIT_DEBUG") == "1" || os.Getenv("MONOKIT_DEBUG") == "true" {
				fmt.Println("Connected to MySQL with profile: " + s.Name())
				fmt.Println("Connection string: " + finalConn)
				fmt.Println("MyCnf path: " + path)
			}
			return true
		} else {
			if os.Getenv("MONOKIT_DEBUG") == "1" || os.Getenv("MONOKIT_DEBUG") == "true" {
				fmt.Println("Error pinging MySQL with profile: " + s.Name())
				fmt.Println("Connection string: " + finalConn)
				fmt.Println("MyCnf path: " + path)
				fmt.Println("Error: " + err.Error())
			}
		}
	}
	return false
}

// ParseMyCnfAndConnect parses the my.cnf file and connects to the database
func ParseMyCnfAndConnect(profile string) (string, error) {
	// Set default values
	host := ""
	port := "3306"
	dbname := ""
	user := "root"
	password := ""
	socket := ""
	var found bool

	var finalConn string

	for _, path := range FindMyCnf() {
		if _, err := os.Stat(path); err == nil {
			// Load the config file
			cfg, err := ini.LoadSources(ini.LoadOptions{AllowBooleanKeys: true}, path)
			if err != nil {
				return "", fmt.Errorf("error loading config file %s: %w", path, err)
			}

			for _, s := range cfg.Sections() {
				// If profile is set and the section name does not contain the profile name, skip it
				if profile != "" && !strings.Contains(s.Name(), profile) {
					continue
				}

				// Override default values if they exist in config
				params := map[string]*string{
					"host":     &host,
					"port":     &port,
					"dbname":   &dbname,
					"user":     &user,
					"password": &password,
					"socket":   &socket,
				}

				for key, ptr := range params {
					if val := s.Key(key).String(); val != "" {
						*ptr = val
					}
				}

				// Create final connection string based on socket and password
				if password == "" && socket != "" {
					finalConn = fmt.Sprintf("%s@unix(%s)/%s", user, socket, dbname)
				} else if socket != "" {
					finalConn = fmt.Sprintf("%s:%s@unix(%s)/%s", user, password, socket, dbname)
				} else {
					finalConn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", user, password, host, port, dbname)
				}

				// Check if the connection is successful if it is stop searching
				if checkConnection(s, finalConn, path) {
					found = true
					break
				}
			}
		}
	}

	if !found {
		return "", fmt.Errorf("no matching entry found for profile %s", profile)
	}

	return finalConn, nil
}

// Connect connects to the database
func Connect(connStr string) error {
	db, err := sql.Open("mysql", connStr)
	if err != nil {
		return err
	}

	Connection = db
	return nil
}

// InaccessibleClusters checks if the database is accessible from other nodes in the cluster
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
