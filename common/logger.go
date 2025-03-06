package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
)

// APILogHook is a logrus hook that sends logs to the API server
type APILogHook struct {
	apiURL    string
	hostName  string
	hostToken string
	component string
	enabled   bool
	logLevels []logrus.Level
}

// NewAPILogHook creates a new hook to send logs to the API server
func NewAPILogHook(component string) *APILogHook {
	// Check if client.yaml exists
	if !ConfExists("client") {
		return &APILogHook{enabled: false}
	}

	// Load client configuration
	var clientConf struct {
		URL string
	}
	ConfInit("client", &clientConf)

	// If URL is not configured, disable the hook
	if clientConf.URL == "" {
		return &APILogHook{enabled: false}
	}

	// Get hostname from common config
	hostName := ""
	if ConfExists("common") {
		var commonConf struct {
			Identifier string
		}
		ConfInit("common", &commonConf)
		hostName = commonConf.Identifier
	}

	// If hostname is not configured, disable the hook
	if hostName == "" {
		return &APILogHook{enabled: false}
	}

	// Try to read the host key
	hostToken := ""
	keyPath := filepath.Join("/var/lib/mono/api/hostkey", hostName)
	if hostKey, err := os.ReadFile(keyPath); err == nil {
		hostToken = string(hostKey)
	} else {
		// If host key is not available, disable the hook
		return &APILogHook{enabled: false}
	}

	return &APILogHook{
		apiURL:    clientConf.URL,
		hostName:  hostName,
		hostToken: hostToken,
		component: component,
		enabled:   true,
		logLevels: []logrus.Level{logrus.InfoLevel, logrus.WarnLevel, logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel},
	}
}

// Fire is called when a log event occurs
func (hook *APILogHook) Fire(entry *logrus.Entry) error {
	// Skip if hook is disabled
	if !hook.enabled {
		return nil
	}

	// Prepare metadata if available
	metadata := ""
	if len(entry.Data) > 0 {
		metadataBytes, err := json.Marshal(entry.Data)
		if err == nil {
			metadata = string(metadataBytes)
		}
	}

	// Send log to API server asynchronously to avoid blocking
	go func() {
		// Import the API client package
		apiClient := NewAPIClient()

		// Use the SubmitLog function from our client
		err := apiClient.SubmitLog(entry.Level.String(), hook.component, entry.Message, metadata)
		if err != nil {
			// Just silently fail, we don't want to create log loops
			// by logging errors from the log hook
		}
	}()

	return nil
}

// Levels returns the log levels that this hook is interested in
func (hook *APILogHook) Levels() []logrus.Level {
	return hook.logLevels
}

func LogInit(userMode bool) {
	logfilePath := "/var/log/monokit.log"

	if userMode {
		xdgStateHome := os.Getenv("XDG_STATE_HOME")
		if xdgStateHome == "" {
			xdgStateHome = os.Getenv("HOME") + "/.local/state"
		}

		// Create the directory if it doesn't exist
		if _, err := os.Stat(xdgStateHome + "/monokit"); os.IsNotExist(err) {
			os.MkdirAll(xdgStateHome+"/monokit", 0755)
		}

		logfilePath = xdgStateHome + "/monokit/monokit.log"
	}

	logrus.SetReportCaller(true)
	logrus.SetFormatter(&logrus.JSONFormatter{
		CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {
			fileName := path.Base(frame.File) + ":" + strconv.Itoa(frame.Line)
			//return frame.Function, fileName
			return "", fileName
		},
	})

	logFile, err := os.OpenFile(logfilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}

	multiWriter := io.MultiWriter(os.Stdout, logFile)
	logrus.SetOutput(multiWriter)

	// Get component name from the calling package
	pc, _, _, _ := runtime.Caller(1)
	funcName := runtime.FuncForPC(pc).Name()
	component := path.Base(path.Dir(funcName))

	// Add API log hook if client.yaml is configured
	apiHook := NewAPILogHook(component)
	if apiHook.enabled {
		logrus.AddHook(apiHook)
	}
}

// LogInitWithComponent initializes logging with a specific component name
func LogInitWithComponent(userMode bool, component string) {
	LogInit(userMode)

	// Replace the API hook with one that has the specified component
	// First remove any existing API hooks
	for i, hook := range logrus.StandardLogger().Hooks {
		for j, h := range hook {
			if _, ok := h.(*APILogHook); ok {
				logrus.StandardLogger().Hooks[i] = append(logrus.StandardLogger().Hooks[i][:j], logrus.StandardLogger().Hooks[i][j+1:]...)
				break
			}
		}
	}

	// Add new API hook with specified component
	apiHook := NewAPILogHook(component)
	if apiHook.enabled {
		logrus.AddHook(apiHook)
	}
}

func LogError(err string) {
	logrus.Error(err)

	// Check if client configuration exists and URL is set
	if ConfExists("client") {
		var clientConf struct {
			URL string
		}
		ConfInit("client", &clientConf)
		if clientConf.URL != "" {
			// Prepare log entry
			logEntry := struct {
				Level     string `json:"level"`
				Component string `json:"component"`
				Message   string `json:"message"`
				Timestamp string `json:"timestamp"`
				Type      string `json:"type"`
			}{
				Level:     "error",
				Component: ScriptName,
				Message:   err,
				Timestamp: time.Now().Format(time.RFC3339),
				Type:      "monokit",
			}

			// Read host key if available
			keyPath := filepath.Join("/var/lib/mono/api/hostkey", Config.Identifier)
			hostKey, _ := os.ReadFile(keyPath)

			// Create and send request
			jsonData, _ := json.Marshal(logEntry)
			req, _ := http.NewRequest("POST", clientConf.URL+"/api/v1/host/logs", bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")
			if len(hostKey) > 0 {
				req.Header.Set("Authorization", string(hostKey))
			}

			// Send request in a goroutine to avoid blocking
			go func() {
				client := &http.Client{Timeout: 5 * time.Second}
				_, err := client.Do(req)
				if err != nil {
					logrus.Error("Failed to send log to API server: " + err.Error())
				}
			}()
		}
	}
}

func LogErrorWithFields(err string, fields logrus.Fields) {
	logrus.WithFields(fields).Error(err)
}

func LogInfo(msg string) {
	logrus.Info(msg)
}

func LogInfoWithFields(msg string, fields logrus.Fields) {
	logrus.WithFields(fields).Info(msg)
}

func LogWarn(msg string) {
	logrus.Warn(msg)
}

func LogWarnWithFields(msg string, fields logrus.Fields) {
	logrus.WithFields(fields).Warn(msg)
}

func LogDebug(msg string) {
	logrus.Debug(msg)
}

func LogDebugWithFields(msg string, fields logrus.Fields) {
	logrus.WithFields(fields).Debug(msg)
}

func LogFunctionEntry(args ...interface{}) {
	// Get the caller function name
	pc, _, _, _ := runtime.Caller(2)
	funcName := runtime.FuncForPC(pc).Name()

	// Log the function entry and arguments
	logrus.Debugf("%s run with arguments %v", funcName, args)
}

func PrettyPrintStr(name string, lessOrMore bool, value string) {
	var color string
	var not string

	if lessOrMore {
		color = Green
	} else {
		color = Fail
		not = "not "
	}

	fmt.Println(Blue + name + Reset + " is " + not + color + value + Reset)
}

func PrettyPrint(name string, lessOrMore string, value float64, hasPercentage bool, wantFloat bool, enableLimit bool, limit float64) {
	var par string
	var floatDepth int
	var final string

	if hasPercentage {
		par = "%)"
	} else {
		par = ")"
	}

	if wantFloat {
		floatDepth = 2
	} else {
		floatDepth = 0
	}

	final = Blue + name + Reset

	if !enableLimit {
		final = final + " is " + lessOrMore + " (" + strconv.FormatFloat(value, 'f', floatDepth, 64) + par + Reset
	} else {
		final = final + " " + lessOrMore
		if limit > value {
			final = final + Green
		} else {
			final = final + Fail
		}

		final = final + strconv.FormatFloat(value, 'f', floatDepth, 64) + "/" + strconv.FormatFloat(limit, 'f', 0, 64) + Reset
	}

	fmt.Println(final)
}

// APIClient provides methods to interact with the API server
type APIClient struct {
	apiURL    string
	hostName  string
	hostToken string
	enabled   bool
}

// NewAPIClient creates a new API client
func NewAPIClient() *APIClient {
	// Check if client.yaml exists
	if !ConfExists("client") {
		return &APIClient{enabled: false}
	}

	// Load client configuration
	var clientConf struct {
		URL string
	}
	ConfInit("client", &clientConf)

	// If URL is not configured, disable the client
	if clientConf.URL == "" {
		return &APIClient{enabled: false}
	}

	// Get hostname from common config
	hostName := ""
	if ConfExists("common") {
		var commonConf struct {
			Identifier string
		}
		ConfInit("common", &commonConf)
		hostName = commonConf.Identifier
	}

	// If hostname is not configured, disable the client
	if hostName == "" {
		return &APIClient{enabled: false}
	}

	// Try to read the host key
	hostToken := ""
	keyPath := filepath.Join("/var/lib/mono/api/hostkey", hostName)
	if hostKey, err := os.ReadFile(keyPath); err == nil {
		hostToken = string(hostKey)
	} else {
		// If host key is not available, disable the client
		return &APIClient{enabled: false}
	}

	return &APIClient{
		apiURL:    clientConf.URL,
		hostName:  hostName,
		hostToken: hostToken,
		enabled:   true,
	}
}

// SubmitLog submits a log entry to the API server
func (c *APIClient) SubmitLog(level, component, message, metadata string) error {
	// Skip if client is disabled
	if !c.enabled {
		return nil
	}

	// Prepare log data
	logData := struct {
		Level     string `json:"level"`
		Component string `json:"component"`
		Message   string `json:"message"`
		Timestamp string `json:"timestamp"`
		Metadata  string `json:"metadata,omitempty"`
		Type      string `json:"type,omitempty"`
	}{
		Level:     level,
		Component: component,
		Message:   message,
		Timestamp: time.Now().Format(time.RFC3339),
		Metadata:  metadata,
		Type:      "monokit",
	}

	// Marshal log data to JSON
	jsonData, err := json.Marshal(logData)
	if err != nil {
		return err
	}

	// Send log to API server
	req, err := http.NewRequest("POST", c.apiURL+"/api/v1/host/logs", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	// Add host token for authentication
	req.Header.Set("Authorization", c.hostToken)
	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to submit log: %s", string(body))
	}

	return nil
}

// LogEntry represents a log entry in the client
type LogEntry struct {
	ID        uint   `json:"id"`
	HostName  string `json:"host_name"`
	Level     string `json:"level"`
	Component string `json:"component"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	Metadata  string `json:"metadata"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// LogPagination represents pagination information for log responses
type LogPagination struct {
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Pages    int64 `json:"pages"`
}

// LogsResponse represents a paginated list of logs
type LogsResponse struct {
	Logs       []LogEntry    `json:"logs"`
	Pagination LogPagination `json:"pagination"`
}

// HostLogsResponse represents a paginated list of logs for a specific host
type HostLogsResponse struct {
	HostName   string        `json:"hostname"`
	Logs       []LogEntry    `json:"logs"`
	Pagination LogPagination `json:"pagination"`
}

// LogSearchRequest represents a log search request
type LogSearchRequest struct {
	HostName    string `json:"host_name,omitempty"`
	Level       string `json:"level,omitempty"`
	Component   string `json:"component,omitempty"`
	MessageText string `json:"message_text,omitempty"`
	StartTime   string `json:"start_time,omitempty"`
	EndTime     string `json:"end_time,omitempty"`
	Page        int    `json:"page,omitempty"`
	PageSize    int    `json:"page_size,omitempty"`
}

// GetLogs retrieves all logs with pagination
func (c *APIClient) GetLogs(page, pageSize int) (*LogsResponse, error) {
	if !c.enabled {
		return nil, fmt.Errorf("API client is not enabled")
	}

	// Build URL with query parameters
	url := fmt.Sprintf("%s/api/v1/logs?page=%d&page_size=%d", c.apiURL, page, pageSize)

	// Create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Add authentication token
	req.Header.Set("Authorization", c.hostToken)

	// Send request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get logs: %s", string(body))
	}

	// Parse response
	var logsResponse LogsResponse
	if err := json.NewDecoder(resp.Body).Decode(&logsResponse); err != nil {
		return nil, err
	}

	return &logsResponse, nil
}

// GetHostLogs retrieves logs for a specific host with pagination
func (c *APIClient) GetHostLogs(hostname string, page, pageSize int) (*HostLogsResponse, error) {
	if !c.enabled {
		return nil, fmt.Errorf("API client is not enabled")
	}

	// Build URL with query parameters
	url := fmt.Sprintf("%s/api/v1/logs/%s?page=%d&page_size=%d", c.apiURL, hostname, page, pageSize)

	// Create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Add authentication token
	req.Header.Set("Authorization", c.hostToken)

	// Send request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get host logs: %s", string(body))
	}

	// Parse response
	var hostLogsResponse HostLogsResponse
	if err := json.NewDecoder(resp.Body).Decode(&hostLogsResponse); err != nil {
		return nil, err
	}

	return &hostLogsResponse, nil
}

// SearchLogs searches logs with various filters
func (c *APIClient) SearchLogs(searchRequest *LogSearchRequest) (*LogsResponse, error) {
	if !c.enabled {
		return nil, fmt.Errorf("API client is not enabled")
	}

	// Marshal search request to JSON
	jsonData, err := json.Marshal(searchRequest)
	if err != nil {
		return nil, err
	}

	// Create request
	req, err := http.NewRequest("POST", c.apiURL+"/api/v1/logs/search", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	// Add headers
	req.Header.Set("Authorization", c.hostToken)
	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to search logs: %s", string(body))
	}

	// Parse response
	var logsResponse LogsResponse
	if err := json.NewDecoder(resp.Body).Decode(&logsResponse); err != nil {
		return nil, err
	}

	return &logsResponse, nil
}
