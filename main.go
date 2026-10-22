package main

import (
	"bufio"
	"fmt"
	"os"
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
		return LogEntry{}, fmt.Errorf("неверный формат строки: %q", line)
	}

	date := matches[1]
	level := matches[2]
	service := matches[3]
	message := matches[4]

	t, err := time.Parse(time.RFC3339, date)
	if err != nil {
		return LogEntry{}, fmt.Errorf("неверный формат времени %q: %w", date, err)
	}

	switch level {
	case "INFO", "WARN", "ERROR":
	default:
		return LogEntry{}, fmt.Errorf("неизвестный уровень %q", level)
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

func ReadLogFile(filename string) ([]LogEntry, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть файл %q: %w", filename, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var entries []LogEntry
	for scanner.Scan() {
		line := scanner.Text()

		if strings.TrimSpace(line) == "" {
			continue
		}

		entry, err := ParseLogLine(line)
		if err != nil {
			fmt.Fprintf(os.Stderr, "не удалось обработать строку: %v\n", err)
			continue
		}
		entries = append(entries, entry)
	}
	if err = scanner.Err(); err != nil {
		return nil, fmt.Errorf("ошибка чтения файла: %w", err)
	}

	return entries, nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "использование: go run . <файл с логами>")
		os.Exit(1)
	}
	filename := os.Args[1]

	entries, err := ReadLogFile(filename)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ошибка:", err)
		os.Exit(1)
	}

	fmt.Printf("Прочитано записей: %d\n\n", len(entries))
	for _, entry := range entries {
		fmt.Println(entry)
	}
}
