package parser

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type LogEntry struct {
	Timestamp time.Time
	Level     string
	Service   string
	Message   string
	RequestID string
	UserID    string
}

func (e LogEntry) String() string {
	return fmt.Sprintf("LogEntry{\n  Timestamp: %v\n  Level: %s\n  Service: %s\n  Message: %s\n  RequestID: %s\n  UserID: %s\n}",
		e.Timestamp, e.Level, e.Service, e.Message, e.RequestID, e.UserID)
}

var lineRe = regexp.MustCompile(`^(\S+) \[(\w+)] ([\w-]+): (.+)$`)
var requestIDRe = regexp.MustCompile(`\brequest_id=(\w+)`)
var userIDRe = regexp.MustCompile(`\buser_id=(\w+)`)

func ParseLogLine(line string) (LogEntry, error) {
	line = strings.TrimSpace(line)

	matches := lineRe.FindStringSubmatch(line)
	if matches == nil {
		return LogEntry{}, fmt.Errorf("invalid line format: %q", line)
	}

	date := matches[1]
	level := matches[2]
	service := matches[3]
	message := matches[4]

	t, err := time.Parse(time.RFC3339, date)
	if err != nil {
		return LogEntry{}, fmt.Errorf("invalid timestamp %q: %w", date, err)
	}

	switch level {
	case "INFO", "WARN", "ERROR":
	default:
		return LogEntry{}, fmt.Errorf("unknown log level %q", level)
	}

	var requestID, userID string
	if m := requestIDRe.FindStringSubmatch(message); m != nil {
		requestID = m[1]
	}
	if m := userIDRe.FindStringSubmatch(message); m != nil {
		userID = m[1]
	}

	return LogEntry{
		Timestamp: t,
		Level:     level,
		Service:   service,
		Message:   message,
		RequestID: requestID,
		UserID:    userID,
	}, nil

}
