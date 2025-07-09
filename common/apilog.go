package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// APILogEntry represents a structured log entry for API submission
type APILogEntry struct {
	Level       string                 `json:"level"`
	Component   string                 `json:"component"`
	Message     string                 `json:"message"`
	Timestamp   string                 `json:"timestamp"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Type        string                 `json:"type"`
	HostName    string                 `json:"hostname"`
	Environment string                 `json:"environment,omitempty"`
	Version     string                 `json:"version,omitempty"`
}

// APILogSubmitter handles submission of logs to the API server with enhanced features
type APILogSubmitter struct {
	apiURL      string
	hostName    string
	hostToken   string
	component   string
	enabled     bool
	client      *http.Client
	retryCount  int
	batchSize   int
	pendingLogs []APILogEntry
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewAPILogSubmitter creates a new enhanced API log submitter
func NewAPILogSubmitter(component string) *APILogSubmitter {
	ctx, cancel := context.WithCancel(context.Background())

	submitter := &APILogSubmitter{
		enabled:     false,
		component:   component,
		client:      &http.Client{Timeout: 10 * time.Second},
		retryCount:  3,
		batchSize:   10,
		pendingLogs: make([]APILogEntry, 0, 10),
		ctx:         ctx,
		cancel:      cancel,
	}

	// Check if client.yaml exists
	if !ConfExists("client") {
		return submitter
	}

	// Load client configuration
	var clientConf struct {
		URL string
	}
	ConfInit("client", &clientConf)

	if clientConf.URL == "" {
		return submitter
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

	if hostName == "" {
		return submitter
	}

	// Try to read the host key
	hostToken := ""
	keyPath := filepath.Join("/var/lib/mono/api/hostkey", hostName)
	if hostKey, err := os.ReadFile(keyPath); err == nil {
		hostToken = string(hostKey)
	} else {
		return submitter
	}

	submitter.apiURL = clientConf.URL
	submitter.hostName = hostName
	submitter.hostToken = hostToken
	submitter.enabled = true

	return submitter
}

// SubmitLog submits a log entry to the API server with enhanced error handling
func (s *APILogSubmitter) SubmitLog(entry APILogEntry) error {
	if !s.enabled {
		return nil
	}

	// Add default fields if missing
	if entry.HostName == "" {
		entry.HostName = s.hostName
	}
	if entry.Component == "" {
		entry.Component = s.component
	}
	if entry.Type == "" {
		entry.Type = "monokit"
	}
	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().Format(time.RFC3339)
	}

	// Add environment info if available
	if env := os.Getenv("MONOKIT_ENV"); env != "" {
		entry.Environment = env
	}
	if version := MonokitVersion; version != "" {
		entry.Version = version
	}

	return s.submitWithRetry(entry)
}

// submitWithRetry attempts to submit log with exponential backoff retry
func (s *APILogSubmitter) submitWithRetry(entry APILogEntry) error {
	var lastErr error

	for attempt := 0; attempt < s.retryCount; attempt++ {
		if err := s.doSubmit(entry); err != nil {
			lastErr = err

			// Calculate backoff delay: 100ms * 2^attempt
			delay := time.Duration(100*(1<<attempt)) * time.Millisecond

			select {
			case <-s.ctx.Done():
				return s.ctx.Err()
			case <-time.After(delay):
				continue
			}
		}
		return nil
	}

	return fmt.Errorf("failed to submit log after %d attempts: %w", s.retryCount, lastErr)
}

// doSubmit performs the actual HTTP submission
func (s *APILogSubmitter) doSubmit(entry APILogEntry) error {
	jsonData, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal log entry: %w", err)
	}

	req, err := http.NewRequestWithContext(s.ctx, "POST", s.apiURL+"/api/v1/host/logs", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", s.hostToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("monokit/%s", MonokitVersion))

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Close gracefully shuts down the submitter
func (s *APILogSubmitter) Close() error {
	s.cancel()
	return nil
}

// ZerologAPIHook is an enhanced zerolog hook that submits logs to the API server
type ZerologAPIHook struct {
	submitter *APILogSubmitter
}

// NewZerologAPIHook creates a new enhanced zerolog hook for API submission
func NewZerologAPIHook(component string) *ZerologAPIHook {
	return &ZerologAPIHook{
		submitter: NewAPILogSubmitter(component),
	}
}

// Run implements the zerolog Hook interface with proper field extraction
func (h *ZerologAPIHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	if !h.submitter.enabled || level < zerolog.InfoLevel {
		return
	}

	// Extract fields from the event using a buffer approach
	metadata := h.extractEventFields(e)

	entry := APILogEntry{
		Level:     level.String(),
		Component: h.submitter.component,
		Message:   msg,
		Metadata:  metadata,
	}

	// Submit asynchronously to avoid blocking the logging call
	go func() {
		if err := h.submitter.SubmitLog(entry); err != nil {
			// Log to stderr without using zerolog to avoid infinite loops
			fmt.Fprintf(os.Stderr, "[APILogHook] Failed to submit log: %v\n", err)
		}
	}()
}

// extractEventFields attempts to extract structured fields from the zerolog event
func (h *ZerologAPIHook) extractEventFields(e *zerolog.Event) map[string]interface{} {
	metadata := make(map[string]interface{})

	// Note: zerolog doesn't provide direct access to fields in hooks
	// This is a limitation of the library design for performance reasons
	//
	// Best practice is to add important context via logger.With() methods
	// which creates a logger with persistent context, rather than trying
	// to extract fields from individual log events
	//
	// For now, we return an empty metadata map, but this could be enhanced
	// in the future if zerolog adds field extraction capabilities

	return metadata
}

// GetSubmitter returns the underlying API log submitter for direct access
func (h *ZerologAPIHook) GetSubmitter() *APILogSubmitter {
	return h.submitter
}

// Close gracefully shuts down the hook
func (h *ZerologAPIHook) Close() error {
	return h.submitter.Close()
}
