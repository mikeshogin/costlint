package daily

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mshogin/costlint/pkg/telemetry"
)

// DailyReport holds aggregated cost metrics for a single day.
type DailyReport struct {
	Date             string             `json:"date"`
	TotalCostUSD     float64            `json:"total_cost_usd"`
	TransactionCount int                `json:"transaction_count"`
	ByType           map[string]float64 `json:"by_type"`
	ByModel          map[string]float64 `json:"by_model"`
	Trend            float64            `json:"trend_vs_yesterday_pct"`
	Anomaly          bool               `json:"anomaly"`
	AnomalyReason    string             `json:"anomaly_reason,omitempty"`
}

// BudgetRecord is the shape of the daily-budget.json file used by geniearchi.
type BudgetRecord struct {
	Date       string             `json:"date"`
	TotalUSD   float64            `json:"total_usd"`
	ByAgent    map[string]float64 `json:"by_agent,omitempty"`
	Entries    []BudgetEntry      `json:"entries,omitempty"`
}

// BudgetEntry is a single entry within the budget JSON.
type BudgetEntry struct {
	Timestamp string  `json:"timestamp"`
	AgentType string  `json:"agent_type"`
	Model     string  `json:"model"`
	CostUSD   float64 `json:"cost_usd"`
}

// GenerateReport builds a DailyReport for today using telemetry JSONL and optional budget JSON.
// If budgetPath is empty or the file does not exist, only telemetry data is used.
func GenerateReport(telemetryPath string, budgetPath string) (DailyReport, error) {
	today := time.Now().UTC().Format("2006-01-02")
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")

	report := DailyReport{
		Date:    today,
		ByType:  make(map[string]float64),
		ByModel: make(map[string]float64),
	}

	// --- Telemetry data ---
	log := telemetry.NewTelemetryLog(telemetryPath)
	events, err := log.Load()
	if err != nil {
		return report, fmt.Errorf("load telemetry: %w", err)
	}

	var todayCost, yesterdayCost float64
	var todayCount, yesterdayCount int

	for _, ev := range events {
		date := eventDate(ev.Timestamp)
		switch date {
		case today:
			report.TotalCostUSD += ev.CostUSD
			report.TransactionCount++
			todayCost += ev.CostUSD
			todayCount++
			if ev.Model != "" {
				report.ByModel[ev.Model] += ev.CostUSD
			}
			// Treat Complexity / Source as "type" when available.
			typ := ev.Complexity
			if typ == "" {
				typ = ev.Source
			}
			if typ == "" {
				typ = "unknown"
			}
			report.ByType[typ] += ev.CostUSD
		case yesterday:
			yesterdayCost += ev.CostUSD
			yesterdayCount++
		}
	}
	_ = todayCount
	_ = yesterdayCount

	// --- Budget JSON (optional, enriches by_type with agent breakdown) ---
	if budgetPath != "" {
		budgetData, berr := os.ReadFile(budgetPath)
		if berr == nil {
			var budgetRecords []BudgetRecord
			// Try array first, then single object.
			if jerr := json.Unmarshal(budgetData, &budgetRecords); jerr != nil {
				var single BudgetRecord
				if jerr2 := json.Unmarshal(budgetData, &single); jerr2 == nil {
					budgetRecords = []BudgetRecord{single}
				}
			}
			for _, br := range budgetRecords {
				if br.Date != today {
					continue
				}
				// Merge by_agent into by_type.
				for agent, cost := range br.ByAgent {
					report.ByType[agent] += cost
				}
				// Merge individual entries.
				for _, entry := range br.Entries {
					if entryDate := eventDate(entry.Timestamp); entryDate != today {
						continue
					}
					typ := entry.AgentType
					if typ == "" {
						typ = "unknown"
					}
					report.ByType[typ] += entry.CostUSD
					if entry.Model != "" {
						report.ByModel[entry.Model] += entry.CostUSD
					}
					report.TotalCostUSD += entry.CostUSD
					report.TransactionCount++
				}
			}
		}
	}

	// --- Trend ---
	if yesterdayCost > 0 {
		report.Trend = ((todayCost - yesterdayCost) / yesterdayCost) * 100
	}

	// --- Anomaly detection ---
	// Flag when today's cost exceeds 2x yesterday's cost.
	if yesterdayCost > 0 && todayCost > 2*yesterdayCost {
		report.Anomaly = true
		report.AnomalyReason = fmt.Sprintf(
			"today $%.4f > 2x yesterday $%.4f (%.0f%% increase)",
			todayCost, yesterdayCost, report.Trend,
		)
	}

	return report, nil
}

// FormatText returns a human-readable representation of DailyReport.
func FormatText(r DailyReport) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Daily Cost Report: %s\n", r.Date)
	fmt.Fprintf(&b, "  Total cost:   $%.4f\n", r.TotalCostUSD)
	fmt.Fprintf(&b, "  Transactions: %d\n", r.TransactionCount)

	if r.Trend != 0 {
		direction := "up"
		if r.Trend < 0 {
			direction = "down"
		}
		fmt.Fprintf(&b, "  Trend:        %s %.1f%% vs yesterday\n", direction, abs(r.Trend))
	} else {
		fmt.Fprintf(&b, "  Trend:        no previous data\n")
	}

	if r.Anomaly {
		fmt.Fprintf(&b, "\n  ANOMALY: %s\n", r.AnomalyReason)
	}

	if len(r.ByModel) > 0 {
		fmt.Fprintf(&b, "\n  By model:\n")
		for model, cost := range r.ByModel {
			fmt.Fprintf(&b, "    %-12s $%.4f\n", model, cost)
		}
	}

	if len(r.ByType) > 0 {
		fmt.Fprintf(&b, "\n  By type:\n")
		for typ, cost := range r.ByType {
			fmt.Fprintf(&b, "    %-12s $%.4f\n", typ, cost)
		}
	}

	return b.String()
}

// eventDate extracts the YYYY-MM-DD part from an RFC3339 timestamp string.
// Returns the raw string if parsing fails.
func eventDate(ts string) string {
	if len(ts) >= 10 {
		return ts[:10]
	}
	return ts
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
