package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/mshogin/costlint/pkg/counter"
	"github.com/mshogin/costlint/pkg/pricing"
	"github.com/mshogin/costlint/pkg/reporter"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: costlint {count|estimate|compare|report}\n\n")
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  count              Count tokens from stdin\n")
		fmt.Fprintf(os.Stderr, "  estimate --model X Estimate cost for model\n")
		fmt.Fprintf(os.Stderr, "  compare            Compare costs across all models\n")
		fmt.Fprintf(os.Stderr, "  report --source X  Generate cost report from telemetry JSONL\n")
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "count":
		runCount()
	case "estimate":
		runEstimate()
	case "compare":
		runCompare()
	case "report":
		runReport()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		os.Exit(1)
	}
}

func runCount() {
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	tc := counter.Count(string(input))
	out, _ := json.MarshalIndent(tc, "", "  ")
	fmt.Println(string(out))
}

func runEstimate() {
	model := "sonnet"
	for i, arg := range os.Args[2:] {
		if arg == "--model" && i+3 < len(os.Args) {
			model = os.Args[i+3]
		}
	}

	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	tc := counter.Count(string(input))
	outputEstimate := counter.EstimateOutput(tc.Input, "")
	cost := pricing.Estimate(model, tc.Input, outputEstimate)

	result := map[string]interface{}{
		"model":         model,
		"input_tokens":  tc.Input,
		"output_tokens": outputEstimate,
		"total_tokens":  tc.Input + outputEstimate,
		"cost_usd":      cost,
	}
	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))
}

func runCompare() {
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	tc := counter.Count(string(input))
	outputEstimate := counter.EstimateOutput(tc.Input, "")
	costs := pricing.CompareModels(tc.Input, outputEstimate)

	result := map[string]interface{}{
		"input_tokens":  tc.Input,
		"output_tokens": outputEstimate,
		"costs":         costs,
	}
	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))
}

func runReport() {
	source := ""
	for i, arg := range os.Args[2:] {
		if arg == "--source" && i+3 < len(os.Args) {
			source = os.Args[i+3]
		}
	}

	if source == "" {
		fmt.Fprintf(os.Stderr, "Usage: costlint report --source telemetry.jsonl\n")
		os.Exit(1)
	}

	report, err := reporter.GenerateFromFile(source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(reporter.FormatReport(report))
}
