//go:build linux

package mysqlHealth

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/monobilisim/monokit/common"
	issues "github.com/monobilisim/monokit/common/redmine/issues"
)

// SelectNow checks if the connection is working
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

// CheckProcessCount checks the number of processes running on the database
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

// CheckDB checks if the database is healthy
// and creates alarm if it is not healthy
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

// GetClusterSize returns the current cluster size from MySQL
func GetClusterSize() (int, string, error) {
	var varname string
	var cluster_size int

	rows := Connection.QueryRow("SHOW STATUS WHERE Variable_name = 'wsrep_cluster_size'")

	if err := rows.Scan(&varname, &cluster_size); err != nil {
		return 0, "", fmt.Errorf("error querying database for cluster size: %w", err)
	}

	return cluster_size, varname, nil
}

// CheckClusterStatus checks the cluster status
// and updates the redmine issue if the cluster size is not accurate
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

	cluster_size, varname, err := GetClusterSize()
	if err != nil {
		common.LogError(err.Error())
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

	createClusterSizeIssue(cluster_size, identifierRedmine, varname)
}

// createClusterSizeIssue creates a Redmine issue if the cluster size is 1 or greater than configured size
func createClusterSizeIssue(cluster_size int, identifierRedmine string, varname string) {
	if cluster_size == 1 || cluster_size > DbHealthConfig.Mysql.Cluster.Size {
		issueIdIfExists := issues.Exists("MySQL Cluster boyutu: "+strconv.Itoa(cluster_size)+" - "+identifierRedmine, "", false)

		if _, err := os.Stat(common.TmpDir + "/mysql-cluster-size-redmine.log"); err == nil && issueIdIfExists == "" {
			common.WriteToFile(common.TmpDir+"/mysql-cluster-size-redmine.log", issueIdIfExists)
		}

		issues.CheckDown("cluster-size", "MySQL Cluster boyutu: "+strconv.Itoa(cluster_size)+" - "+identifierRedmine,
			"MySQL Cluster boyutu: "+strconv.Itoa(cluster_size)+" - "+common.Config.Identifier+"\n`"+varname+": "+strconv.Itoa(cluster_size)+"`",
			false, 0)
	}
}

// CheckNodeStatus checks the node status
// and creates alarm if the node is not ready
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

// CheckClusterSynced checks if the cluster is synced
// and creates alarm if it is not synced
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
