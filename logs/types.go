package logs

import (
	"time"
)

type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Component string                 `json:"component,omitempty"`
	Version   string                 `json:"version,omitempty"`
	Operation string                 `json:"operation,omitempty"`
	Action    string                 `json:"action,omitempty"`
	Type      string                 `json:"type,omitempty"`
	Raw       map[string]interface{} `json:"-"`
}

type LogFilter struct {
	From        *time.Time
	To          *time.Time
	Component   string
	Operation   string
	Action      string
	Type        string
	FromVersion string
	Fields      map[string]string // For arbitrary key=value filtering
}

type OutputOptions struct {
	Ugly      bool
	GetFields []string // Fields to extract and output directly
}
