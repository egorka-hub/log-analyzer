package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
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

const noRequestIDKey = "<no-request-id>"

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

func ScanLogDirectory(dirPath string) ([]string, error) {
	var logFiles []string

	err := filepath.WalkDir(dirPath, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !entry.IsDir() && strings.HasSuffix(path, ".log") {
			logFiles = append(logFiles, path)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("не удалось обойти каталог %q: %w", dirPath, err)
	}

	return logFiles, nil
}

func ProcessMultipleFiles(filePaths []string) ([]LogEntry, error) {
	var allEntries []LogEntry

	for _, path := range filePaths {
		entries, err := ReadLogFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ошибка: %v\n", err)
			continue
		}

		fmt.Printf("%s: %d entries\n", filepath.Base(path), len(entries))
		allEntries = append(allEntries, entries...)
	}

	return allEntries, nil
}

func CorrelateRequests(entries []LogEntry) map[string][]LogEntry {
	grouped := make(map[string][]LogEntry)
	for _, entry := range entries {
		key := entry.RequestID
		if key == "" {
			key = noRequestIDKey
		}
		grouped[key] = append(grouped[key], entry)
	}
	return grouped
}

func DetectFailedRequests(correlatedRequests map[string][]LogEntry) []string {
	var failed []string

	for requestID, entries := range correlatedRequests {
		for _, entry := range entries {
			if entry.Level == "ERROR" || entry.Level == "WARN" {
				failed = append(failed, requestID)
				break
			}
		}

	}

	sort.Strings(failed)

	return failed
}

func FindFirstFailure(requestEntries []LogEntry) (LogEntry, bool) {
	entries := slices.Clone(requestEntries)

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})

	for _, entry := range entries {
		if entry.Level == "ERROR" || entry.Level == "WARN" {
			return entry, true
		}
	}

	return LogEntry{}, false
}

func main() {
	dir := "./logs"

	fmt.Printf("Scanning directory: %s\n", dir)

	files, err := ScanLogDirectory(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ошибка: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d log files:\n", len(files))
	for _, path := range files {
		fmt.Printf("  - %s\n", filepath.Base(path))
	}

	fmt.Print("\nProcessing files...\n")
	result, err := ProcessMultipleFiles(files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ошибка: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nTotal entries processed:", len(result))

	grouped := CorrelateRequests(result)
	orphaned := grouped[noRequestIDKey]
	delete(grouped, noRequestIDKey)

	fmt.Println("Unique requests found:", len(grouped))
	fmt.Println("Orphaned entries (no request_id):", len(orphaned))

	var sampleID string
	for id, reqEntries := range grouped {
		if sampleID == "" || len(reqEntries) > len(grouped[sampleID]) ||
			(len(reqEntries) == len(grouped[sampleID]) && id < sampleID) {
			sampleID = id
		}
	}

	if sampleID != "" {
		fmt.Printf("\nSample correlated request (%s):\n", sampleID)
		for _, e := range grouped[sampleID] {
			fmt.Printf("  - %s: %s\n", e.Service, e.Message)
		}
	}

	failed := DetectFailedRequests(grouped)

	fmt.Println("\nFailed requests analysis:")
	fmt.Printf("Total failed requests: %d out of %d\n", len(failed), len(grouped))

	fmt.Println("\nFailed request IDs:")
	for _, id := range failed {
		fmt.Printf("  - %s\n", id)
	}

	if len(failed) > 0 {
		id := failed[0]
		if entry, ok := FindFirstFailure(grouped[id]); ok {
			fmt.Printf("\nFirst failure for %s:\n", id)
			fmt.Printf("  Service: %s\n", entry.Service)
			fmt.Printf("  Error: %s\n", entry.Message)
			fmt.Printf("  Time: %s\n", entry.Timestamp.Format(time.RFC3339Nano))
		}
	}
}
