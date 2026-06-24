package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/monobilisim/monokit/common/healthdb"
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

// RedmineBotUserID is no longer a build-time constant. The bot user ID is
// resolved dynamically from the API key owner via /users/current.json (see
// getBotUserID). If the lookup fails, the close path logs the error and skips
// the reassignment — preserving the existing (human) assignee rather than
// guessing.

var (
	botUserOnce sync.Once
	botUserID   string
)

// getBotUserID resolves the Redmine user ID of the API key owner ("Redmine
// Bot (Mono)" in the standard monokit deployment). On the first call it
// queries /users/current.json and caches the result for the lifetime of the
// process. If the lookup fails, it returns an empty string and logs an error;
// the caller is expected to skip the assignee override in that case.
func getBotUserID() string {
	botUserOnce.Do(func() {
		uid, err := getCurrentUserId()
		if err == nil && uid != "" {
			botUserID = uid
			log.Info().
				Str("component", "redmine").
				Str("operation", "get_bot_user_id").
				Str("bot_user_id", uid).
				Msg("Bot user ID resolved and cached from /users/current.json")
			return
		}
		// Lookup failed — leave botUserID empty. The Close path will skip
		// the AssignedToId override, preserving the existing assignee.
		log.Error().
			Str("component", "redmine").
			Str("operation", "get_bot_user_id").
			Err(err).
			Msg("Could not resolve bot user ID from /users/current.json; assignee will not be overridden (existing assignee preserved)")
	})
	return botUserID
}

func redmineIssueKey(service string) string {
	return strings.Replace(service, "/", "-", -1) + ":redmine:issue"
}
func redmineStatKey(service string) string {
	return strings.Replace(service, "/", "-", -1) + ":redmine:stat"
}

func setIssueID(service, id string) error {
	return healthdb.PutJSON("redmine", redmineIssueKey(service), id, nil, time.Now())
}

func getIssueID(service string) (string, bool) {
	s, _, _, found, err := healthdb.GetJSON("redmine", redmineIssueKey(service))
	if err != nil || !found || s == "" || s == "0" {
		return "", false
	}
	return s, true
}

func deleteIssueID(service string) { _ = healthdb.Delete("redmine", redmineIssueKey(service)) }

func redmineCheckIssueLog(service string) bool {
	key := redmineIssueKey(service)
	jsonStr, _, _, found, err := healthdb.GetJSON("redmine", key)
	if err != nil || !found {
		return false
	}
	// historical file held plain string issue ID, sometimes "0"
	if jsonStr == "" || jsonStr == "0" {
		_ = healthdb.Delete("redmine", key)
		return false
	}
	return true
}

func redmineWrapper(service string, subject string, message string) {

	if !redmineCheckIssueLog(service) {
		Create(service, subject, message)
	} else {
		Update(service, message, true)
	}
}

func CheckUp(service string, message string) {
	// If we have a stat record, delete it and close the issue
	key := redmineStatKey(service)
	if _, _, _, found, _ := healthdb.GetJSON("redmine", key); found {
		_ = healthdb.Delete("redmine", key)
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

	key := redmineStatKey(service)
	currentDate := time.Now().Format("2006-01-02 15:04:05 -0700")

	// Load existing state from SQLite
	jsonStr, _, _, found, _ := healthdb.GetJSON("redmine", key)
	if found {
		var j common.ServiceFile
		if err := json.Unmarshal([]byte(jsonStr), &j); err != nil {
			return
		}
		if j.Locked {
			return
		}
		oldDateParsed, err := time.Parse("2006-01-02 15:04:05 -0700", j.Date)
		if err != nil {
			log.Error().Err(err).Str("date", j.Date).Msg("Failed to parse date from state")
			oldDateParsed = time.Now().Add(-25 * time.Hour)
		}

		fin := &common.ServiceFile{Date: currentDate, Locked: true}
		if interval == 0 {
			if oldDateParsed.Format("2006-01-02") != time.Now().Format("2006-01-02") {
				data, _ := json.Marshal(&common.ServiceFile{Date: currentDate, Locked: false})
				_ = healthdb.PutJSON("redmine", key, string(data), nil, time.Now())
				redmineWrapper(service, subject, message)
			}
			return
		}

		if time.Since(oldDateParsed).Hours() > 24 {
			data, _ := json.Marshal(fin)
			_ = healthdb.PutJSON("redmine", key, string(data), nil, time.Now())
			redmineWrapper(service, subject, message)
		} else {
			timeDiff := time.Since(oldDateParsed)
			if timeDiff.Minutes() >= interval {
				data, _ := json.Marshal(fin)
				_ = healthdb.PutJSON("redmine", key, string(data), nil, time.Now())
				redmineWrapper(service, subject, message)
			}
		}
		return
	}

	// No existing state: create initial unlocked record
	data, _ := json.Marshal(&common.ServiceFile{Date: currentDate, Locked: false})
	_ = healthdb.PutJSON("redmine", key, string(data), nil, time.Now())
	if interval == 0 {
		redmineWrapper(service, subject, message)
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

	// issue id now in SQLite (no more filePath)

	log.Debug().Str("component", "redmine").Str("operation", "create_issue").Msg("Using SQLite to store service issue id")

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
		assignedToId, _ := getAssignedToId(existingIssueId) // ok=true only if API succeeded AND assigned

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

		existingIssueIdAtoi, err := strconv.Atoi(existingIssueId)
		if err != nil {
			log.Error().Err(err).Str("component", "redmine").Str("operation", "create_issue").Str("input", existingIssueId).Msg("Failed to convert existing issue ID to integer")
			return
		}

		// Reopen the issue (status ID 8 = "In Progress")
		// Mevcut atamayı koruyarak body'ye geri yaz; böylece Redmine'ın varsayılan
		// "atanmamış" davranışı manuel atamayı ezmesin. Okuma başarısız olursa
		// alanı hiç gönderme.
		issue := Issue{
			Id:       existingIssueIdAtoi,
			Notes:    "Sorun devam ettiğinden iş yeniden açıldı.\n" + message,
			StatusId: 8,
		}
		if assignedToId != "" {
			// (existing behavior: only set if we successfully read a positive
			// ID; fail-safe for API errors / unassigned, matches previous code.)
			issue.AssignedToId = assignedToId
		}
		body := RedmineIssue{Issue: issue}

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

						// Persist the issue ID in SQLite
						_ = setIssueID(service, existingIssueId)
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

    // Determine tracker (job type). Default stays 7; upCheck uses 3
    trackerId := 7
    if strings.HasPrefix(service, "upcheck/") {
        trackerId = 3
    }

    body := RedmineIssue{Issue: Issue{ProjectId: projectId, TrackerId: trackerId, Description: message, Subject: subject, PriorityId: priorityId}}

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

	// persist issue id in SQLite
	_ = setIssueID(service, strconv.Itoa(data.Issue.Id))

	descPreview := message
	if len(descPreview) > 200 {
		descPreview = descPreview[:200] + "…"
	}
	log.Info().
		Str("component", "redmine").
		Str("operation", "create_issue").
		Int("issue_id", data.Issue.Id).
		Str("service", service).
		Str("subject", subject).
		Str("desc_preview", descPreview).
		Msg("Successfully created new Redmine issue")
}

func ExistsNote(service string, message string) bool {
	// Check if a note in an issue already exists
	idStr, ok := getIssueID(service)
	if !ok {
		return false
	}
	redmineUrlFinal := common.Config.Redmine.Url + "/issues/" + idStr + ".json?include=journals"

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

	if !redmineCheckIssueLog(service) {
		return
	}
	idStr, ok := getIssueID(service)
	if !ok {
		return
	}
	issueId, err := strconv.Atoi(idStr)
	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "update_issue").Str("content", idStr).Str("service", service).Msg("Failed to convert issue ID to integer")
		return
	}
	if issueId == 0 {
		deleteIssueID(service)
		return
	}

	// Issue Redmine'da kapatılmış/silinmişse not düşme; sadece durumu KESİN biliyorsak
	// (known) işlem yap. Bilinmiyorsa (network/parse hatası) state'e dokunma (fail-safe).
	if closed, known := issueIsClosed(idStr); known && closed {
		log.Info().
			Str("component", "redmine").
			Str("operation", "update_issue").
			Int("issue_id", issueId).
			Str("service", service).
			Msg("Issue Redmine'da kapalı/erişilemez; monokit not düşmüyor ve yerel kaydı temizliyor")
		deleteIssueID(service)
		return
	}

	// update issue
	// Mevcut atamayı koruyarak body'ye geri yaz; bazı Redmine ayarları
	// (workflow rules, plugin'ler) PUT'ta belirtilmeyen alanı "atanmamış"a
	// çevirebiliyor. Okuma başarısız olursa alanı hiç gönderme.
	issue := Issue{Id: issueId, Notes: message}
	if existingAssigned, known := getAssignedToId(idStr); known && existingAssigned != "" {
		issue.AssignedToId = existingAssigned
	}
	body := RedmineIssue{Issue: issue}

	jsonBody, err := json.Marshal(body)

	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "update_issue").Str("service", service).Int("issue_id", issueId).Msg("Failed to marshal issue update request")
	}

	updateUrl := common.Config.Redmine.Url + "/issues/" + idStr + ".json"
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

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		preview := message
		if len(preview) > 200 {
			preview = preview[:200] + "…"
		}
		log.Info().
			Str("component", "redmine").
			Str("operation", "update_issue").
			Int("issue_id", issueId).
			Str("service", service).
			Int("note_length", len(message)).
			Str("note_preview", preview).
			Msg("Successfully updated Redmine issue")
	}
}

// getAssignedToId fetches the current assignee of a Redmine issue.
//
// Return values:
//   - assignedID != "", ok=true:  issue is assigned to the user with this ID
//   - assignedID == "", ok=true:  issue is unassigned (API succeeded, no assignee)
//   - assignedID == "", ok=false: API lookup failed (network, non-200, parse);
//     caller must treat as "unknown" and apply fail-safe behavior
//     (do not override the assignee).
func getAssignedToId(id string) (assignedID string, ok bool) {

	// Make request to Redmine API to get the assigned_to_id
	redmineUrlFinal := common.Config.Redmine.Url + "/issues/" + id + ".json"

	req, err := http.NewRequest("GET", redmineUrlFinal, nil)
	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "get_assigned_to_id").Str("url", redmineUrlFinal).Str("issue_id", id).Msg("Failed to create request for issue assignment info")
		return "", false
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
		return "", false
	}

	defer resp.Body.Close()

	log.Debug().
		Str("component", "redmine").
		Str("operation", "get_assigned_to_id").
		Str("issue_id", id).
		Int("status_code", resp.StatusCode).
		Msg("Read issue to preserve current assignee")

	// read response and get assigned_to_id
	var data map[string]interface{}

	err = json.NewDecoder(resp.Body).Decode(&data)

	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "get_assigned_to_id").Str("issue_id", id).Msg("Failed to decode issue assignment response")
		return "", false
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
		return "", false
	}

	// Check if id exists

	issueObj, ok := data["issue"].(map[string]interface{})
	if !ok {
		return "", false
	}
	assignedTo, ok := issueObj["assigned_to"]
	if !ok || assignedTo == nil {
		return "", true
	}
	assignedMap, ok := assignedTo.(map[string]interface{})
	if !ok {
		return "", true
	}
	idFloat, idOk := assignedMap["id"].(float64)
	if !idOk {
		return "", true
	}
	return strconv.Itoa(int(idFloat)), true
}

// issueIsClosed, bir issue'nun Redmine'da kapalı/silinmiş olup olmadığını sorgular.
//   - closed=true,  known=true: issue kapalı (status.is_closed veya status.id>=5) ya da 404 (silinmiş).
//   - closed=false, known=true: issue açık.
//   - known=false: durum kesin bilinemedi (network/timeout/5xx/403/parse hatası);
//     çağıran taraf state'e dokunmamalı (fail-safe).
func issueIsClosed(id string) (closed bool, known bool) {
	redmineUrlFinal := common.Config.Redmine.Url + "/issues/" + id + ".json"

	req, err := http.NewRequest("GET", redmineUrlFinal, nil)
	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "issue_is_closed").Str("url", redmineUrlFinal).Str("issue_id", id).Msg("Failed to create request for issue status")
		return false, false
	}
	common.AddUserAgent(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Redmine-API-Key", common.Config.Redmine.Api_key)

	client := &http.Client{
		Timeout: time.Second * 10,
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "issue_is_closed").Str("url", redmineUrlFinal).Str("issue_id", id).Msg("Failed to send request for issue status")
		return false, false
	}
	defer resp.Body.Close()

	// 404 → issue silinmiş; artık geçerli değil, kapalı muamelesi yap
	if resp.StatusCode == 404 {
		return true, true
	}

	// 200 dışında her şey (5xx, 403, vb.) → durum bilinemez
	if resp.StatusCode != 200 {
		return false, false
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "issue_is_closed").Str("issue_id", id).Msg("Failed to decode issue status response")
		return false, false
	}

	issueObj, ok := data["issue"].(map[string]interface{})
	if !ok {
		return false, false
	}
	status, ok := issueObj["status"].(map[string]interface{})
	if !ok {
		return false, false
	}

	// Tercihen is_closed boolean'ı; yoksa status.id>=5 fallback (5=Closed)
	if isClosed, ok := status["is_closed"].(bool); ok {
		return isClosed, true
	}
	if statusId, ok := status["id"].(float64); ok {
		return statusId >= 5, true
	}

	return false, false
}

func Close(service string, message string) {
	if !common.Config.Redmine.Enabled {
		return
	}

	// Ensure we have an issue ID
	if id, ok := getIssueID(service); !ok || id == "" {
		return
	}

	if !redmineCheckIssueLog(service) {
		return
	}

	// read id from sqlite
	idStr, _ := getIssueID(service)
	issueId, err := strconv.Atoi(idStr)
	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "close_issue").Str("content", idStr).Str("service", service).Msg("Failed to convert issue ID to integer for closing")
	}

	// if parse failed or id empty, stop
	if issueId == 0 {
		deleteIssueID(service)
		return
	}

	// If the issue is already closed in Redmine (by a human), monokit neither
	// posts a note nor reassigns to the bot. Idempotent close.
	if closed, known := issueIsClosed(idStr); known && closed {
		log.Info().
			Str("component", "redmine").
			Str("operation", "close_issue").
			Int("issue_id", issueId).
			Str("service", service).
			Msg("Issue already closed in Redmine; skipping close and assignee handoff")
		deleteIssueID(service)
		return
	}

	// Auto-close reassign policy (per ops requirement):
	//   - UNASSIGNED issue               → Redmine Bot (Mono) takes ownership.
	//   - Assigned to the SAME BOT       → idempotent, keep bot assignee.
	//   - Assigned to a HUMAN / other    → close but DO NOT touch the
	//                                     assignee. Never steal an open
	//                                     ticket from a human operator.
	//   - Redmine API lookup failed      → fail-safe: leave AssignedToId
	//                                     unset so Redmine keeps whatever
	//                                     assignee it already has.
	issue := Issue{
		Id:       issueId,
		Notes:    message,
		StatusId: 5,
	}
	botID := getBotUserID()
	if botID != "" {
		if currentAssignee, known := getAssignedToId(idStr); known {
			if currentAssignee == "" || currentAssignee == botID {
				issue.AssignedToId = botID
			} else {
				log.Info().
					Str("component", "redmine").
					Str("operation", "close_issue").
					Int("issue_id", issueId).
					Str("service", service).
					Str("current_assignee_id", currentAssignee).
					Msg("Issue is assigned to a non-bot user; preserving existing assignee on auto-close (no reassign to bot)")
			}
		} else {
			log.Warn().
				Str("component", "redmine").
				Str("operation", "close_issue").
				Int("issue_id", issueId).
				Str("service", service).
				Msg("Could not read current assignee; preserving existing assignee on auto-close (fail-safe)")
		}
	}
	body := RedmineIssue{Issue: issue}
	jsonBody, err := json.Marshal(body)

	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "close_issue").Str("service", service).Int("issue_id", issueId).Msg("Failed to marshal issue close request")
	}

	closeUrl := common.Config.Redmine.Url + "/issues/" + idStr + ".json"
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

	// remove stored id
	deleteIssueID(service)

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

	if !redmineCheckIssueLog(service) {
		return ""
	}
	id, ok := getIssueID(service)
	if !ok {
		return ""
	}
	return id
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

// redminePercentKey: her service+partition için son yüzdeyi saklayan healthdb anahtarı
func redminePercentKey(service, partition string) string {
	// hem service hem partition içersin; "/", " " karakterlerini güvenli hale getir
	s := strings.NewReplacer("/", "-", " ", "_").Replace(service)
	p := strings.NewReplacer("/", "-", " ", "_").Replace(partition)
	return s + ":redmine:pct:" + p
}

func getLastPercent(service, partition string) (float64, bool) {
	s, _, _, found, err := healthdb.GetJSON("redmine", redminePercentKey(service, partition))
	if err != nil || !found || s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// LastReportedPercent, bir service+partition için en son kaydedilmiş yüzdeyi döner.
// Tüketiciler (örn. osHealth) mesaj zenginleştirmesi için kullanabilir.
func LastReportedPercent(service, partition string) (float64, bool) {
	return getLastPercent(service, partition)
}

func setLastPercent(service, partition string, pct float64) {
	_ = healthdb.PutJSON("redmine", redminePercentKey(service, partition), strconv.FormatFloat(pct, 'f', 2, 64), nil, time.Now())
}

func clearLastPercent(service, partition string) {
	_ = healthdb.Delete("redmine", redminePercentKey(service, partition))
}

// ClearAllPercentTracking, bir service için tutulan tüm yüzde tracking kayıtlarını siler.
// CheckUp içinden veya service normale döndüğünde çağrılabilir.
func ClearAllPercentTracking(service string) {
	// TODO: healthdb şu an tüm key'leri listeleyen bir API sunmuyor; service normale dönünce
	// CheckDownOnIncrease içindeki yeni çağrılar getLastPercent okur ve üzerine yazar, bu yüzden
	// stale kayıtlar bir sonraki artışta doğal olarak ezilir. Eğer istenirse healthdb'ye
	// PrefixList/PrefixDelete eklenip burada çağrılabilir.
	_ = service
}

// CheckDownOnIncrease, eşiği aşan (veya aşmış olan) bir durum için:
//   - issue yoksa: Create + son yüzdeyi kaydet
//   - issue varsa VE currentPct > kaydedilmişPct (yani yüzde tırmandıysa): Update + yeni yüzdeyi kaydet
//   - yoksa (düştüyse veya sabitse): hiçbir şey yapma
//
// Davranış CheckDown'daki zaman aralığı throttle'undan bağımsızdır; amaç kritik artışları kaçırmamaktır.
//
// partition: örn. "/var", "/". Boş olmamalı; mountpoint tanımlayıcı.
//
// createMessage yeni issue açılırken Description olarak; updateMessage ise mevcut
// issue'ya not (journal) düşerken kullanılır. Böylece "yükseldi" başlığı yalnızca
// not'ta görünür, yeni açılan issue'nun Description'ında görünmez.
func CheckDownOnIncrease(service, subject, createMessage, updateMessage, partition string, currentPct float64) {
	if partition == "" {
		return
	}
	if !redmineCheckIssueLog(service) {
		// issue yok → aç, yüzdeyi kaydet
		Create(service, subject, createMessage)
		setLastPercent(service, partition, currentPct)
		touchStat(service)
		return
	}
	last, ok := getLastPercent(service, partition)
	if !ok {
		// issue var ama yüzde kaydı yok (eski davranıştan kalan bir issue olabilir) → notu at, kaydet
		Update(service, updateMessage, false)
		setLastPercent(service, partition, currentPct)
		touchStat(service)
		return
	}
	currentRounded := math.Round(currentPct)
	lastRounded := math.Round(last)
	if currentRounded > lastRounded {
		Update(service, updateMessage, false)
		setLastPercent(service, partition, currentPct)
		touchStat(service)
	}
}

// touchStat, :redmine:stat namespace'ine şu anki zamanı yazar; böylece CheckUp
// eşik altına düşüldüğünde issue'yu kapatabilsin. CheckDown'un throttle kaydıyla
// aynı şekildedir.
func touchStat(service string) {
	currentDate := time.Now().Format("2006-01-02 15:04:05 -0700")
	data, _ := json.Marshal(&common.ServiceFile{Date: currentDate, Locked: true})
	_ = healthdb.PutJSON("redmine", redmineStatKey(service), string(data), nil, time.Now())
}
