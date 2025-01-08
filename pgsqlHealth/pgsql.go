//go:build linux
package pgsqlHealth

import (
	"os"
	"fmt"
	"log"
	"time"
	"errors"
	"strconv"
    "os/exec"
	"strings"
	"reflect"
	"net/http"
	"database/sql"
	"encoding/json"
	"gopkg.in/yaml.v3"
	_ "github.com/lib/pq"
	"github.com/monobilisim/monokit/common"
    issues "github.com/monobilisim/monokit/common/redmine/issues"
)

var Connection *sql.DB
var nodeName string

type Response struct {
	Members []Member `json:"members"`
	Pause   bool     `json:"pause"`
	Scope   string   `json:"scope"`
}

type Member struct {
	Name     string `json:"name"`
	Role     string `json:"role"`
	State    string `json:"state"`
	APIURL   string `json:"api_url"`
	Host     string `json:"host"`
	Port     int64  `json:"port"`
	Timeline int64  `json:"timeline"`
	Lag      *int64 `json:"lag,omitempty"`
}

func walgVerify() {
    var integrityCheck string
    var timelineCheck string
    verifyOut, err := exec.Command("wal-g", "wal-verify", "integrity", "timeline").Output()
    if err != nil {
        common.LogError(fmt.Sprintf("Error executing command: %v\n", err))
        return
    }

    for _, line := range strings.Split(string(verifyOut), "\n") {
        if strings.Contains(line, "Integrity check status") {
            integrityCheck = strings.Split(line, ": ")[1]
        }
        if strings.Contains(line, "Timeline check status") {
            timelineCheck = strings.Split(line, ": ")[1]
        }
    }
    
    if integrityCheck != "OK" {
        common.PrettyPrintStr("WAL-G integrity check", false, "OK")
        common.AlarmCheckDown("wal_g_integrity_check", "WAL-G integrity check failed, integrity check status: " + integrityCheck, false)
        issues.CheckDown("wal_g_integrity_check", "WAL-G bütünlük kontrolü başarısız oldu", "Bütünlük durumu: " + integrityCheck, false, 0)
    } else {
        common.PrettyPrintStr("WAL-G integrity check", true, "OK")
        common.AlarmCheckUp("wal_g_integrity_check", "WAL-G integrity check is now OK", false)
        issues.CheckUp("wal_g_integrity_check", "WAL-G bütünlük kontrolü başarılı \n Bütünlük durumu: " + integrityCheck)
    }

    if timelineCheck != "OK" {
        common.PrettyPrintStr("WAL-G timeline check", false, "OK")
        common.AlarmCheckDown("wal_g_timeline_check", "WAL-G timeline check failed, timeline check status: " + timelineCheck, false)
        issues.CheckDown("wal_g_timeline_check", "WAL-G timeline kontrolü başarısız oldu", "Timeline durumu: " + timelineCheck, false, 0)
    } else {
        common.PrettyPrintStr("WAL-G timeline check", true, "OK")
        common.AlarmCheckUp("wal_g_timeline_check", "WAL-G timeline check is now OK", false)
        issues.CheckUp("wal_g_timeline_check", "WAL-G timeline kontrolü başarılı \n Timeline durumu: " + timelineCheck)
    }

}

func getPatroniUrl() (string, error) {
	// Read the file
	data, err := os.ReadFile("/etc/patroni/patroni.yml")
	if err != nil {
		common.LogError(fmt.Sprintf("couldn't read patroni config file: %v\n", err))
		return "", err
	}

	// Create a struct to hold the YAML data
	type patroniConf struct {
		Name    string `json:"name"`
		Restapi struct {
			ConnectAddress string `yaml:"connect_address"`
		} `yaml:"restapi"`
	}
	var patroni patroniConf
	// Unmarshal the YAML data into the struct
	err = yaml.Unmarshal(data, &patroni)
	if err != nil {
		common.LogError(fmt.Sprintf("couldn't unmarshal patroni config file: %v\n", err))
		return "", err
	}
	nodeName = patroni.Name

	return patroni.Restapi.ConnectAddress, nil
}

func Connect() error {
	pgPass := "/var/lib/postgresql/.pgpass"
    var psqlConn string
    if _, err := os.Stat(pgPass); err == nil {
	    content, err := os.ReadFile(pgPass)
	    if err != nil {
	    	common.LogError("Error reading file: " + err.Error())
	    	return err
	    }

	    // Split the content into lines
	    lines := strings.Split(string(content), "\n")

	    var host, port, user, password string

	    // Find the line containing "localhost"
	    for _, line := range lines {
	    	if strings.Contains(line, "localhost") {
	    		// Parse the line using colon (:) as a separator
	    		parts := strings.Split(strings.TrimSpace(line), ":")
	    		if len(parts) != 5 {
	    			common.LogError("Invalid .pgpass file format")
	    			return errors.New("invalid .pgpass file format")
	    		}

	    		host = parts[0]
	    		port = parts[1]
	    		user = parts[3]
	    		password = parts[4]

	    		break
	    	}
	    }

	    // connection string
	    psqlConn = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=disable", host, port, user, password)
    } else {
        // Try to do UNIX auth
        psqlConn = "dbname=postgres sslmode=disable host=/var/run/postgresql"
    }


	// open database
	db, err := sql.Open("postgres", psqlConn)
	if err != nil {
		common.LogError("Couldn't connect to postgresql: " + err.Error())
		return err
	}

	err = db.Ping()
	if err != nil {
		common.LogError("Couldn't ping postgresql: " + err.Error())
		return err
	}

	Connection = db
	return nil
}

func uptime() {
	var result int
	query := `SELECT EXTRACT(EPOCH FROM current_timestamp - pg_postmaster_start_time())::int AS uptime_seconds`
	err := Connection.QueryRow(query).Scan(&result)
	if err != nil {
		common.LogError(fmt.Sprintf("Error executing query: %s - Error: %v\n", query, err))
		common.PrettyPrintStr("PostgreSQL uptime", false, "accessible")
		return
	}
	days := result / (24 * 3600)
	hours := (result % (24 * 3600)) / 3600
	minutes := (result % 3600) / 60
	seconds := result % 60

	common.PrettyPrintStr("PostgreSQL uptime", true, fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds))
}

func writeActiveConnections() {
	query := `
		SELECT pid,usename, client_addr, now() - pg_stat_activity.query_start AS duration, query, state
		FROM pg_stat_activity
		WHERE state='active'
		ORDER BY duration DESC;
	`
	//run the query and write its output to a file
	rows, err := Connection.Query(query)
	if err != nil {
		common.LogError(fmt.Sprintf("Error executing query: %s - Error: %v\n", query, err))
		return
	}
	defer rows.Close()

	dayOfWeek := time.Now().Weekday().String()
	logFileName := fmt.Sprintf("/var/log/monodb/pgsql-stat_activity-%s.log", dayOfWeek)
	logFile, err := os.Create(logFileName)
	if err != nil {
		common.LogError("Failed to create log file: " + err.Error())
	}
	defer logFile.Close()

	// Write query results to log file
	_, _ = fmt.Fprintf(logFile, "Query executed at: %v\n\n", time.Now())
	_, _ = fmt.Fprintf(logFile, "%-5s %-10s %-15s %-20s %-50s %-10s\n", "PID", "User", "Client Addr", "Duration", "Query", "State")
	_, _ = fmt.Fprintln(logFile, strings.Repeat("-", 120))

	for rows.Next() {
		var (
			pid        int
			usename    string
			clientAddr string
			duration   string
			query      string
			state      string
		)

		err := rows.Scan(&pid, &usename, &clientAddr, &duration, &query, &state)
		if err != nil {
			common.LogError("Failed to scan row: " + err.Error())
			continue
		}

		_, _ = fmt.Fprintf(logFile, "%-5d %-10s %-15s %-20s %-50s %-10s\n",
			pid, usename, clientAddr, duration, query[:40], state)
	}

}

func activeConnections() {
	var maxConn, used, increase int
	aboveLimitFile := common.TmpDir + "/last-connection-above-limit.txt"

	query := `
		SELECT max_conn, used FROM (SELECT COUNT(*) used
        FROM pg_stat_activity) t1, (SELECT setting::int max_conn FROM pg_settings WHERE name='max_connections') t2
	`
	err := Connection.QueryRow(query).Scan(&maxConn, &used)
	if err != nil {
		common.LogError(fmt.Sprintf("Error executing query: %s - Error: %v\n", query, err))
		common.PrettyPrintStr("PostgreSQL active connections", false, "accessible")
        common.AlarmCheckDown("postgres_active_conn", "An error occurred while checking active connections: " + err.Error(), false)
		return
	} else {
        common.AlarmCheckUp("postgres_active_conn", "Active connections are now accessible", true)
    }

	usedPercent := (used * 100) / maxConn

	if _, err := os.Stat(aboveLimitFile); os.IsNotExist(err) {
		increase = 1
	} else {
		// set increase to the content of the file
		content, err := os.ReadFile(aboveLimitFile)
		if err != nil {
			common.LogError(fmt.Sprintf("Error reading file: %v\n", err))
			increase = 1
		}
		increase = int(content[0])
	}

	if usedPercent >= DbHealthConfig.Postgres.Limits.Conn_percent {
		if _, err := os.Stat(aboveLimitFile); os.IsNotExist(err) {
			writeActiveConnections()

		}
		common.PrettyPrintStr("Number of active connections", true, fmt.Sprintf("%d/%d and above %d", used, maxConn, DbHealthConfig.Postgres.Limits.Conn_percent))
        common.AlarmCheckDown("postgres_num_active_conn", fmt.Sprintf("Number of active connections: %d/%d and above %d", used, maxConn, DbHealthConfig.Postgres.Limits.Conn_percent), false)
		difference := (used - maxConn) / 10
		if difference > increase {
			writeActiveConnections()
			increase = difference + 1
		}
		fmt.Println("-----------------", increase)
		fmt.Println("-----------------", []byte{byte(increase)})
		err = os.WriteFile(aboveLimitFile, []byte(strconv.Itoa(increase)), 0644)
		if err != nil {
			common.LogError(fmt.Sprintf("Error writing file: %v\n", err))
		}
	} else {
		common.PrettyPrintStr("Number of active connections", true, fmt.Sprintf("%d/%d", used, maxConn))
        common.AlarmCheckUp("postgres_num_active_conn", fmt.Sprintf("Number of active connections is now: %d/%d", used, maxConn), true)
		if _, err := os.Stat(aboveLimitFile); err == nil {
			err := os.Remove(aboveLimitFile)
			if err != nil {
				common.LogError(fmt.Sprintf("Error deleting file: %v\n", err))
			}
		}
	}
}

func runningQueries() {
	query := `SELECT COUNT(*) AS active_queries_count FROM pg_stat_activity WHERE state = 'active';`

	var activeQueriesCount int
	err := Connection.QueryRow(query).Scan(&activeQueriesCount)
	if err != nil {
		common.LogError(fmt.Sprintf("Error executing query: %s - Error: %v\n", query, err))
		common.PrettyPrintStr("Number of running queries", false, "accessible")
        common.AlarmCheckDown("postgres_running_queries", "An error occurred while checking running queries: " + err.Error(), false)
		return
	} else {
        common.AlarmCheckUp("postgres_running_queries", "Running queries are now accessible", true)
    }

	if activeQueriesCount > DbHealthConfig.Postgres.Limits.Query {
		common.PrettyPrintStr("Number of running queries", true, fmt.Sprintf("%d/%d", activeQueriesCount, DbHealthConfig.Postgres.Limits.Query))
        common.AlarmCheckDown("postgres_num_running_queries", fmt.Sprintf("Number of running queries: %d/%d", activeQueriesCount, DbHealthConfig.Postgres.Limits.Query), false)
	} else {
		common.PrettyPrintStr("Number of running queries", true, fmt.Sprintf("%d/%d", activeQueriesCount, DbHealthConfig.Postgres.Limits.Query))
        common.AlarmCheckUp("postgres_num_running_queries", fmt.Sprintf("Number of running queries is now: %d/%d", activeQueriesCount, DbHealthConfig.Postgres.Limits.Query), true)
	}
}

func clusterStatus() {
    
    if common.SystemdUnitActive("patroni.service") {
        common.PrettyPrintStr("Patroni Service", true, "accessible")
        common.AlarmCheckUp("patroni_service", "Patroni service is now accessible", false)
    } else {
        common.PrettyPrintStr("Patroni Service", false, "accessible")
        common.AlarmCheckDown("patroni_service", "Patroni service is not accessible", false)
    }

	outputJSON := common.TmpDir + "/raw_output.json"
	clusterURL := "http://" + patroniApiUrl + "/cluster"

	client := &http.Client{
		Timeout: time.Second * 10,
	}

	resp, err := client.Get(clusterURL)
	if err != nil {
		common.LogError(fmt.Sprintf("Error executing query: %s - Error: %v\n", clusterURL, err))
		common.PrettyPrintStr("Patroni API", false, "accessible")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		common.PrettyPrintStr("Patroni API", false, "accessible")
		return
	}
	common.PrettyPrintStr("Patroni API", true, "accessible")

	var result Response
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		common.LogError(fmt.Sprintf("Error decoding JSON: %v\n", err))
		return
	}

	var oldResult Response
	if _, err := os.Stat(outputJSON); err == nil {
		oldOutput, err := os.ReadFile(outputJSON)
		if err != nil {
			common.LogError(fmt.Sprintf("Error reading file: %v\n", err))
			return
		}
		err = json.Unmarshal(oldOutput, &oldResult)
		if err != nil {
			log.Fatal("Error during Unmarshal(): ", err)
		}
	}

	for _, member := range result.Members {
		if member.Name == nodeName {
			common.PrettyPrintStr("This node", true, member.Role)
		}
	}

	common.SplitSection("Cluster Roles:")
	for _, member := range result.Members {
		common.PrettyPrintStr(member.Name, true, member.Role)
		if reflect.DeepEqual(oldResult, (Response{})) {
			continue
		}
		for _, oldMember := range oldResult.Members {
			if oldMember.Name == member.Name {
				if oldMember.Role != member.Role {
					common.PrettyPrintStr(member.Name, true, oldMember.Role+" -> "+member.Role)
					if oldMember.Name == nodeName {
                        common.Alarm("[ Patroni - " + common.Config.Identifier + " ] [:info:] Role of " + member.Name + " has changed! Old: **" + oldMember.Role + "** New: **" + member.Role + "**", "", "", false)
					}
					if member.Role == "leader" {
                        common.Alarm("[ Patroni - " + common.Config.Identifier + " ] [:check:] " + member.Name + " is now the leader!", "", "", false)
						if DbHealthConfig.Postgres.Leader_switch_hook != "" {
                            // send a request to patroniApiUrl and get .role
                            req, err := http.NewRequest("GET", member.APIURL, nil)
                            if err != nil {
                                common.LogError(fmt.Sprintf("Error creating request: %v\n", err))
                                return
                            }

                            resp, err := client.Do(req)

                            if err != nil {
                                common.LogError(fmt.Sprintf("Error executing request: %v\n", err))
                                return
                            }

                            var role map[string]interface{}
                            err = json.NewDecoder(resp.Body).Decode(&role)

                            if err != nil {
                                common.LogError(fmt.Sprintf("Error decoding JSON: %v\n", err))
                                return
                            }

                            if role["role"] == "leader" {
                                cmd := exec.Command(DbHealthConfig.Postgres.Leader_switch_hook)
                                err := cmd.Run()
                                if err != nil {
                                    common.LogError(fmt.Sprintf("Error running leader switch hook: %v\n", err))
                                    common.Alarm("[ Patroni - " + common.Config.Identifier + " ] [:red_circle:] Error running leader switch hook: " + err.Error(), "", "", false)
                                } else {
                                    common.Alarm("[ Patroni - " + common.Config.Identifier + " ] [:check:] Leader switch hook has been run successfully!", "", "", false)
                                }
                            }
						}
					}
				}
			}
		}
	}

	f, err := os.Create(outputJSON)
	if err != nil {
		common.LogError(fmt.Sprintf("Error creating file: %v\n", err))
		return
	}
	defer f.Close()
	encoder := json.NewEncoder(f)
	encoder.Encode(result)
}
