package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"log-analyzer/internal/cli"
	"log-analyzer/internal/processor"
	"log-analyzer/internal/reporter"
	"log-analyzer/internal/scanner"
)

func main() {
	start := time.Now()

	cfg, err := cli.ParseCommandLineArgs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Scanning directory: %s\n", cfg.InputDir)

	files, err := scanner.ScanLogDirectory(cfg.InputDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d log files:\n", len(files))
	for _, path := range files {
		fmt.Printf("  - %s\n", filepath.Base(path))
	}

	fmt.Print("\nProcessing files...\n")
	result, err := processor.ProcessMultipleFiles(files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nTotal entries processed:", len(result))

	grouped := processor.CorrelateRequests(result)
	orphaned := grouped[processor.NoRequestIDKey]
	delete(grouped, processor.NoRequestIDKey)

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

	failed := processor.DetectFailedRequests(grouped)

	fmt.Println("\nFailed requests analysis:")
	fmt.Printf("Total failed requests: %d out of %d\n", len(failed), len(grouped))

	fmt.Println("\nFailed request IDs:")
	for _, id := range failed {
		fmt.Printf("  - %s\n", id)
	}

	if len(failed) > 0 {
		id := failed[0]
		if entry, ok := processor.FindFirstFailure(grouped[id]); ok {
			fmt.Printf("\nFirst failure for %s:\n", id)
			fmt.Printf("  Service: %s\n", entry.Service)
			fmt.Printf("  Error: %s\n", entry.Message)
			fmt.Printf("  Time: %s\n", entry.Timestamp.Format(time.RFC3339Nano))
		}
	}

	if len(failed) > 0 {
		id := failed[0]
		fmt.Printf("\nFailed request timeline: %s\n\n", id)
		for _, e := range processor.SortTimelineByTimestamp(grouped[id]) {
			fmt.Printf("%s %-7s %s: %s\n",
				e.Timestamp.Format("15:04:05.000Z07:00"), "["+e.Level+"]", e.Service, e.Message)
		}
	}

	reports := make([]reporter.FailedRequestReport, 0, len(failed))
	for _, id := range failed {
		report := reporter.FailedRequestReport{RequestID: id}
		if entry, ok := processor.FindFirstFailure(grouped[id]); ok {
			report.FailingService = entry.Service
			report.ErrorMessage = entry.Message
		}
		for _, e := range processor.SortTimelineByTimestamp(grouped[id]) {
			report.Timeline = append(report.Timeline, fmt.Sprintf("%s [%s] %s: %s",
				e.Timestamp.Format("15:04:05.000Z07:00"), e.Level, e.Service, e.Message))
		}
		reports = append(reports, report)
	}

	report := reporter.AnalysisResult{
		TotalEntriesProcessed: len(result),
		FailedRequestsFound:   len(failed),
		ProcessingTimeSeconds: time.Since(start).Seconds(),
		FailedRequests:        reports,
	}

	if err := reporter.WriteJSONReport(report, cfg.OutputFile); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\nReport saved to %s\n", cfg.OutputFile)
}
