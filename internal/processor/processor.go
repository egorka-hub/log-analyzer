package processor

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"log-analyzer/internal/parser"
)

const NoRequestIDKey = "<no-request-id>"

func ReadLogFile(filename string) ([]parser.LogEntry, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %q: %w", filename, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var entries []parser.LogEntry
	for scanner.Scan() {
		line := scanner.Text()

		if strings.TrimSpace(line) == "" {
			continue
		}

		entry, err := parser.ParseLogLine(line)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to parse line: %v\n", err)
			continue
		}
		entries = append(entries, entry)
	}
	if err = scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return entries, nil
}

func ProcessMultipleFiles(filePaths []string) ([]parser.LogEntry, error) {
	var allEntries []parser.LogEntry

	for _, path := range filePaths {
		entries, err := ReadLogFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			continue
		}

		fmt.Printf("%s: %d entries\n", filepath.Base(path), len(entries))
		allEntries = append(allEntries, entries...)
	}

	return allEntries, nil
}

func CorrelateRequests(entries []parser.LogEntry) map[string][]parser.LogEntry {
	grouped := make(map[string][]parser.LogEntry)
	for _, entry := range entries {
		key := entry.RequestID
		if key == "" {
			key = NoRequestIDKey
		}
		grouped[key] = append(grouped[key], entry)
	}
	return grouped
}

func DetectFailedRequests(correlatedRequests map[string][]parser.LogEntry) []string {
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

func FindFirstFailure(requestEntries []parser.LogEntry) (parser.LogEntry, bool) {
	entries := slices.Clone(requestEntries)

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})

	for _, entry := range entries {
		if entry.Level == "ERROR" || entry.Level == "WARN" {
			return entry, true
		}
	}

	return parser.LogEntry{}, false
}

func SortTimelineByTimestamp(entries []parser.LogEntry) []parser.LogEntry {
	sorted := make([]parser.LogEntry, len(entries))
	copy(sorted, entries)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})

	return sorted
}
