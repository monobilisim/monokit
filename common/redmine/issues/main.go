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
)

// Helper function to convert string to int
func atoi(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		common.LogError("strconv.Atoi error: " + err.Error())
		return 0
	}
	return i
}

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
				common.LogError("os.Remove error: " + err.Error())
			}
			return false
		}

		// Check if file is 0, if so delete the file and return
		read, err := os.ReadFile(filePath)

		if err != nil {
			common.LogError("os.ReadFile error: " + err.Error())
		}

		if string(read) == "0" {
			err := os.Remove(filePath)
			if err != nil {
				common.LogError("os.Remove error: " + err.Error())
			}
			return false
		}

		return true
	}

	return false
}

func redmineWrapper(service string, subject string, message string) {

	if redmineCheckIssueLog(service) == false {
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
			common.LogError("Error opening file for writing: \n" + err.Error())
		}

		var j common.ServiceFile

		fileRead, err := io.ReadAll(file)

		if err != nil {
			common.LogError("Error reading file: \n" + err.Error())
			return
		}

		err = json.Unmarshal(fileRead, &j)

		if err != nil {
			common.LogError("Error parsing JSON: \n" + err.Error())
			return
		}

		// Return if locked == true
		if j.Locked == true {
			return
		}

		oldDate := j.Date
		oldDateParsed, err := time.Parse("2006-01-02 15:04:05 -0700", oldDate)

		if err != nil {
			common.LogError("Error parsing date: \n" + err.Error())
		}

		finJson := &common.ServiceFile{
			Date:   currentDate,
			Locked: true,
		}

		if interval == 0 {
			if oldDateParsed.Format("2006-01-02") != time.Now().Format("2006-01-02") {
				jsonData, err := json.Marshal(&common.ServiceFile{Date: currentDate, Locked: false})

				if err != nil {
					common.LogError("Error marshalling JSON: \n" + err.Error())
				}

				err = os.WriteFile(filePath, jsonData, 0644)

				redmineWrapper(service, subject, message)
			}
			return
		}

		if time.Now().Sub(oldDateParsed).Hours() > 24 {
			jsonData, err := json.Marshal(finJson)

			if err != nil {
				common.LogError("Error marshalling JSON: \n" + err.Error())
			}

			err = os.WriteFile(filePath, jsonData, 0644)

			if err != nil {
				common.LogError("Error writing to file: \n" + err.Error())
			}

			redmineWrapper(service, subject, message)
		} else {
			if j.Locked == false {
				// currentDate - oldDate in minutes
				timeDiff := time.Now().Sub(oldDateParsed) //.Minutes()

				if timeDiff.Minutes() >= interval {
					jsonData, err := json.Marshal(finJson)
					if err != nil {
						common.LogError("Error marshalling JSON: \n" + err.Error())
					}

					err = os.WriteFile(filePath, jsonData, 0644)

					if err != nil {
						common.LogError("Error writing to file: \n" + err.Error())
					}

					redmineWrapper(service, subject, message)
				}
			}
		}
	} else {

		file, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0644)
		defer file.Close()

		if err != nil {
			common.LogError("Error opening file for writing: \n" + err.Error())
			return
		}

		jsonData, err := json.Marshal(&common.ServiceFile{Date: currentDate, Locked: false})

		if err != nil {
			common.LogError("Error marshalling JSON: \n" + err.Error())
		}

		err = os.WriteFile(filePath, jsonData, 0644)

		if err != nil {
			common.LogError("Error writing to file: \n" + err.Error())
		}

		if interval == 0 {
			redmineWrapper(service, subject, message)
		}
	}
}

// Function to check for recent issues
func findRecentIssue(subject string, hoursBack int) string {
	common.LogDebug("findRecentIssue - Looking for recent issues with subject: " + subject + " in last " + strconv.Itoa(hoursBack) + " hours")

	var projectId string
	if common.Config.Redmine.Project_id == "" {
		projectId = strings.Split(common.Config.Identifier, "-")[0]
	} else {
		projectId = common.Config.Redmine.Project_id
	}

	if common.Config.Redmine.Enabled == false {
		common.LogDebug("findRecentIssue - Redmine is disabled")
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

	common.LogDebug("findRecentIssue - Current time: " + now.Format(time.RFC3339))
	common.LogDebug("findRecentIssue - Looking back to: " + hoursAgo.Format(time.RFC3339))

	// Try each date format
	for _, dateFormat := range dateFormats {
		common.LogDebug("findRecentIssue - Trying date format: " + dateFormat)

		// Build URL, let http.NewRequest handle URL encoding
		baseUrl := common.Config.Redmine.Url + "/issues.json"
		req, err := http.NewRequest("GET", baseUrl, nil)
		if err != nil {
			common.LogError("http.NewRequest error: " + err.Error())
			continue
		}

		// Add query params
		q := req.URL.Query()
		q.Add("project_id", projectId)
		q.Add("subject", subject) // Try exact match first
		q.Add("created_on", dateFormat)
		q.Add("status_id", "*") // All statuses
		req.URL.RawQuery = q.Encode()

		common.LogDebug("findRecentIssue - Request URL: " + req.URL.String())

		// Set headers
		common.AddUserAgent(req)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Redmine-API-Key", common.Config.Redmine.Api_key)

		// Make request
		client := &http.Client{Timeout: time.Second * 10}
		resp, err := client.Do(req)
		if err != nil {
			common.LogError("client.Do error: " + err.Error())
			continue
		}

		// Process response
		defer resp.Body.Close()
		common.LogDebug("findRecentIssue - Response status: " + resp.Status)

		// Read full response for debugging
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			common.LogError("Error reading response: " + err.Error())
			continue
		}

		common.LogDebug("findRecentIssue - Response body: " + string(body))

		// Parse JSON
		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			common.LogError("JSON unmarshal error: " + err.Error())
			continue
		}

		// Check if we have results
		totalCount, ok := data["total_count"].(float64)
		if !ok || totalCount == 0 {
			common.LogDebug("findRecentIssue - No issues found with exact subject match")

			// Try with partial match if exact match failed
			q.Set("subject", "~"+subject)
			req.URL.RawQuery = q.Encode()

			common.LogDebug("findRecentIssue - Trying with partial match: " + req.URL.String())

			resp2, err := client.Do(req)
			if err != nil {
				common.LogError("client.Do error (partial match): " + err.Error())
				continue
			}

			defer resp2.Body.Close()
			body2, err := io.ReadAll(resp2.Body)
			if err != nil {
				common.LogError("Error reading response (partial match): " + err.Error())
				continue
			}

			common.LogDebug("findRecentIssue - Partial match response: " + string(body2))

			if err := json.Unmarshal(body2, &data); err != nil {
				common.LogError("JSON unmarshal error (partial match): " + err.Error())
				continue
			}

			totalCount, ok = data["total_count"].(float64)
			if !ok || totalCount == 0 {
				common.LogDebug("findRecentIssue - No issues found with partial subject match either")
				continue
			}
		}

		// We have results - find the most recent relevant issue
		common.LogDebug("findRecentIssue - Found " + strconv.Itoa(int(totalCount)) + " issues")

		issues, ok := data["issues"].([]interface{})
		if !ok || len(issues) == 0 {
			common.LogDebug("findRecentIssue - Issues array is empty or invalid")
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

			common.LogDebug(fmt.Sprintf("findRecentIssue - Found issue #%d with status %s (ID: %d)",
				issueId, statusName, statusId))

			// Return the first issue that matches (they should be sorted by creation date, newest first)
			return strconv.Itoa(issueId)
		}
	}

	common.LogDebug("findRecentIssue - No recent issues found after trying all date formats")
	return ""
}

func getCurrentUserId() (string, error) {
	if !common.Config.Redmine.Enabled {
		common.LogDebug("getCurrentUserId - Redmine is disabled")
		return "", fmt.Errorf("redmine is disabled")
	}

	// Build URL for the current user
	redmineUrl := common.Config.Redmine.Url + "/users/current.json"

	// Create request
	req, err := http.NewRequest("GET", redmineUrl, nil)
	if err != nil {
		common.LogError("getCurrentUserId - http.NewRequest error: " + err.Error())
		return "", err
	}

	common.AddUserAgent(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Redmine-API-Key", common.Config.Redmine.Api_key)

	// Execute request
	client := &http.Client{Timeout: time.Second * 10}
	resp, err := client.Do(req)
	if err != nil {
		common.LogError("getCurrentUserId - client.Do error: " + err.Error())
		return "", err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != 200 {
		errMsg := fmt.Sprintf("getCurrentUserId - Redmine API returned status code %d instead of 200", resp.StatusCode)
		common.LogError(errMsg)
		return "", fmt.Errorf(errMsg)
	}

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		common.LogError("getCurrentUserId - error reading response body: " + err.Error())
		return "", err
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		common.LogError("getCurrentUserId - json.Unmarshal error: " + err.Error())
		return "", err
	}

	// Extract user ID
	user, ok := data["user"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("getCurrentUserId - couldn't find user in response")
	}

	userId, ok := user["id"].(float64)
	if !ok {
		return "", fmt.Errorf("getCurrentUserId - couldn't find user id in response")
	}

	return strconv.Itoa(int(userId)), nil
}

func Create(service string, subject string, message string) {
	common.LogDebug("Create - Creating/reopening issue for service: " + service + ", subject: " + subject)

	serviceReplaced := strings.Replace(service, "/", "-", -1)
	filePath := common.TmpDir + "/" + serviceReplaced + "-redmine.log"
	common.LogDebug("Create - Using log file: " + filePath)

	if common.Config.Redmine.Enabled == false {
		common.LogDebug("Create - Redmine is disabled, returning")
		return
	}

	if redmineCheckIssueLog(service) == true {
		common.LogDebug("Create - Issue log already exists, returning")
		return
	}

	// Check if a similar issue exists in the last 6 hours
	existingIssueId := findRecentIssue(subject, 6)
	if existingIssueId != "" {
		common.LogDebug("Create - Found existing issue #" + existingIssueId + ", reopening instead of creating a new one")

		// Get the assigned user
		assignedToId := getAssignedToId(existingIssueId)

		// Get current user
		currentUserId, err := getCurrentUserId()
		if err != nil {
			common.LogError("getCurrentUserId error: " + err.Error())
			return
		}
		common.LogDebug("Create - Current user ID: " + currentUserId)
		common.LogDebug("Create - Assigned user ID: " + assignedToId)

		if assignedToId == currentUserId {
			assignedToId = ""
		}

		// Reopen the issue (status ID 2 = "In Progress")
		body := RedmineIssue{Issue: Issue{
			Id:           atoi(existingIssueId),
			Notes:        "Sorun devam ettiğinden iş yeniden açıldı.\n" + message,
			StatusId:     8,
			AssignedToId: assignedToId,
		}}

		jsonBody, err := json.Marshal(body)
		if err != nil {
			common.LogError("json.Marshal error: " + err.Error())
			// Continue to creating new issue if reopening fails
		} else {
			// PUT request to update the issue
			req, err := http.NewRequest("PUT", common.Config.Redmine.Url+"/issues/"+existingIssueId+".json", bytes.NewBuffer(jsonBody))
			if err != nil {
				common.LogError("http.NewRequest error: " + err.Error())
				// Continue to creating new issue if reopening fails
			} else {
				common.AddUserAgent(req)
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Redmine-API-Key", common.Config.Redmine.Api_key)

				common.LogDebug("Create - Sending PUT request to reopen issue: " + common.Config.Redmine.Url + "/issues/" + existingIssueId + ".json")
				common.LogDebug("Create - Request body: " + string(jsonBody))

				client := &http.Client{Timeout: time.Second * 10}
				resp, err := client.Do(req)

				if err != nil {
					common.LogError("client.Do error: " + err.Error())
					// Continue to creating new issue
				} else {
					defer resp.Body.Close()

					// Check response
					if resp.StatusCode >= 200 && resp.StatusCode < 300 {
						common.LogDebug("Create - Successfully reopened issue #" + existingIssueId)

						// Write the issue ID to the service's log file
						err = os.WriteFile(filePath, []byte(existingIssueId), 0644)
						if err != nil {
							common.LogError("os.WriteFile error: " + err.Error())
						}
						return
					} else {
						respBody, _ := io.ReadAll(resp.Body)
						common.LogError("Failed to reopen issue, status: " + resp.Status + ", response: " + string(respBody))
						// Continue to creating new issue
					}
				}
			}
		}

		common.LogDebug("Create - Failed to reopen existing issue, will create a new one")
	}

	common.LogDebug("Create - Creating a new issue")
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
		common.LogError("json.Marshal error: " + err.Error())
	}

	req, err := http.NewRequest("POST", common.Config.Redmine.Url+"/issues.json", bytes.NewBuffer(jsonBody))
	if err != nil {
		common.LogError("http.NewRequest error: " + err.Error())
	}
	common.AddUserAgent(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Redmine-API-Key", common.Config.Redmine.Api_key)

	client := &http.Client{
		Timeout: time.Second * 10,
	}

	resp, err := client.Do(req)

	if err != nil {
		common.LogError("client.Do error: " + err.Error() + "\n" + "Redmine URL: " + common.Config.Redmine.Url + "/issues.json" + "\n" + "Redmine JSON: " + string(jsonBody))
		return
	}

	defer resp.Body.Close()

	// read response
	var data RedmineIssue

	err = json.NewDecoder(resp.Body).Decode(&data)

	if err != nil {
		common.LogError("json.NewDecoder error: " + err.Error())
	}

	// get issue id, convert to string
	issueId := []byte(strconv.Itoa(data.Issue.Id))

	// write issue id to file
	err = os.WriteFile(filePath, issueId, 0644)

	if err != nil {
		common.LogError("os.WriteFile error while trying to read '" + filePath + "'" + err.Error())
	}
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
			common.LogError("os.Remove error: " + err.Error())
		}
		return false
	}

	// read file
	file, err := os.ReadFile(filePath)

	if err != nil {
		common.LogError("os.ReadFile error: " + err.Error())
		return false
	}

	if string(file) == "0" {
		err := os.Remove(filePath)
		if err != nil {
			common.LogError("os.Remove error: " + err.Error())
		}
	}

	redmineUrlFinal := common.Config.Redmine.Url + "/issues/" + string(file) + ".json?include=journals"

	// Send a GET request to the Redmine API to get all issues
	req, err := http.NewRequest("GET", redmineUrlFinal, nil)
	if err != nil {
		common.LogError("http.NewRequest error: " + err.Error())
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
		common.LogError("client.Do error: " + err.Error() + "\n" + "Redmine URL: " + redmineUrlFinal)
		return false
	}

	defer resp.Body.Close()

	// read response and get notes
	var data map[string]interface{}

	err = json.NewDecoder(resp.Body).Decode(&data)

	if err != nil {
		common.LogError("json.NewDecoder error: " + err.Error())
		return false
	}

	// If not 200, log error
	if resp.StatusCode != 200 {
		// Unmarshal the response body
		common.LogError("Redmine API returned status code " + strconv.Itoa(resp.StatusCode) + " instead of 200\n" + "Redmine URL: " + redmineUrlFinal)
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

	if common.Config.Redmine.Enabled == false {
		return
	}

	req, err := http.NewRequest("DELETE", common.Config.Redmine.Url+"/issues/"+strconv.Itoa(id)+".json", nil)
	if err != nil {
		common.LogError("http.NewRequest error: " + err.Error())
	}
	common.AddUserAgent(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Redmine-API-Key", common.Config.Redmine.Api_key)

	client := &http.Client{
		Timeout: time.Second * 10,
	}

	resp, err := client.Do(req)

	if err != nil {
		common.LogError("client.Do error: " + err.Error() + "\n" + "Redmine URL: " + common.Config.Redmine.Url + "/issues/" + strconv.Itoa(id) + ".json")
		return
	}

	defer resp.Body.Close()
}

func Update(service string, message string, checkNote bool) {

	if common.Config.Redmine.Enabled == false {
		return
	}

	if checkNote {
		if ExistsNote(service, message) {
			return
		}
	}

	serviceReplaced := strings.Replace(service, "/", "-", -1)
	filePath := common.TmpDir + "/" + serviceReplaced + "-redmine.log"

	if redmineCheckIssueLog(service) == false {
		return
	}

	// read file
	file, err := os.ReadFile(filePath)

	if err != nil {
		common.LogError("os.ReadFile error: " + err.Error())
	}

	// get issue id
	issueId, err := strconv.Atoi(string(file))

	if err != nil {
		common.LogError("strconv.Atoi error: " + err.Error())
	}

	if issueId == 0 {
		// Remove file
		err := os.Remove(filePath)
		if err != nil {
			common.LogError("os.Remove error: " + err.Error())
		}
		return
	}

	// update issue
	body := RedmineIssue{Issue: Issue{Id: issueId, Notes: message}}

	jsonBody, err := json.Marshal(body)

	if err != nil {
		common.LogError("json.Marshal error: " + err.Error())
	}

	req, err := http.NewRequest("PUT", common.Config.Redmine.Url+"/issues/"+string(file)+".json", bytes.NewBuffer(jsonBody))
	if err != nil {
		common.LogError("http.NewRequest error: " + err.Error())
	}
	common.AddUserAgent(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Redmine-API-Key", common.Config.Redmine.Api_key)

	client := &http.Client{
		Timeout: time.Second * 10,
	}

	resp, err := client.Do(req)

	if err != nil {
		common.LogError("client.Do error: " + err.Error() + "\n" + "Redmine URL: " + common.Config.Redmine.Url + "/issues/" + string(file) + ".json" + "\n" + "Redmine JSON: " + string(jsonBody))
		return
	}

	defer resp.Body.Close()
}

func getAssignedToId(id string) string {

	// Make request to Redmine API to get the assigned_to_id
	redmineUrlFinal := common.Config.Redmine.Url + "/issues/" + id + ".json"

	req, err := http.NewRequest("GET", redmineUrlFinal, nil)
	if err != nil {
		common.LogError("http.NewRequest error: " + err.Error())
	}
	common.AddUserAgent(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Redmine-API-Key", common.Config.Redmine.Api_key)

	client := &http.Client{
		Timeout: time.Second * 10,
	}

	resp, err := client.Do(req)

	if err != nil {
		common.LogError("client.Do error: " + err.Error() + "\n" + "Redmine URL: " + redmineUrlFinal)
		return ""
	}

	defer resp.Body.Close()

	// read response and get assigned_to_id
	var data map[string]interface{}

	err = json.NewDecoder(resp.Body).Decode(&data)

	if err != nil {
		common.LogError("json.NewDecoder error: " + err.Error())
	}

	// If not 200, log error

	if resp.StatusCode != 200 {
		// Unmarshal the response body
		common.LogError("Redmine API returned status code " + strconv.Itoa(resp.StatusCode) + " instead of 200\n" + "Redmine URL: " + redmineUrlFinal)
		return ""
	}

	// Check if id exists

	if data["issue"].(map[string]interface{})["assigned_to"] == nil {
		return ""
	}

	return strconv.Itoa(int(data["issue"].(map[string]interface{})["assigned_to"].(map[string]interface{})["id"].(float64)))
}

func Close(service string, message string) {
	if common.Config.Redmine.Enabled == false {
		return
	}

	serviceReplaced := strings.Replace(service, "/", "-", -1)
	filePath := common.TmpDir + "/" + serviceReplaced + "-redmine.log"

	// check if filePath exists, if not return
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return
	}

	if redmineCheckIssueLog(service) == false {
		return
	}

	// read file
	file, err := os.ReadFile(filePath)
	if err != nil {
		common.LogError("os.ReadFile error while trying to read '" + filePath + "'" + err.Error())
	}

	issueId, err := strconv.Atoi(string(file))

	if err != nil {
		common.LogError("strconv.Atoi error: " + err.Error())
	}

	if issueId == 0 {
		// Remove file
		err := os.Remove(filePath)
		if err != nil {
			common.LogError("os.Remove error: " + err.Error())
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
		common.LogError("json.Marshal error: " + err.Error())
	}

	req, err := http.NewRequest("PUT", common.Config.Redmine.Url+"/issues/"+string(file)+".json", bytes.NewBuffer(jsonBody))
	if err != nil {
		common.LogError("http.NewRequest error: " + err.Error())
	}
	common.AddUserAgent(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Redmine-API-Key", common.Config.Redmine.Api_key)

	client := &http.Client{
		Timeout: time.Second * 10,
	}

	resp, err := client.Do(req)

	if err != nil {
		common.LogError("client.Do error: " + err.Error() + "\n" + "Redmine URL: " + common.Config.Redmine.Url + "/issues/" + string(file) + ".json" + "\n" + "Redmine JSON: " + string(jsonBody))
		return
	}

	defer resp.Body.Close()

	// remove file
	err = os.Remove(filePath)

	if err != nil {
		common.LogError("os.Remove error: " + err.Error())
	}
}

func Show(service string) string {
	if common.Config.Redmine.Enabled == false {
		return ""
	}

	serviceReplaced := strings.Replace(service, "/", "-", -1)
	filePath := common.TmpDir + "/" + serviceReplaced + "-redmine.log"

	if redmineCheckIssueLog(service) == false {
		return ""
	}

	// read file
	file, err := os.ReadFile(filePath)
	if err != nil {
		common.LogError("os.ReadFile error: " + err.Error())
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

	if common.Config.Redmine.Enabled == false {
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
		common.LogError("http.NewRequest error: " + err.Error())
	}
	common.AddUserAgent(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Redmine-API-Key", common.Config.Redmine.Api_key)

	client := &http.Client{
		Timeout: time.Second * 10,
	}

	resp, err := client.Do(req)

	if err != nil {
		common.LogError("client.Do error: " + err.Error() + "\n" + "Redmine URL: " + redmineUrlFinal)
		return ""
	}

	defer resp.Body.Close()

	// read response and get issue ID
	var data map[string]interface{}

	err = json.NewDecoder(resp.Body).Decode(&data)

	if err != nil {
		common.LogError("json.NewDecoder error: " + err.Error())
	}

	// If not 200, log error
	if resp.StatusCode != 200 {
		// Unmarshal the response body
		common.LogError("Redmine API returned status code " + strconv.Itoa(resp.StatusCode) + " instead of 200\n" + "Redmine URL: " + redmineUrlFinal)
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
