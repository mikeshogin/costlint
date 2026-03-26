package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/mshogin/costlint/pkg/ab"
	"github.com/mshogin/costlint/pkg/budget"
	"github.com/mshogin/costlint/pkg/cache"
	"github.com/mshogin/costlint/pkg/counter"
	"github.com/mshogin/costlint/pkg/daily"
	"github.com/mshogin/costlint/pkg/perf"
	"github.com/mshogin/costlint/pkg/pricing"
	"github.com/mshogin/costlint/pkg/reporter"
	"github.com/mshogin/costlint/pkg/telemetry"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: costlint {count|estimate|compare|subscription|report|budget|ab|cache|perf}\n\n")
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  count                                              Count tokens from stdin\n")
		fmt.Fprintf(os.Stderr, "  estimate --model X                                 Estimate cost for model\n")
		fmt.Fprintf(os.Stderr, "  compare                                            Compare costs across all models\n")
		fmt.Fprintf(os.Stderr, "  subscription --plan X --model Y --tokens N         Compare subscription vs pay-as-you-go\n")
		fmt.Fprintf(os.Stderr, "  report --source X                                  Generate cost report from telemetry JSONL\n")
		fmt.Fprintf(os.Stderr, "  budget --max N --model X                           Track cumulative cost; alert at 80%%, exit 1 when exceeded\n")
		fmt.Fprintf(os.Stderr, "  ab --name T --model-a A --model-b B               A/B cost comparison; reads prompts from stdin\n")
		fmt.Fprintf(os.Stderr, "  cache --model X                                    Prompt caching telemetry; reads prompts from stdin\n")
		fmt.Fprintf(os.Stderr, "  perf                                               Benchmark all operations and report latency as JSON\n")
		fmt.Fprintf(os.Stderr, "  telemetry ingest                                   Ingest promptlint JSONL from stdin into ~/.costlint-telemetry.jsonl\n")
		fmt.Fprintf(os.Stderr, "  telemetry summary                                  Show aggregated metrics from ~/.costlint-telemetry.jsonl\n")
		fmt.Fprintf(os.Stderr, "  daily [--format json|text]                         Generate today's cost report with trend and anomaly detection\n")
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
	case "subscription":
		runSubscription()
	case "report":
		runReport()
	case "budget":
		runBudget()
	case "ab":
		runAB()
	case "cache":
		runCache()
	case "perf":
		runPerf()
	case "telemetry":
		runTelemetry()
	case "daily":
		runDaily()
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

// runSubscription compares a subscription plan vs pay-as-you-go for a given monthly token volume.
// Usage:
//
//	costlint subscription --plan claude_max_5 --model sonnet --tokens 10000000
//	costlint subscription --plan all --model sonnet --tokens 10000000
func runSubscription() {
	plan := "all"
	model := "sonnet"
	var tokens int64

	args := os.Args[2:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--plan":
			if i+1 < len(args) {
				i++
				plan = args[i]
			}
		case "--model":
			if i+1 < len(args) {
				i++
				model = args[i]
			}
		case "--tokens":
			if i+1 < len(args) {
				i++
				v, err := strconv.ParseInt(args[i], 10, 64)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Invalid --tokens value: %s\n", args[i])
					os.Exit(1)
				}
				tokens = v
			}
		}
	}

	if tokens <= 0 {
		fmt.Fprintf(os.Stderr, "Usage: costlint subscription --plan <plan|all> --model <model> --tokens <monthly_tokens>\n")
		fmt.Fprintf(os.Stderr, "  --plan    Subscription plan key (claude_max_5, claude_max_20) or 'all'\n")
		fmt.Fprintf(os.Stderr, "  --model   Model for pay-as-you-go pricing (haiku, sonnet, opus)\n")
		fmt.Fprintf(os.Stderr, "  --tokens  Estimated monthly token usage\n")
		os.Exit(1)
	}

	if plan == "all" {
		comparisons, err := pricing.CompareAllSubscriptions(model, tokens)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		out, _ := json.MarshalIndent(comparisons, "", "  ")
		fmt.Println(string(out))
		return
	}

	cmp, err := pricing.CompareSubscription(plan, model, tokens)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	out, _ := json.MarshalIndent(cmp, "", "  ")
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

// runAB reads prompts from stdin (one per line) and estimates cost for each
// on both models, then prints a comparison summary.
//
// Usage:
//
//	echo -e "prompt1\nprompt2" | costlint ab --name test1 --model-a sonnet --model-b haiku
func runAB() {
	name := "ab-test"
	modelA := "sonnet"
	modelB := "haiku"

	args := os.Args[2:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name":
			if i+1 < len(args) {
				i++
				name = args[i]
			}
		case "--model-a":
			if i+1 < len(args) {
				i++
				modelA = args[i]
			}
		case "--model-b":
			if i+1 < len(args) {
				i++
				modelB = args[i]
			}
		}
	}

	test := ab.NewABTest(name, modelA, modelB)

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		prompt := scanner.Text()
		if prompt == "" {
			continue
		}
		tc := counter.Count(prompt)
		test.AddResult(modelA, prompt, tc.Input)
		test.AddResult(modelB, prompt, tc.Input)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
		os.Exit(1)
	}

	summary := test.Summary()

	result := map[string]interface{}{
		"name":              test.Name,
		"model_a":           test.ModelA,
		"model_b":           test.ModelB,
		"sample_size":       test.SampleSize,
		"model_a_avg_cost":  summary.ModelAAvgCost,
		"model_b_avg_cost":  summary.ModelBAvgCost,
		"savings_pct":       summary.SavingsPct,
		"recommendation":    summary.Recommendation,
	}
	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))
}

// runPerf benchmarks all costlint operations on sample data and reports latency as JSON.
//
// Usage:
//
//	costlint perf
func runPerf() {
	const iterations = 1000

	// Sample data for benchmarks.
	sampleText := "The quick brown fox jumps over the lazy dog. " +
		"This is a sample prompt to benchmark token counting and pricing estimation. " +
		"It contains multiple sentences to produce a realistic token count."

	samplePrompts := []string{
		"explain recursion in Go",
		"explain recursion",
		"what is a goroutine in Go?",
		"how do goroutines work in Go?",
		"write a function to reverse a string in Go",
	}

	type result struct {
		Operation    string  `json:"operation"`
		Iterations   int     `json:"iterations"`
		AvgMs        float64 `json:"avg_ms"`
		MinMs        float64 `json:"min_ms"`
		MaxMs        float64 `json:"max_ms"`
		OpsPerSecond float64 `json:"ops_per_second"`
	}

	var results []result

	// Benchmark: token counting speed.
	countResult := perf.Benchmark(func() {
		counter.Count(sampleText)
	}, iterations)
	results = append(results, result{
		Operation:    "count",
		Iterations:   countResult.Iterations,
		AvgMs:        countResult.AvgMs,
		MinMs:        countResult.MinMs,
		MaxMs:        countResult.MaxMs,
		OpsPerSecond: countResult.OpsPerSecond,
	})

	// Benchmark: pricing estimation speed.
	tc := counter.Count(sampleText)
	outputEstimate := counter.EstimateOutput(tc.Input, "")
	estimateResult := perf.Benchmark(func() {
		pricing.Estimate("sonnet", tc.Input, outputEstimate)
	}, iterations)
	results = append(results, result{
		Operation:    "estimate",
		Iterations:   estimateResult.Iterations,
		AvgMs:        estimateResult.AvgMs,
		MinMs:        estimateResult.MinMs,
		MaxMs:        estimateResult.MaxMs,
		OpsPerSecond: estimateResult.OpsPerSecond,
	})

	// Benchmark: cache similarity check speed.
	cacheResult := perf.Benchmark(func() {
		sim := cache.NewCacheSimulator("sonnet")
		for _, p := range samplePrompts {
			sim.Add(p)
		}
	}, iterations/10) // fewer iterations; each run processes multiple prompts
	results = append(results, result{
		Operation:    "cache",
		Iterations:   cacheResult.Iterations,
		AvgMs:        cacheResult.AvgMs,
		MinMs:        cacheResult.MinMs,
		MaxMs:        cacheResult.MaxMs,
		OpsPerSecond: cacheResult.OpsPerSecond,
	})

	out, _ := json.MarshalIndent(results, "", "  ")
	fmt.Println(string(out))
}

// runCache reads prompts from stdin (one per line), simulates cache hit/miss using
// Jaccard similarity on word sets, and prints a summary report.
//
// Usage:
//
//	echo -e "explain recursion\nexplain recursion in Go" | costlint cache --model sonnet
func runCache() {
	model := "sonnet"
	args := os.Args[2:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--model":
			if i+1 < len(args) {
				i++
				model = args[i]
			}
		}
	}

	sim := cache.NewCacheSimulator(model)

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		prompt := scanner.Text()
		if prompt == "" {
			continue
		}
		sim.Add(prompt)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(cache.FormatCacheReport(sim.Metrics))
}

// defaultTelemetryPath returns the path to the shared telemetry log file.
func defaultTelemetryPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".costlint-telemetry.jsonl"
	}
	return home + "/.costlint-telemetry.jsonl"
}

// runTelemetry dispatches to ingest or summary sub-commands.
//
// Usage:
//
//	promptlint analyze ... | costlint telemetry ingest
//	costlint telemetry summary
func runTelemetry() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: costlint telemetry {ingest|summary}\n")
		fmt.Fprintf(os.Stderr, "  ingest   Read promptlint JSONL from stdin and append to ~/.costlint-telemetry.jsonl\n")
		fmt.Fprintf(os.Stderr, "  summary  Print aggregated metrics from ~/.costlint-telemetry.jsonl\n")
		os.Exit(1)
	}

	log := telemetry.NewTelemetryLog(defaultTelemetryPath())

	switch os.Args[2] {
	case "ingest":
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
			os.Exit(1)
		}
		n, err := log.Ingest(data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error ingesting telemetry: %v\n", err)
			os.Exit(1)
		}
		result := map[string]interface{}{
			"ingested": n,
			"log":      defaultTelemetryPath(),
		}
		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(out))

	case "summary":
		s, err := log.Summary()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading telemetry: %v\n", err)
			os.Exit(1)
		}
		out, _ := json.MarshalIndent(s, "", "  ")
		fmt.Println(string(out))

	default:
		fmt.Fprintf(os.Stderr, "Unknown telemetry sub-command: %s\n", os.Args[2])
		os.Exit(1)
	}
}

// defaultBudgetPath returns the path to the geniearchi daily budget JSON.
func defaultBudgetPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home + "/projects/geniearchi/daily-budget.json"
}

// runDaily generates a daily cost report for today from telemetry and budget data.
//
// Usage:
//
//	costlint daily
//	costlint daily --format json
//	costlint daily --format text
func runDaily() {
	format := "json"
	args := os.Args[2:]
	for i := 0; i < len(args); i++ {
		if args[i] == "--format" && i+1 < len(args) {
			i++
			format = args[i]
		}
	}

	report, err := daily.GenerateReport(defaultTelemetryPath(), defaultBudgetPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating daily report: %v\n", err)
		os.Exit(1)
	}

	switch format {
	case "text":
		fmt.Print(daily.FormatText(report))
	default:
		out, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(out))
	}
}
