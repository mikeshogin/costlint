package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/mshogin/costlint/pkg/budget"
	"github.com/mshogin/costlint/pkg/counter"
	"github.com/mshogin/costlint/pkg/pricing"
	"github.com/mshogin/costlint/pkg/reporter"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: costlint {count|estimate|compare|report|budget}\n\n")
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  count                          Count tokens from stdin\n")
		fmt.Fprintf(os.Stderr, "  estimate --model X             Estimate cost for model\n")
		fmt.Fprintf(os.Stderr, "  compare                        Compare costs across all models\n")
		fmt.Fprintf(os.Stderr, "  report --source X              Generate cost report from telemetry JSONL\n")
		fmt.Fprintf(os.Stderr, "  budget --max N --model X       Track cumulative cost; alert at 80%%, exit 1 when exceeded\n")
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
	case "budget":
		runBudget()
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

// runBudget reads stdin line by line, tracks cumulative token cost and alerts
// when 80% of the budget is consumed, exiting with code 1 when the budget is exceeded.
func runBudget() {
	maxUSD := 0.0
	model := "sonnet"
	args := os.Args[2:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--max":
			if i+1 < len(args) {
				i++
				v, err := strconv.ParseFloat(args[i], 64)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Invalid --max value: %s\n", args[i])
					os.Exit(1)
				}
				maxUSD = v
			}
		case "--model":
			if i+1 < len(args) {
				i++
				model = args[i]
			}
		}
	}

	if maxUSD <= 0 {
		fmt.Fprintf(os.Stderr, "Usage: costlint budget --max <usd> [--model <model>]\n")
		fmt.Fprintf(os.Stderr, "  --max    Maximum budget in USD (e.g. 5.00)\n")
		fmt.Fprintf(os.Stderr, "  --model  Model key: haiku, sonnet (default), opus\n")
		os.Exit(1)
	}

	b := budget.NewBudget(maxUSD, model)
	alerted := false

	scanner := bufio.NewScanner(os.Stdin)
	// Increase scanner buffer for long lines (e.g. large context dumps).
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		tc := counter.Count(line)
		b.Add(tc.Input)

		if msg := b.Alert(); msg != "" {
			if b.IsOverBudget() {
				fmt.Fprintf(os.Stderr, "%s\n", msg)
				data, _ := b.ToJSON()
				fmt.Println(string(data))
				os.Exit(1)
			}
			if !alerted {
				fmt.Fprintf(os.Stderr, "%s\n", msg)
				alerted = true
			}
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
		os.Exit(1)
	}

	data, err := b.ToJSON()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error serialising budget: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(data))

	if b.IsOverBudget() {
		os.Exit(1)
	}
}
