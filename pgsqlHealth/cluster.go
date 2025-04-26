//go:build linux

// This file implements PostgreSQL cluster health monitoring functionality using Patroni
//
// It provides functions to:
// - Check Patroni cluster status and service health
// - Monitor cluster members and their states
// - Track cluster size and configuration
// - Generate alerts for cluster issues
//
// The main functions are:
// - clusterStatus(): Checks overall cluster health and Patroni service status
// - manipulatePatroniListOutput(): Formats Patroni cluster member list output
package pgsqlHealth

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
	db "github.com/monobilisim/monokit/common/db"
	issues "github.com/monobilisim/monokit/common/redmine/issues"
)

// clusterStatus performs the following steps:
// 1. Checks if the Patroni service is running
// 2. Retrieves the current and previous cluster statuses
// 3. Checks the role of the current node
// 4. Checks for changes in the cluster roles
// 5. Checks the state of each cluster member
// 6. Saves the current cluster status to a JSON file for future comparison
func clusterStatus(patroniApiUrl string, dbConfig db.DbHealth) { // Added parameters
	checkPatroniService()

	result, oldResult := getClusterStatus(patroniApiUrl) // Pass patroniApiUrl
	if result == nil {
		return
	}

	checkThisNodeRole(result)
	checkClusterRoleChanges(result, oldResult, dbConfig) // Pass dbConfig
	checkClusterStates(result)

	// Save current state for future comparison
	saveOutputJSON(result)
}

// checkPatroniService checks if the Patroni service is running
// and updates the alarm status accordingly
func checkPatroniService() {
	if common.SystemdUnitActive("patroni.service") {
		common.PrettyPrintStr("Patroni Service", true, "accessible")
		common.AlarmCheckUp("patroni_service", "Patroni service is now accessible", false)
	} else {
		common.PrettyPrintStr("Patroni Service", false, "accessible")
		common.AlarmCheckDown("patroni_service", "Patroni service is not accessible", false, "", "")
	}
}

// getClusterStatus retrieves the cluster status from the Patroni API
// and returns the current and previous cluster statuses
func getClusterStatus(patroniApiUrl string) (*Response, *Response) { // Added parameter
	client := &http.Client{Timeout: time.Second * 10}
	clusterURL := "http://" + patroniApiUrl + "/cluster" // Use passed parameter

	resp, err := client.Get(clusterURL)
	if err != nil {
		common.LogError(fmt.Sprintf("Error executing query: %s - Error: %v\n", clusterURL, err))
		common.PrettyPrintStr("Patroni API", false, "accessible")
		return nil, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		common.PrettyPrintStr("Patroni API", false, "accessible")
		return nil, nil
	}
	common.PrettyPrintStr("Patroni API", true, "accessible")

	var result Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		common.LogError(fmt.Sprintf("Error decoding JSON: %v\n", err))
		return nil, nil
	}

	oldResult := loadOldResult()
	return &result, oldResult
}

// loadOldResult loads the previous cluster status from the JSON file
// and returns it
func loadOldResult() *Response {
	outputJSON := common.TmpDir + "/raw_output.json"
	if _, err := os.Stat(outputJSON); err == nil {
		oldOutput, err := os.ReadFile(outputJSON)
		if err != nil {
			common.LogError(fmt.Sprintf("Error reading file: %v\n", err))
			return nil
		}
		var oldResult Response
		if err := json.Unmarshal(oldOutput, &oldResult); err != nil {
			log.Fatal("Error during Unmarshal(): ", err)
		}
		return &oldResult
	}
	return nil
}

// checkThisNodeRole checks the role of the current node
// and prints the result
func checkThisNodeRole(result *Response) {
	for _, member := range result.Members {
		if member.Name == nodeName {
			common.PrettyPrintStr("This node", true, member.Role)
			break
		}
	}
}

// handleLeaderSwitch handles the leader switch event
// by running the leader switch hook if it is configured
func handleLeaderSwitch(member Member, client *http.Client, dbConfig db.DbHealth) { // Added dbConfig parameter
	if dbConfig.Postgres.Leader_switch_hook == "" { // Use passed dbConfig
		return
	}

	/*req, err := http.NewRequest("GET", member.APIURL, nil)
	if err != nil {
		common.LogError(fmt.Sprintf("Error creating request: %v\n", err))
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		common.LogError(fmt.Sprintf("Error executing request: %v\n", err))
		return
	}
	defer resp.Body.Close()

	var role map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&role); err != nil {
		common.LogError(fmt.Sprintf("Error decoding JSON: %v\n", err))
		return
	}

	if role["role"] == "leader" {
		runLeaderSwitchHook()
	}*/

	runLeaderSwitchHook(dbConfig) // Pass dbConfig
}

// runLeaderSwitchHook runs the leader switch hook
// and logs the result
func runLeaderSwitchHook(dbConfig db.DbHealth) { // Added dbConfig parameter
	cmd := exec.Command("sh", "-c", dbConfig.Postgres.Leader_switch_hook) // Use passed dbConfig
	if err := cmd.Run(); err != nil {
		common.LogError(fmt.Sprintf("Error running leader switch hook: %v\n", err))
		common.Alarm("[ Patroni - "+common.Config.Identifier+" ] [:red_circle:] Error running leader switch hook: "+err.Error(), "", "", false)
	} else {
		common.Alarm("[ Patroni - "+common.Config.Identifier+" ] [:check:] Leader switch hook has been run successfully!", "", "", false)
	}
}

// checkClusterRoleChanges checks for changes in the cluster roles
// and logs the changes
func checkClusterRoleChanges(result, oldResult *Response, dbConfig db.DbHealth) { // Added dbConfig parameter
	common.SplitSection("Cluster Roles:")
	for _, member := range result.Members {
		common.PrettyPrintStr(member.Name, true, member.Role)
		if reflect.DeepEqual(*oldResult, (Response{})) {
			continue
		}
		for _, oldMember := range oldResult.Members {
			if oldMember.Name == member.Name {
				if oldMember.Role != member.Role {
					common.PrettyPrintStr(member.Name, true, oldMember.Role+" -> "+member.Role)
					if oldMember.Name == nodeName {
						common.Alarm("[ Patroni - "+common.Config.Identifier+" ] [:info:] Role of "+member.Name+" has changed! Old: **"+oldMember.Role+"** New: **"+member.Role+"**", "", "", false)
					}
					if member.Role == "leader" {
						common.Alarm("[ Patroni - "+common.Config.Identifier+" ] [:check:] "+member.Name+" is now the leader!", "", "", false)
						// Need dbConfig here, but checkClusterRoleChanges doesn't have it yet.
						// This will be fixed in the next step by adding dbConfig to checkClusterRoleChanges.
						// Pass the actual dbConfig received by this function
						handleLeaderSwitch(member, &http.Client{}, dbConfig)
					}
				}
			}
		}
	}
}

// checkClusterStates checks the state of each cluster member
// and logs the results
func checkClusterStates(result *Response) {
	oldOutputFile := common.TmpDir + "/old_raw_output.json"

	common.SplitSection("Cluster States:")
	var runningClusters []Member
	var stoppedClusters []Member
	for _, member := range result.Members {
		if member.State == "running" || member.State == "streaming" {
			common.PrettyPrintStr(member.Name, true, member.State)
			common.AlarmCheckUp("patroni_size", "Node "+member.Name+" state: "+member.State, false)
			runningClusters = append(runningClusters, member)
		} else {
			fmt.Println(common.Blue + member.Name + common.Reset + " is " + common.Fail + member.State + common.Reset)
			common.AlarmCheckDown("patroni_size", "Node "+member.Name+" state: "+member.State, false, "", "")
			stoppedClusters = append(stoppedClusters, member)
		}
	}
	rcLen := strconv.Itoa(len(runningClusters))
	cmd := exec.Command("patronictl", "list")
	out, listErr := cmd.Output()
	var listTable string

	if listErr != nil {
		common.LogError(fmt.Sprintf("Error reading file: %v\n", listErr))
		listTable = fmt.Sprintf("Couln't get tables from command `patronictl list`\n Error: %v", listErr)
	} else {
		listTable = manipulatePatroniListOutput(string(out))
	}

	if _, err := os.Stat(oldOutputFile); err == nil {
		var oldResult Response
		oldOutput, err := os.ReadFile(oldOutputFile)
		if err != nil {
			common.LogError(fmt.Sprintf("Error reading file: %v\n", err))
			return
		}
		err = json.Unmarshal(oldOutput, &oldResult)
		clusterLen := strconv.Itoa(len(oldResult.Members))
		if len(runningClusters) <= 1 {
			issues.CheckDown("cluster_size_issue", "Patroni Cluster Size: "+rcLen+"/"+clusterLen, "Patroni cluster size: "+rcLen+"/"+clusterLen+"\n"+listTable, false, 0)
		}
		if err != nil {
			log.Fatal("Error during Unmarshal(): ", err)
		}

		if len(oldResult.Members) == len(result.Members) {
			issues.CheckUp("cluster_size_issue", "Patroni cluster size returnerd to normal: "+rcLen+"/"+clusterLen+"\n"+listTable)
			err := os.Remove(oldOutputFile)
			if err != nil {
				common.LogError(fmt.Sprintf("Error deleting file: %v\n", err))
			}
		} else {
			issues.Update("cluster_size_issue", "Patroni cluster size: "+rcLen+"/"+clusterLen+"\n"+listTable, true)
		}

	} else {
		var rslt Response
		if result != nil {
			rslt = *result // Properly dereference the pointer
		}
		if len(stoppedClusters) > 0 {
			f, err := os.Create(oldOutputFile)
			if err != nil {
				common.LogError(fmt.Sprintf("Error creating file: %v\n", err))
				return
			}
			defer f.Close()
			encoder := json.NewEncoder(f)
			encoder.Encode(rslt)
		}
		clusterLen := strconv.Itoa(len(rslt.Members))
		if len(runningClusters) <= 1 {
			issues.CheckDown("cluster_size_issue", "Patroni Cluster Size: "+rcLen+"/"+clusterLen, "Patroni cluster size: "+rcLen+"/"+clusterLen+"\n"+listTable, false, 0)
		}
	}
}

// saveOutputJSON saves the current cluster status to a JSON file
// for future comparison
func saveOutputJSON(result *Response) {
	outputJSON := common.TmpDir + "/raw_output.json"
	f, err := os.Create(outputJSON)
	if err != nil {
		common.LogError(fmt.Sprintf("Error creating file: %v\n", err))
		return
	}
	defer f.Close()
	encoder := json.NewEncoder(f)
	encoder.Encode(result)
}

// manipulatePatroniListOutput manipulates the output of the patroni list command
// to make it more readable
func manipulatePatroniListOutput(output string) string {
	lines := strings.Split(output, "\n")
	lines = lines[1:]

	if len(lines) < 2 {
		return strings.Join(lines, "\n")
	}
	lines[1] = "|--|--|--|--|--|--|"

	lines = lines[:len(lines)-2]

	return strings.Join(lines, "\n")
}
