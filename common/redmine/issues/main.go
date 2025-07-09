package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/rs/zerolog/log"
)

type Issue struct {
	Id           int    `json:"id,omitempty"`
	Notes        string `json:"notes,omitempty"`
	ProjectId    string `json:"project_id,omitempty"`
	TrackerId    int    `json:"tracker_id,omitempty"`
	Description  string `json:"description,omitempty"`
	Subject      string `json:"subject,omitempty"`
	PriorityId   int    `json:"priority_id,omitempty"`
	StatusId     int    `json:"status_id,omitempty"`
	AssignedToId string `json:"assigned_to_id"`
}

type RedmineIssue struct {
	Issue Issue `json:"issue"`
}

func redmineCheckIssueLog(service string) bool {
	serviceReplaced := strings.Replace(service, "/", "-", -1)
	filePath := common.TmpDir + "/" + serviceReplaced + "-redmine.log"

	// If file exists, return
	if _, err := os.Stat(filePath); err == nil {
		// Check if file is empty, if so delete the file and return
		if common.IsEmptyOrWhitespace(filePath) {
			err := os.Remove(filePath)
			if err != nil {
				log.Error().Err(err).Msg("os.Remove error")
			}
			return false
		}

		// Check if file is 0, if so delete the file and return
		read, err := os.ReadFile(filePath)

		if err != nil {
			log.Error().Err(err).Str("component", "redmine").Str("operation", "check_issue_log").Str("file_path", filePath).Msg("Failed to read issue log file")
		}

		if string(read) == "0" {
			err := os.Remove(filePath)
			if err != nil {
				log.Error().Err(err).Str("component", "redmine").Str("operation", "check_issue_log").Str("file_path", filePath).Msg("Failed to remove zero-content issue log file")
			}
			return false
		}

		return true
	}

	return false
}

func redmineWrapper(service string, subject string, message string) {

	if !redmineCheckIssueLog(service) {
		Create(service, subject, message)
	} else {
		Update(service, message, true)
	}
}

func CheckUp(service string, message string) {
	// Remove slashes from service and replace them with -
	serviceReplaced := strings.Replace(service, "/", "-", -1)
	file_path := common.TmpDir + "/" + serviceReplaced + "-redmine-stat.log"

	// Check if the file exists, close issue and remove file if it does
	if _, err := os.Stat(file_path); err == nil {
		os.Remove(file_path)
		Close(service, message)
	}
}

func CheckDown(service string, subject string, message string, EnableCustomIntervals bool, CustomInterval float64) {
	var interval float64

	if EnableCustomIntervals {
		interval = CustomInterval
	} else {
		interval = common.Config.Redmine.Interval
	}

	// Remove slashes from service and replace them with -
	serviceReplaced := strings.Replace(service, "/", "-", -1)
	filePath := common.TmpDir + "/" + serviceReplaced + "-redmine-stat.log"
	currentDate := time.Now().Format("2006-01-02 15:04:05 -0700")

	// Check if the file exists
	if _, err := os.Stat(filePath); err == nil {
		// Open file and load the JSON

		file, err := os.OpenFile(filePath, os.O_RDONLY, 0644)
		defer file.Close()

		if err != nil {
			log.Error().Err(err).Str("file_path", filePath).Msg("Failed to open file for reading")
		}

		var j common.ServiceFile

		fileRead, err := io.ReadAll(file)

		if err != nil {
			log.Error().Err(err).Str("file_path", filePath).Msg("Failed to read file content")
			return
		}

		err = json.Unmarshal(fileRead, &j)

		if err != nil {
			log.Error().Err(err).Str("file_path", filePath).Msg("Failed to parse JSON from file")
			return
		}

		// Return if locked == true
		if j.Locked {
			return
		}

		oldDate := j.Date
		oldDateParsed, err := time.Parse("2006-01-02 15:04:05 -0700", oldDate)

		if err != nil {
			log.Error().Err(err).Str("date", oldDate).Msg("Failed to parse date from file")
		}

		finJson := &common.ServiceFile{
			Date:   currentDate,
			Locked: true,
		}

		if interval == 0 {
			if oldDateParsed.Format("2006-01-02") != time.Now().Format("2006-01-02") {
				jsonData, err := json.Marshal(&common.ServiceFile{Date: currentDate, Locked: false})

				if err != nil {
					log.Error().Err(err).Msg("Failed to marshal service file JSON")
				}

				_ = os.WriteFile(filePath, jsonData, 0644)

				redmineWrapper(service, subject, message)
			}
			return
		}

		if time.Since(oldDateParsed).Hours() > 24 {
			jsonData, err := json.Marshal(finJson)

			if err != nil {
				log.Error().Err(err).Msg("Failed to marshal service file JSON")
			}

			err = os.WriteFile(filePath, jsonData, 0644)

			if err != nil {
				log.Error().Err(err).Str("file_path", filePath).Msg("Failed to write service file")
			}

			redmineWrapper(service, subject, message)
		} else {
			if !j.Locked {
				// currentDate - oldDate in minutes
				timeDiff := time.Now().Sub(oldDateParsed) //.Minutes()

				if timeDiff.Minutes() >= interval {
					jsonData, err := json.Marshal(finJson)
					if err != nil {
						log.Error().Err(err).Msg("Failed to marshal service file JSON")
					}

					err = os.WriteFile(filePath, jsonData, 0644)

					if err != nil {
						log.Error().Err(err).Str("file_path", filePath).Msg("Failed to write service file")
					}

					redmineWrapper(service, subject, message)
				}
			}
		}
	} else {

		file, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0644)
		defer file.Close()

		if err != nil {
			log.Error().Err(err).Str("file_path", filePath).Msg("Failed to open file for writing")
			return
		}

		jsonData, err := json.Marshal(&common.ServiceFile{Date: currentDate, Locked: false})

		if err != nil {
			log.Error().Err(err).Msg("Failed to marshal service file JSON")
		}

		err = os.WriteFile(filePath, jsonData, 0644)

		if err != nil {
			log.Error().Err(err).Str("file_path", filePath).Msg("Failed to write service file")
		}

		if interval == 0 {
			redmineWrapper(service, subject, message)
		}
	}
}

// Function to check for recent issues
func findRecentIssue(subject string, hoursBack int) string {
	log.Debug().
		Str("component", "redmine").
		Str("operation", "find_recent_issue").
		Str("subject", subject).
		Int("hours_back", hoursBack).
		Msg("Looking for recent issues")

	var projectId string
	if common.Config.Redmine.Project_id == "" {
		projectId = strings.Split(common.Config.Identifier, "-")[0]
	} else {
		projectId = common.Config.Redmine.Project_id
	}

	if !common.Config.Redmine.Enabled {
		log.Debug().Str("component", "redmine").Msg("Redmine integration is disabled")
		return ""
	}

	// Calculate time range
	now := time.Now()
	hoursAgo := now.Add(-time.Duration(hoursBack) * time.Hour)

	// Try different date formats for Redmine API
	dateFormats := []string{
		hoursAgo.Format("2006-01-02"),                 // Just the date
		">=" + hoursAgo.Format("2006-01-02"),          // Date with >=
		hoursAgo.Format("2006-01-02T15:04:05"),        // ISO without timezone
		">=" + hoursAgo.Format("2006-01-02T15:04:05"), // ISO with >= without timezone
		hoursAgo.Format(time.RFC3339),                 // Full RFC3339
		">=" + hoursAgo.Format(time.RFC3339),          // Full RFC3339 with >=
	}

	log.Debug().
		Str("component", "redmine").
		Str("current_time", now.Format(time.RFC3339)).
		Str("search_from", hoursAgo.Format(time.RFC3339)).
		Msg("Time range for recent issue search")

	// Try each date format
	for _, dateFormat := range dateFormats {
		log.Debug().
			Str("component", "redmine").
			Str("date_format", dateFormat).
			Msg("Attempting date format for issue search")

		// Build URL, let http.NewRequest handle URL encoding
		baseUrl := common.Config.Redmine.Url + "/issues.json"
		req, err := http.NewRequest("GET", baseUrl, nil)
		if err != nil {
			log.Error().Err(err).Str("base_url", baseUrl).Msg("Failed to create HTTP request")
			continue
		}

		// Add query params
		q := req.URL.Query()
		q.Add("project_id", projectId)
		q.Add("subject", subject) // Try exact match first
		q.Add("created_on", dateFormat)
		q.Add("status_id", "*") // All statuses
		req.URL.RawQuery = q.Encode()

		log.Debug().
			Str("component", "redmine").
			Str("request_url", req.URL.String()).
			Msg("Sending request to find recent issues")

		// Set headers
		common.AddUserAgent(req)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Redmine-API-Key", common.Config.Redmine.Api_key)

		// Make request
		client := &http.Client{Timeout: time.Second * 10}
		resp, err := client.Do(req)
		if err != nil {
			log.Error().Err(err).Str("url", req.URL.String()).Msg("Failed to send HTTP request")
			continue
		}

		// Process response
		defer resp.Body.Close()
		log.Debug().
			Str("component", "redmine").
			Int("status_code", resp.StatusCode).
			Msg("Received response from Redmine API")

		// Read full response for debugging
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Error().Err(err).Msg("Failed to read response body")
			continue
		}

		log.Debug().
			Str("component", "redmine").
			Int("response_size", len(body)).
			Msg("Read response body from Redmine API")

		// Parse JSON
		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			log.Error().Err(err).Msg("Failed to parse JSON response")
			continue
		}

		// Check if we have results
		totalCount, ok := data["total_count"].(float64)
		if !ok || totalCount == 0 {
			log.Debug().
				Str("component", "redmine").
				Msg("No issues found with exact subject match, trying partial match")

			// Try with partial match if exact match failed
			q.Set("subject", "~"+subject)
			req.URL.RawQuery = q.Encode()

			log.Debug().
				Str("component", "redmine").
				Str("partial_match_url", req.URL.String()).
				Msg("Trying partial subject match")

			resp2, err := client.Do(req)
			if err != nil {
				log.Error().Err(err).Str("url", req.URL.String()).Msg("Failed to send partial match request")
				continue
			}

			defer resp2.Body.Close()
			body2, err := io.ReadAll(resp2.Body)
			if err != nil {
				log.Error().Err(err).Msg("Failed to read partial match response")
				continue
			}

			log.Debug().
				Str("component", "redmine").
				Int("response_size", len(body2)).
				Msg("Read partial match response body")

			if err := json.Unmarshal(body2, &data); err != nil {
				log.Error().Err(err).Msg("Failed to parse partial match JSON response")
				continue
			}

			totalCount, ok = data["total_count"].(float64)
			if !ok || totalCount == 0 {
				log.Debug().
					Str("component", "redmine").
					Msg("No issues found with partial subject match either")
				continue
			}
		}

		// We have results - find the most recent relevant issue
		log.Debug().
			Str("component", "redmine").
			Int("total_count", int(totalCount)).
			Msg("Found matching issues")

		issues, ok := data["issues"].([]interface{})
		if !ok || len(issues) == 0 {
			log.Debug().
				Str("component", "redmine").
				Msg("Issues array is empty or invalid")
			continue
		}

		// Check each issue
		for _, issue := range issues {
			issueMap, ok := issue.(map[string]interface{})
			if !ok {
				continue
			}

			issueId := int(issueMap["id"].(float64))
			status, ok := issueMap["status"].(map[string]interface{})
			if !ok {
				continue
			}

			statusId := int(status["id"].(float64))
			statusName := status["name"].(string)

			log.Debug().
				Str("component", "redmine").
				Int("issue_id", issueId).
				Str("status_name", statusName).
				Int("status_id", statusId).
				Msg("Found matching issue")

			// Return the first issue that matches (they should be sorted by creation date, newest first)
			return strconv.Itoa(issueId)
		}
	}

	log.Debug().
		Str("component", "redmine").
		Msg("No recent issues found after trying all date formats")
	return ""
}

func getCurrentUserId() (string, error) {
	if !common.Config.Redmine.Enabled {
		log.Debug().Str("component", "redmine").Msg("Redmine integration is disabled")
		return "", fmt.Errorf("redmine is disabled")
	}

	// Build URL for the current user
	redmineUrl := common.Config.Redmine.Url + "/users/current.json"

	// Create request
	req, err := http.NewRequest("GET", redmineUrl, nil)
	if err != nil {
		log.Error().Err(err).Str("url", redmineUrl).Msg("Failed to create HTTP request for current user")
		return "", err
	}

	common.AddUserAgent(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Redmine-API-Key", common.Config.Redmine.Api_key)

	// Execute request
	client := &http.Client{Timeout: time.Second * 10}
	resp, err := client.Do(req)
	if err != nil {
		log.Error().Err(err).Str("url", redmineUrl).Msg("Failed to send HTTP request for current user")
		return "", err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != 200 {
		errMsg := fmt.Sprintf("Redmine API returned status code %d instead of 200", resp.StatusCode)
		log.Error().
			Str("component", "redmine").
			Int("status_code", resp.StatusCode).
			Str("url", redmineUrl).
			Msg("Failed to get current user from Redmine API")
		return "", fmt.Errorf(errMsg)
	}

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read current user response body")
		return "", err
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		log.Error().Err(err).Msg("Failed to parse current user JSON response")
		return "", err
	}

	// Extract user ID
	user, ok := data["user"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("could not find user in response")
	}

	userId, ok := user["id"].(float64)
	if !ok {
		return "", fmt.Errorf("could not find user id in response")
	}

	return strconv.Itoa(int(userId)), nil
}

func Create(service string, subject string, message string) {
	log.Debug().
		Str("component", "redmine").
		Str("operation", "create_issue").
		Str("service", service).
		Str("subject", subject).
		Msg("Starting issue creation process")

	serviceReplaced := strings.Replace(service, "/", "-", -1)
	filePath := common.TmpDir + "/" + serviceReplaced + "-redmine.log"

	log.Debug().
		Str("component", "redmine").
		Str("operation", "create_issue").
		Str("file_path", filePath).
		Msg("Using service log file")

	if !common.Config.Redmine.Enabled {
		log.Debug().Str("component", "redmine").Msg("Redmine integration is disabled")
		return
	}

	if redmineCheckIssueLog(service) {
		log.Debug().
			Str("component", "redmine").
			Str("operation", "create_issue").
			Str("service", service).
			Msg("Issue log already exists, skipping creation")
		return
	}

	// Check if a similar issue exists in the last 6 hours
	existingIssueId := findRecentIssue(subject, 6)
	if existingIssueId != "" {
		log.Debug().
			Str("component", "redmine").
			Str("operation", "create_issue").
			Str("existing_issue_id", existingIssueId).
			Str("service", service).
			Msg("Found existing issue, attempting to reopen instead of creating new one")

		// Get the assigned user
		assignedToId := getAssignedToId(existingIssueId)

		// Get current user
		currentUserId, err := getCurrentUserId()
		if err != nil {
			log.Error().Err(err).Msg("Failed to get current user ID")
			return
		}

		log.Debug().
			Str("component", "redmine").
			Str("operation", "create_issue").
			Str("current_user_id", currentUserId).
			Str("assigned_to_id", assignedToId).
			Str("issue_id", existingIssueId).
			Msg("User assignment details for issue reopening")

		if assignedToId == currentUserId {
			assignedToId = ""
		}

		existingIssueIdAtoi, err := strconv.Atoi(existingIssueId)
		if err != nil {
			log.Error().Err(err).Str("component", "redmine").Str("operation", "create_issue").Str("input", existingIssueId).Msg("Failed to convert existing issue ID to integer")
			return
		}

		// Reopen the issue (status ID 8 = "In Progress")
		body := RedmineIssue{Issue: Issue{
			Id:           existingIssueIdAtoi,
			Notes:        "Sorun devam ettiğinden iş yeniden açıldı.\n" + message,
			StatusId:     8,
			AssignedToId: assignedToId,
		}}

		jsonBody, err := json.Marshal(body)
		if err != nil {
			log.Error().Err(err).Str("component", "redmine").Str("operation", "create_issue").Str("issue_id", existingIssueId).Msg("Failed to marshal issue reopen request")
			// Continue to creating new issue if reopening fails
		} else {
			// PUT request to update the issue
			updateUrl := common.Config.Redmine.Url + "/issues/" + existingIssueId + ".json"
			req, err := http.NewRequest("PUT", updateUrl, bytes.NewBuffer(jsonBody))
			if err != nil {
				log.Error().Err(err).Str("component", "redmine").Str("operation", "create_issue").Str("url", updateUrl).Str("issue_id", existingIssueId).Msg("Failed to create issue reopen request")
				// Continue to creating new issue if reopening fails
			} else {
				common.AddUserAgent(req)
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Redmine-API-Key", common.Config.Redmine.Api_key)

				log.Debug().
					Str("component", "redmine").
					Str("operation", "create_issue").
					Str("method", "PUT").
					Str("url", updateUrl).
					Str("issue_id", existingIssueId).
					Msg("Sending request to reopen existing issue")

				client := &http.Client{Timeout: time.Second * 10}
				resp, err := client.Do(req)

				if err != nil {
					log.Error().Err(err).Str("component", "redmine").Str("operation", "create_issue").Str("url", updateUrl).Str("issue_id", existingIssueId).Msg("Failed to send issue reopen request")
					// Continue to creating new issue
				} else {
					defer resp.Body.Close()

					// Check response
					if resp.StatusCode >= 200 && resp.StatusCode < 300 {
						log.Debug().
							Str("component", "redmine").
							Str("operation", "create_issue").
							Str("issue_id", existingIssueId).
							Str("service", service).
							Msg("Successfully reopened existing issue")

						// Write the issue ID to the service's log file
						err = os.WriteFile(filePath, []byte(existingIssueId), 0644)
						if err != nil {
							log.Error().Err(err).Str("component", "redmine").Str("operation", "create_issue").Str("file_path", filePath).Str("issue_id", existingIssueId).Msg("Failed to write reopened issue ID to log file")
						}
						return
					} else {
						respBody, _ := io.ReadAll(resp.Body)
						log.Warn().
							Str("component", "redmine").
							Str("operation", "create_issue").
							Int("status_code", resp.StatusCode).
							Int("response_size", len(respBody)).
							Str("issue_id", existingIssueId).
							Str("service", service).
							Msg("Failed to reopen existing issue, will create new one")
						// Continue to creating new issue
					}
				}
			}
		}

		log.Debug().
			Str("component", "redmine").
			Str("operation", "create_issue").
			Str("service", service).
			Msg("Failed to reopen existing issue, proceeding to create new one")
	}

	log.Debug().
		Str("component", "redmine").
		Str("operation", "create_issue").
		Str("subject", subject).
		Str("service", service).
		Msg("Creating new Redmine issue")

	var priorityId int
	var projectId string

	if common.Config.Redmine.Priority_id == 0 {
		priorityId = 5
	} else {
		priorityId = common.Config.Redmine.Priority_id
	}

	if common.Config.Redmine.Project_id == "" {
		projectId = strings.Split(common.Config.Identifier, "-")[0]
	} else {
		projectId = common.Config.Redmine.Project_id
	}

	body := RedmineIssue{Issue: Issue{ProjectId: projectId, TrackerId: 7, Description: message, Subject: subject, PriorityId: priorityId}}

	jsonBody, err := json.Marshal(body)

	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "create_issue").Str("service", service).Msg("Failed to marshal new issue request")
	}

	createUrl := common.Config.Redmine.Url + "/issues.json"
	req, err := http.NewRequest("POST", createUrl, bytes.NewBuffer(jsonBody))
	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "create_issue").Str("url", createUrl).Str("service", service).Msg("Failed to create new issue request")
	}
	common.AddUserAgent(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Redmine-API-Key", common.Config.Redmine.Api_key)

	client := &http.Client{
		Timeout: time.Second * 10,
	}

	resp, err := client.Do(req)

	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "create_issue").Str("url", createUrl).Str("service", service).Msg("Failed to send new issue creation request")
		return
	}

	defer resp.Body.Close()

	// read response
	var data RedmineIssue

	err = json.NewDecoder(resp.Body).Decode(&data)

	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "create_issue").Str("service", service).Msg("Failed to decode new issue response")
	}

	// get issue id, convert to string
	issueId := []byte(strconv.Itoa(data.Issue.Id))

	// write issue id to file
	err = os.WriteFile(filePath, issueId, 0644)

	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "create_issue").Str("file_path", filePath).Int("issue_id", data.Issue.Id).Str("service", service).Msg("Failed to write new issue ID to log file")
	}

	log.Info().
		Str("component", "redmine").
		Str("operation", "create_issue").
		Int("issue_id", data.Issue.Id).
		Str("service", service).
		Str("subject", subject).
		Msg("Successfully created new Redmine issue")
}

func ExistsNote(service string, message string) bool {
	// Check if a note in an issue already exists
	serviceReplaced := strings.Replace(service, "/", "-", -1)
	filePath := common.TmpDir + "/" + serviceReplaced + "-redmine.log"

	// check if filePath exists, if not return
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return false
	}

	// Check if file is empty, if so delete the file and return
	if common.IsEmptyOrWhitespace(filePath) {
		err := os.Remove(filePath)
		if err != nil {
			log.Error().Err(err).Str("component", "redmine").Str("operation", "exists_note").Str("file_path", filePath).Str("service", service).Msg("Failed to remove empty log file")
		}
		return false
	}

	// read file
	file, err := os.ReadFile(filePath)

	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "exists_note").Str("file_path", filePath).Str("service", service).Msg("Failed to read issue log file")
		return false
	}

	if string(file) == "0" {
		err := os.Remove(filePath)
		if err != nil {
			log.Error().Err(err).Str("component", "redmine").Str("operation", "exists_note").Str("file_path", filePath).Str("service", service).Msg("Failed to remove zero-content log file")
		}
	}

	redmineUrlFinal := common.Config.Redmine.Url + "/issues/" + string(file) + ".json?include=journals"

	// Send a GET request to the Redmine API to get all issues
	req, err := http.NewRequest("GET", redmineUrlFinal, nil)
	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "exists_note").Str("url", redmineUrlFinal).Str("service", service).Msg("Failed to create request for issue journals")
		return false
	}
	common.AddUserAgent(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Redmine-API-Key", common.Config.Redmine.Api_key)

	client := &http.Client{
		Timeout: time.Second * 10,
	}

	resp, err := client.Do(req)

	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "exists_note").Str("url", redmineUrlFinal).Str("service", service).Msg("Failed to send request for issue journals")
		return false
	}

	defer resp.Body.Close()

	// read response and get notes
	var data map[string]interface{}

	err = json.NewDecoder(resp.Body).Decode(&data)

	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "exists_note").Str("service", service).Msg("Failed to decode issue journals response")
		return false
	}

	// If not 200, log error
	if resp.StatusCode != 200 {
		log.Error().
			Str("component", "redmine").
			Str("operation", "exists_note").
			Int("status_code", resp.StatusCode).
			Str("url", redmineUrlFinal).
			Str("service", service).
			Msg("Redmine API returned non-200 status for issue journals")
		return false
	}

	// Check if the note already exists
	for _, journal := range data["issue"].(map[string]interface{})["journals"].([]interface{}) {
		if journal.(map[string]interface{})["notes"].(string) == message {
			return true
		}
	}

	return false
}

func Delete(id int) {

	if !common.Config.Redmine.Enabled {
		return
	}

	deleteUrl := common.Config.Redmine.Url + "/issues/" + strconv.Itoa(id) + ".json"
	req, err := http.NewRequest("DELETE", deleteUrl, nil)
	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "delete_issue").Str("url", deleteUrl).Int("issue_id", id).Msg("Failed to create issue deletion request")
	}
	common.AddUserAgent(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Redmine-API-Key", common.Config.Redmine.Api_key)

	client := &http.Client{
		Timeout: time.Second * 10,
	}

	resp, err := client.Do(req)

	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "delete_issue").Str("url", deleteUrl).Int("issue_id", id).Msg("Failed to send issue deletion request")
		return
	}

	defer resp.Body.Close()
}

func Update(service string, message string, checkNote bool) {

	if !common.Config.Redmine.Enabled {
		return
	}

	if checkNote {
		if ExistsNote(service, message) {
			return
		}
	}

	serviceReplaced := strings.Replace(service, "/", "-", -1)
	filePath := common.TmpDir + "/" + serviceReplaced + "-redmine.log"

	if !redmineCheckIssueLog(service) {
		return
	}

	// read file
	file, err := os.ReadFile(filePath)

	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "update_issue").Str("file_path", filePath).Str("service", service).Msg("Failed to read issue log file for update")
	}

	// get issue id
	issueId, err := strconv.Atoi(string(file))

	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "update_issue").Str("content", string(file)).Str("service", service).Msg("Failed to convert issue ID to integer")
	}

	if issueId == 0 {
		// Remove file
		err := os.Remove(filePath)
		if err != nil {
			log.Error().Err(err).Str("component", "redmine").Str("operation", "update_issue").Str("file_path", filePath).Str("service", service).Msg("Failed to remove invalid issue log file")
		}
		return
	}

	// update issue
	body := RedmineIssue{Issue: Issue{Id: issueId, Notes: message}}

	jsonBody, err := json.Marshal(body)

	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "update_issue").Str("service", service).Int("issue_id", issueId).Msg("Failed to marshal issue update request")
	}

	updateUrl := common.Config.Redmine.Url + "/issues/" + string(file) + ".json"
	req, err := http.NewRequest("PUT", updateUrl, bytes.NewBuffer(jsonBody))
	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "update_issue").Str("url", updateUrl).Str("service", service).Int("issue_id", issueId).Msg("Failed to create issue update request")
	}
	common.AddUserAgent(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Redmine-API-Key", common.Config.Redmine.Api_key)

	client := &http.Client{
		Timeout: time.Second * 10,
	}

	resp, err := client.Do(req)

	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "update_issue").Str("url", updateUrl).Str("service", service).Int("issue_id", issueId).Msg("Failed to send issue update request")
		return
	}

	defer resp.Body.Close()
}

func getAssignedToId(id string) string {

	// Make request to Redmine API to get the assigned_to_id
	redmineUrlFinal := common.Config.Redmine.Url + "/issues/" + id + ".json"

	req, err := http.NewRequest("GET", redmineUrlFinal, nil)
	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "get_assigned_to_id").Str("url", redmineUrlFinal).Str("issue_id", id).Msg("Failed to create request for issue assignment info")
	}
	common.AddUserAgent(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Redmine-API-Key", common.Config.Redmine.Api_key)

	client := &http.Client{
		Timeout: time.Second * 10,
	}

	resp, err := client.Do(req)

	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "get_assigned_to_id").Str("url", redmineUrlFinal).Str("issue_id", id).Msg("Failed to send request for issue assignment info")
		return ""
	}

	defer resp.Body.Close()

	// read response and get assigned_to_id
	var data map[string]interface{}

	err = json.NewDecoder(resp.Body).Decode(&data)

	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "get_assigned_to_id").Str("issue_id", id).Msg("Failed to decode issue assignment response")
	}

	// If not 200, log error

	if resp.StatusCode != 200 {
		log.Error().
			Str("component", "redmine").
			Str("operation", "get_assigned_to_id").
			Int("status_code", resp.StatusCode).
			Str("url", redmineUrlFinal).
			Str("issue_id", id).
			Msg("Redmine API returned non-200 status for issue assignment info")
		return ""
	}

	// Check if id exists

	if data["issue"].(map[string]interface{})["assigned_to"] == nil {
		return ""
	}

	return strconv.Itoa(int(data["issue"].(map[string]interface{})["assigned_to"].(map[string]interface{})["id"].(float64)))
}

func Close(service string, message string) {
	if !common.Config.Redmine.Enabled {
		return
	}

	serviceReplaced := strings.Replace(service, "/", "-", -1)
	filePath := common.TmpDir + "/" + serviceReplaced + "-redmine.log"

	// check if filePath exists, if not return
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return
	}

	if !redmineCheckIssueLog(service) {
		return
	}

	// read file
	file, err := os.ReadFile(filePath)
	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "close_issue").Str("file_path", filePath).Str("service", service).Msg("Failed to read issue log file for closing")
	}

	issueId, err := strconv.Atoi(string(file))

	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "close_issue").Str("content", string(file)).Str("service", service).Msg("Failed to convert issue ID to integer for closing")
	}

	if issueId == 0 {
		// Remove file
		err := os.Remove(filePath)
		if err != nil {
			log.Error().Err(err).Str("component", "redmine").Str("operation", "close_issue").Str("file_path", filePath).Str("service", service).Msg("Failed to remove invalid issue log file")
		}
		return
	}

	assignedToId := getAssignedToId(string(file))

	if assignedToId == "" {
		assignedToId = "me"
	}

	// update issue
	body := RedmineIssue{Issue: Issue{Id: issueId, Notes: message, StatusId: 5, AssignedToId: assignedToId}}
	jsonBody, err := json.Marshal(body)

	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "close_issue").Str("service", service).Int("issue_id", issueId).Msg("Failed to marshal issue close request")
	}

	closeUrl := common.Config.Redmine.Url + "/issues/" + string(file) + ".json"
	req, err := http.NewRequest("PUT", closeUrl, bytes.NewBuffer(jsonBody))
	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "close_issue").Str("url", closeUrl).Str("service", service).Int("issue_id", issueId).Msg("Failed to create issue close request")
	}
	common.AddUserAgent(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Redmine-API-Key", common.Config.Redmine.Api_key)

	client := &http.Client{
		Timeout: time.Second * 10,
	}

	resp, err := client.Do(req)

	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "close_issue").Str("url", closeUrl).Str("service", service).Int("issue_id", issueId).Msg("Failed to send issue close request")
		return
	}

	defer resp.Body.Close()

	// remove file
	err = os.Remove(filePath)

	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "close_issue").Str("file_path", filePath).Str("service", service).Int("issue_id", issueId).Msg("Failed to remove issue log file after closing")
	}

	log.Info().
		Str("component", "redmine").
		Str("operation", "close_issue").
		Int("issue_id", issueId).
		Str("service", service).
		Msg("Successfully closed Redmine issue")
}

func Show(service string) string {
	if !common.Config.Redmine.Enabled {
		return ""
	}

	serviceReplaced := strings.Replace(service, "/", "-", -1)
	filePath := common.TmpDir + "/" + serviceReplaced + "-redmine.log"

	if !redmineCheckIssueLog(service) {
		return ""
	}

	// read file
	file, err := os.ReadFile(filePath)
	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "show_issue").Str("file_path", filePath).Str("service", service).Msg("Failed to read issue log file for show")
	}

	// get issue ID
	return string(file)
}

func Exists(subject string, date string, search bool) string {
	var projectId string

	if common.Config.Redmine.Project_id == "" {
		projectId = strings.Split(common.Config.Identifier, "-")[0]
	} else {
		projectId = common.Config.Redmine.Project_id
	}

	if !common.Config.Redmine.Enabled {
		return ""
	}

	subject = strings.Replace(subject, " ", "%20", -1)

	redmineUrlFinal := common.Config.Redmine.Url + "/issues.json?project_id=" + projectId

	if search {
		redmineUrlFinal += "&subject=~" + subject
	} else {
		redmineUrlFinal += "&subject=" + subject
	}

	if date != "" {
		redmineUrlFinal += "&created_on=" + date
	}

	// Send a GET request to the Redmine API to get all issues
	req, err := http.NewRequest("GET", redmineUrlFinal, nil)
	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "exists_issue").Str("url", redmineUrlFinal).Str("subject", subject).Msg("Failed to create issue search request")
	}
	common.AddUserAgent(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Redmine-API-Key", common.Config.Redmine.Api_key)

	client := &http.Client{
		Timeout: time.Second * 10,
	}

	resp, err := client.Do(req)

	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "exists_issue").Str("url", redmineUrlFinal).Str("subject", subject).Msg("Failed to send issue search request")
		return ""
	}

	defer resp.Body.Close()

	// read response and get issue ID
	var data map[string]interface{}

	err = json.NewDecoder(resp.Body).Decode(&data)

	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "exists_issue").Str("subject", subject).Msg("Failed to decode issue search response")
	}

	// If not 200, log error
	if resp.StatusCode != 200 {
		log.Error().
			Str("component", "redmine").
			Str("operation", "exists_issue").
			Int("status_code", resp.StatusCode).
			Str("url", redmineUrlFinal).
			Str("subject", subject).
			Msg("Redmine API returned non-200 status for issue search")
		return ""
	}

	if data["total_count"] == nil || data["total_count"].(float64) == 0 {
		return ""
	} else {
		if data["issues"].([]interface{})[0].(map[string]interface{})["status"].(map[string]interface{})["id"].(float64) == 5 {
			return ""
		} else {
			return strconv.Itoa(int(data["issues"].([]interface{})[0].(map[string]interface{})["id"].(float64)))
		}
	}
}
