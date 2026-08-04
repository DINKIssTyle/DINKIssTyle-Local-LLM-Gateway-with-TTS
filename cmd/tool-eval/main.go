package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dinkisstyle-chat/internal/evalharness"
	"dinkisstyle-chat/internal/toolruntime"
)

func main() {
	envFile := flag.String("env-file", ".env.eval.local", "local environment file (never written to reports)")
	scenarioFile := flag.String("scenarios", "testbed/scenarios.json", "scenario definition file")
	scenarioID := flag.String("scenario", "latest_migration_news", "scenario id to run")
	scenarioPrefix := flag.String("scenario-prefix", "", "run every scenario whose id starts with this prefix")
	runAll := flag.Bool("all", false, "run every scenario")
	list := flag.Bool("list", false, "list scenario ids without calling the LLM")
	searchQuery := flag.String("search", "", "run search_web directly without calling the LLM")
	outputDir := flag.String("out", ".eval-results", "report output directory")
	flag.Parse()
	if strings.TrimSpace(*searchQuery) != "" {
		runDirectSearch(*searchQuery)
		return
	}

	scenarios, err := evalharness.LoadScenarios(*scenarioFile)
	if err != nil {
		fatal(err)
	}
	if *list {
		for _, scenario := range scenarios {
			fmt.Printf("%s\t%s\n", scenario.ID, scenario.Prompt)
		}
		return
	}
	config, err := evalharness.LoadConfig(*envFile)
	if err != nil {
		fatal(err)
	}

	selected := make([]evalharness.Scenario, 0, len(scenarios))
	for _, scenario := range scenarios {
		prefixMatch := strings.TrimSpace(*scenarioPrefix) != "" && strings.HasPrefix(scenario.ID, strings.TrimSpace(*scenarioPrefix))
		if *runAll || prefixMatch || (strings.TrimSpace(*scenarioPrefix) == "" && scenario.ID == *scenarioID) {
			selected = append(selected, scenario)
		}
	}
	if len(selected) == 0 {
		if strings.TrimSpace(*scenarioPrefix) != "" {
			fatal(fmt.Errorf("scenario prefix %q matched nothing", *scenarioPrefix))
		}
		fatal(fmt.Errorf("scenario %q not found", *scenarioID))
	}

	runner := evalharness.Runner{Config: config}
	overallPassed := true
	for _, scenario := range selected {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		report := runner.Run(ctx, scenario)
		cancel()
		stamp := time.Now().Format("20060102-150405")
		path := filepath.Join(*outputDir, stamp+"-"+safeName(scenario.ID)+".json")
		if err := evalharness.WriteReport(path, report); err != nil {
			fatal(err)
		}
		status := "PASS"
		if !report.Passed {
			status = "FAIL"
			overallPassed = false
		}
		fmt.Printf("[%s] %s | skills=%s rounds=%d tools=%d sources=%d tokens=%d request_bytes=%d corrections=%d | %s\n",
			status, scenario.ID, strings.Join(report.SelectedSkills, ","), report.LLMRounds, len(report.ToolTrace), len(report.Sources), report.Usage.TotalTokens,
			totalRequestBytes(report.RoundMetrics), report.ProtocolCorrections, path)
		for _, check := range report.Checks {
			checkStatus := "ok"
			if !check.Passed {
				checkStatus = "failed"
			}
			fmt.Printf("  - %s: %s (%s)\n", check.Name, checkStatus, check.Details)
		}
	}
	if !overallPassed {
		os.Exit(1)
	}
}

func totalRequestBytes(rounds []evalharness.RoundMetric) int {
	total := 0
	for _, round := range rounds {
		total += round.RequestBytes
	}
	return total
}

func runDirectSearch(query string) {
	arguments, err := json.Marshal(map[string]string{"query": strings.TrimSpace(query)})
	if err != nil {
		fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	result, err := toolruntime.Default.Call(ctx, toolruntime.ExecutionContext{
		RequestID: "tool-eval-direct-search",
		UserID:    "tool-eval",
	}, "search_web", arguments)
	if strings.TrimSpace(result.Content) != "" {
		fmt.Println(result.Content)
	}
	if err != nil {
		fatal(err)
	}
}

func safeName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(value)
	if value == "" {
		return "scenario"
	}
	return value
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "tool-eval:", err)
	os.Exit(2)
}
