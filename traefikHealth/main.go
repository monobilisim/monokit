//go:build linux

package traefikHealth

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
	api "github.com/monobilisim/monokit/common/api"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

// DetectTraefik checks if Traefik seems to be installed.
// It checks for the systemd service unit file and log directory.
func DetectTraefik() bool {
	// 1. Check if traefik.service unit file exists
	if !common.SystemdUnitExists("traefik.service") {
		log.Debug().Msg("traefikHealth auto-detection failed: traefik.service unit file not found.")
		return false
	}
	log.Debug().Msg("traefikHealth auto-detection: traefik.service unit file found.")

	// 2. Check for Traefik log directory
	logDir := "/var/log/traefik"
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		log.Debug().Msg(fmt.Sprintf("traefikHealth auto-detection failed: Log directory not found at %s", logDir))
		return false
	}
	log.Debug().Msg(fmt.Sprintf("traefikHealth auto-detection: Found log directory: %s", logDir))

	log.Debug().Msg("traefikHealth auto-detected successfully (service active and log directory exists).")
	return true
}

func init() {
	common.RegisterComponent(common.Component{
		Name:       "traefikHealth",
		EntryPoint: Main,
		Platform:   "linux",
		AutoDetect: DetectTraefik, // Add the auto-detect function here
	})
}

type TraefikHealth struct {
	Types_To_Check []string
	Ports_To_Check []uint32
}

var TraefikHealthConfig TraefikHealth
var healthData *TraefikHealthData

func checkLogs(file string, lastRunStr string, currentTimeStr string, typesToCheck []string) {
	// Get logs between last run and now using ioutil and json.Unmarshal
	// Read file
	logs, err := os.Open(file)
	if err != nil {
		log.Error().Err(err).Msg("Error reading file")
		return
	}
	defer logs.Close()

	// Read logs
	logsScanner := bufio.NewScanner(logs)

	// Parse lastRun
	lastRun, err := time.Parse("2006-01-02T15:04:05-07:00", lastRunStr)
	if err != nil {
		log.Error().Err(err).Msg("Error parsing last run time")
		return
	}

	// Parse currentTime
	currentTime, err := time.Parse("2006-01-02T15:04:05-07:00", currentTimeStr)
	if err != nil {
		log.Error().Err(err).Msg("Error parsing current time")
		return
	}

	// Loop through logs
	for logsScanner.Scan() {
		logLine := strings.TrimSpace(logsScanner.Text())

		// Skip empty logs
		if logLine == "" {
			continue
		}

		// Parse JSON
		var logJSON map[string]interface{}
		err := json.Unmarshal([]byte(logLine), &logJSON)
		if err != nil {
			log.Error().Err(err).Msg("Error parsing JSON")
			continue
		}

		// Check if time field exists
		timeValue, exists := logJSON["time"]
		if !exists || timeValue == nil {
			log.Error().Msg("Log entry missing 'time' field")
			continue
		}

		// Check if time field is a string
		timeStr, ok := timeValue.(string)
		if !ok {
			log.Error().Msg("Log 'time' field is not a string")
			continue
		}

		// Check if log is between lastRun and currentTime
		logTime, err := time.Parse("2006-01-02T15:04:05-07:00", timeStr)
		if err != nil {
			log.Error().Err(err).Msg("Error parsing log time")
			continue
		}

		for _, typeToCheck := range typesToCheck {
			levelValue, levelExists := logJSON["level"]
			if !levelExists || levelValue == nil {
				continue
			}

			level, ok := levelValue.(string)
			if !ok || level != typeToCheck {
				continue
			}

			if logTime.After(lastRun) && logTime.Before(currentTime) {
				// Create a log entry
				entry := LogEntry{
					Time:  timeStr,
					Level: level,
				}

				// Check if message exists and is not empty
				hasValidMessage := false
				if messageValue, exists := logJSON["message"]; exists && messageValue != nil {
					if messageStr, ok := messageValue.(string); ok && messageStr != "" {
						entry.Message = messageStr
						hasValidMessage = true
					} else {
						entry.Message = "Unknown message"
					}
				} else {
					entry.Message = "No message"
				}

				// Skip entries without meaningful messages
				if !hasValidMessage {
					continue
				}

				// Add error message if it exists
				if errorValue, exists := logJSON["error"]; exists && errorValue != nil {
					if errorStr, ok := errorValue.(string); ok {
						entry.Error = errorStr
					}
				}

				// Add provider name if it exists
				if providerValue, exists := logJSON["providerName"]; exists && providerValue != nil {
					if providerStr, ok := providerValue.(string); ok {
						entry.Provider = providerStr
					}
				}

				// Add domains if they exist
				if domainsValue, exists := logJSON["domains"]; exists && domainsValue != nil {
					if domainsStr, ok := domainsValue.(string); ok {
						entry.Domains = domainsStr
					}
				}

				// Add to appropriate list and update health data
				extraMsg := ""
				if entry.Error != "" {
					extraMsg = ", with error message: " + strings.Replace(entry.Error, "\"", "'", -1)
				}
				if entry.Provider != "" {
					extraMsg = extraMsg + " and with provider name: " + strings.Replace(entry.Provider, "\"", "'", -1)
				}
				if entry.Domains != "" {
					extraMsg = extraMsg + " and with domains: " + strings.Replace(entry.Domains, "\"", "\"", -1)
				}

				// Send alarm
				common.Alarm("[ "+common.ScriptName+" - "+common.Config.Identifier+" ] [:red_circle:] Traefik has had a/an "+typeToCheck+" with message: "+strings.Replace(entry.Message, "\"", "'", -1)+extraMsg+" at "+entry.Time, "", "", false)

				// Store in appropriate list
				if typeToCheck == "error" {
					healthData.Logs.Errors = append(healthData.Logs.Errors, entry)
					healthData.IsHealthy = false
					healthData.Logs.HasIssues = true
				} else if typeToCheck == "warning" {
					healthData.Logs.Warnings = append(healthData.Logs.Warnings, entry)
					healthData.Logs.HasIssues = true
				}
			}
		}
	}
}

func checkService() {
	serviceActive := common.SystemdUnitActive("traefik.service")
	healthData.Service.Active = serviceActive

	if !serviceActive {
		common.AlarmCheckDown("traefik_svc", "Service traefik is not active", false, "", "")
		healthData.IsHealthy = false
	} else {
		common.AlarmCheckUp("traefik_svc", "Service traefik is now active", false)
	}
}

func checkPorts() {
	ports := common.ConnsByProcMulti("traefik")
	healthData.Ports.AllPortsOK = true

	for _, port := range TraefikHealthConfig.Ports_To_Check {
		portOpen := common.ContainsUint32(port, ports)
		healthData.Ports.PortStatus[port] = portOpen

		if portOpen {
			common.AlarmCheckUp("traefik_port_"+fmt.Sprint(port), "Port "+fmt.Sprint(port)+" is open", false)
		} else {
			common.AlarmCheckDown("traefik_port_"+fmt.Sprint(port), "Port "+fmt.Sprint(port)+" is closed", false, "", "")
			healthData.Ports.AllPortsOK = false
			healthData.IsHealthy = false
		}
	}
}

func Main(cmd *cobra.Command, args []string) {
	version := "0.2.0" // Updated version number
	common.ScriptName = "traefikHealth"
	common.TmpDir = common.TmpDir + "traefikHealth"
	common.Init()

	// Initialize health data
	healthData = NewTraefikHealthData()
	healthData.Version = version

	// Initialize config
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

	// Check service status
	checkService()

	// Check port status
	checkPorts()

	// Format current time for logcheck
	currentTime := time.Now().Format("2006-01-02T15:04:05-07:00")
	healthData.Logs.LastChecked = currentTime

	// Check if last run time file exists
	if _, err := os.Stat(common.TmpDir + "/lastRun"); os.IsNotExist(err) {
		common.WriteToFile(common.TmpDir+"/lastRun", currentTime)
	} else {
		// Read last run time from TmpDir + "/lastRun"
		lastRun, err := ioutil.ReadFile(common.TmpDir + "/lastRun")
		if err != nil {
			log.Error().Err(err).Msg("Error reading last run time")
		} else {
			// Get logs between last run and now
			checkLogs("/var/log/traefik/traefik.json", string(lastRun), currentTime, TraefikHealthConfig.Types_To_Check)
		}
	}

	// Write currentTime to TmpDir + "/lastRun"
	common.WriteToFile(common.TmpDir+"/lastRun", currentTime)

	// Render the health data
	fmt.Println(healthData.RenderAll())
}
