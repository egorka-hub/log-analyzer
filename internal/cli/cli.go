package cli

import (
	"flag"
	"fmt"
	"os"
)

type Config struct {
	InputDir   string
	OutputFile string
}

func ParseCommandLineArgs() (Config, error) {
	inputDir := flag.String("input-dir", ".", "Directory containing .log files")
	outputFile := flag.String("output-file", "results.json", "JSON output file path")

	flag.Parse()

	info, err := os.Stat(*inputDir)
	if err != nil {
		return Config{}, fmt.Errorf("invalid input dir: %w", err)
	}
	if !info.IsDir() {
		return Config{}, fmt.Errorf("%q is not a directory", *inputDir)
	}

	return Config{
		InputDir:   *inputDir,
		OutputFile: *outputFile,
	}, nil
}
