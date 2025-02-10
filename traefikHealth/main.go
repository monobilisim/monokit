//go:build linux

package traefikHealth

import (
	"os"
	"fmt"
	"time"
	"bufio"
	"strings"
	"io/ioutil"
	"encoding/json"
	"github.com/spf13/cobra"
	"github.com/monobilisim/monokit/api"
	"github.com/monobilisim/monokit/common"
)

type TraefikHealth struct {
	Types_To_Check []string
	Ports_To_Check []uint32
}

var TraefikHealthConfig TraefikHealth

func LogChecker(file string, lastRunStr string, currentTimeStr string, typesToCheck []string) {
	// Get logs between last run and now using ioutil and json.Unmarshal
	// Read file
	logs, err := os.Open(file)
	if err != nil {
		common.LogError("Error reading file: " + err.Error())
		return
	}

	// Read logs
	logsScanner := bufio.NewScanner(logs)

	// Parse lastRun
	lastRun, err := time.Parse("2006-01-02T15:04:05-07:00", lastRunStr)
	if err != nil {
		common.LogError("Error parsing last run time: " + err.Error())
		return
	}
	
	// Parse currentTime
	currentTime, err := time.Parse("2006-01-02T15:04:05-07:00", currentTimeStr)
	if err != nil {
		common.LogError("Error parsing current time: " + err.Error())
		return
	}

	// Loop through logs
	for logsScanner.Scan() {
		log := strings.TrimSpace(logsScanner.Text())

		// Skip empty logs
		if log == "" {
			continue
		}

		// Check if log is between lastRun and currentTime
		// If so, parse JSON and check for error or warning

		// Parse JSON
		var logJSON map[string]interface{}

		err := json.Unmarshal([]byte(log), &logJSON)

		if err != nil {
			fmt.Println(log)
			common.LogError("Error parsing JSON: " + err.Error())
			continue
		}

		// Check if log is between lastRun and currentTime
		logTime, err := time.Parse("2006-01-02T15:04:05-07:00", logJSON["time"].(string))
		if err != nil {
			common.LogError("Error parsing log time: " + err.Error())
			continue
		}

		for _, typeToCheck := range typesToCheck {
			if logTime.After(lastRun) && logTime.Before(currentTime) {
				// Check for error or warning
				if logJSON["level"] == typeToCheck {
					common.PrettyPrintStr(logJSON["time"].(string), true, "a/an " + typeToCheck)
					
					extraMsg := ""

					if logJSON["error"] != nil {
						extraMsg = ", with error message: " + strings.Replace(logJSON["error"].(string), "\"", "'", -1)
					}

					if logJSON["providerName"] != nil {
						extraMsg = extraMsg + " and with provider name: " + strings.Replace(logJSON["providerName"].(string), "\"", "'", -1)
					}

					if logJSON["domains"] != nil {
						extraMsg = extraMsg + " and with domains: " + strings.Replace(logJSON["domains"].(string), "\"", "\"", -1)
					}

					common.Alarm("[ " + common.ScriptName + " - " + common.Config.Identifier + " ] [:red_circle:] Traefik has had a/an " + typeToCheck + " with message: " + strings.Replace(logJSON["message"].(string), "\"", "'", -1) + extraMsg + " at " + logJSON["time"].(string), "", "", false)
				}
			}
		}
	}
}

func Main(cmd *cobra.Command, args []string) {
	version := "0.1.0"
	common.ScriptName = "traefikHealth"
	common.TmpDir = common.TmpDir + "traefikHealth"
	common.Init()

	if common.ConfExists("traefik") {
		common.ConfInit("traefik", &TraefikHealthConfig)
	}
	
	if len(TraefikHealthConfig.Types_To_Check) == 0 {
		TraefikHealthConfig.Types_To_Check = []string{"error"}
	}

	if len(TraefikHealthConfig.Ports_To_Check) == 0 {
		TraefikHealthConfig.Ports_To_Check = []uint32{80, 443}
	}
    
    api.WrapperGetServiceStatus("traefikHealth")

	fmt.Println("Traefik Health - v" + version + " - " + time.Now().Format("2006-01-02 15:04:05"))

	common.SplitSection("Service")

	if !common.SystemdUnitActive("traefik.service") {
		common.PrettyPrintStr("Service traefik", false, "active")
		common.AlarmCheckDown("traefik_svc", "Service traefik is not active", false, "", "")
	} else {
		common.PrettyPrintStr("Service traefik", true, "active")
		common.AlarmCheckUp("traefik_svc", "Service traefik is now active", false)
	}

	common.SplitSection("Ports")
	
	ports := common.ConnsByProcMulti("traefik")
	
	for _, port := range TraefikHealthConfig.Ports_To_Check {
		if common.ContainsUint32(port, ports) {
			common.PrettyPrintStr("Port "+fmt.Sprint(port), true, "open")
			common.AlarmCheckUp("traefik_port_"+fmt.Sprint(port), "Port "+fmt.Sprint(port)+" is open", false)
		} else {
			common.PrettyPrintStr("Port "+fmt.Sprint(port), false, "closed")
			common.AlarmCheckDown("traefik_port_"+fmt.Sprint(port), "Port "+fmt.Sprint(port)+" is closed", false, "", "")
		}
	}

	// Format current time for logcheck
	currentTime := time.Now().Format("2006-01-02T15:04:05-07:00")

	common.SplitSection("Logcheck")

	// We need to check /var/log/traefik/traefik.log for JSON that came between last run and now
	
	// Check if last run time file exists
	if _, err := os.Stat(common.TmpDir + "/lastRun"); os.IsNotExist(err) {
		common.WriteToFile(common.TmpDir + "/lastRun", currentTime)
		return
	}

	// Read last run time from TmpDir + "/lastRun"
	lastRun, err := ioutil.ReadFile(common.TmpDir + "/lastRun")
	if err != nil {
		common.LogError("Error reading last run time: " + err.Error())
		return
	}

	// Get logs between last run and now using ioutil and json.Unmarshal
	LogChecker("/var/log/traefik/traefik.json", string(lastRun), currentTime, TraefikHealthConfig.Types_To_Check)

	// Write currentTime to TmpDir + "/lastRun"
	common.WriteToFile(common.TmpDir + "/lastRun", currentTime)
}
