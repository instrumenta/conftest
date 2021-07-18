package runner

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/open-policy-agent/conftest/output"
	"github.com/open-policy-agent/conftest/policy"
	"github.com/open-policy-agent/opa/tester"
	"github.com/open-policy-agent/opa/topdown"
)

// VerifyRunner is the runner for the Verify command, executing
// Rego policy unit-tests.
type VerifyRunner struct {
	Policy  []string
	Data    []string
	Output  string
	NoColor bool `mapstructure:"no-color"`
	Trace   bool
	Report  string
}

const (
	ReportFull  = "full"
	ReportNotes = "notes"
	ReportFails = "fails"
)

func (r *VerifyRunner) IsReportOptionOn() bool {
	return r.Report == ReportFull ||
		r.Report == ReportNotes ||
		r.Report == ReportFails

}

// Run executes the Rego tests for the given policies.
func (r *VerifyRunner) Run(ctx context.Context) ([]output.CheckResult, []*tester.Result, error) {
	engine, err := policy.LoadWithData(ctx, r.Policy, r.Data)
	if err != nil {
		return nil, nil, fmt.Errorf("load: %w", err)
	}

	// Traces should be enabled when Trace or Report options are on
	enableTracing := r.Trace || r.IsReportOptionOn()

	if enableTracing {
		engine.EnableTracing()
	}

	runner := tester.NewRunner().SetCompiler(engine.Compiler()).SetStore(engine.Store()).SetModules(engine.Modules()).EnableTracing(enableTracing).SetRuntime(engine.Runtime())
	ch, err := runner.RunTests(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("running tests: %w", err)
	}

	var results []output.CheckResult
	var rawResults []*tester.Result
	for result := range ch {
		if result.Error != nil {
			return nil, nil, fmt.Errorf("run test: %w", result.Error)
		}

		rawResults = append(rawResults, result)
		buf := new(bytes.Buffer)
		topdown.PrettyTrace(buf, result.Trace)
		var traces []string
		for _, line := range strings.Split(buf.String(), "\n") {
			if len(line) > 0 {
				traces = append(traces, line)
			}
		}

		var outputResult output.Result
		if result.Fail || result.Skip {
			outputResult.Message = result.Package + "." + result.Name
		}

		queryResult := output.QueryResult{
			Query:   result.Name,
			Results: []output.Result{outputResult},
			Traces:  traces,
		}

		checkResult := output.CheckResult{
			FileName: result.Location.File,
			Queries:  []output.QueryResult{queryResult},
		}
		if result.Fail {
			checkResult.Failures = []output.Result{outputResult}
		} else if result.Skip {
			checkResult.Skipped = []output.Result{outputResult}
		} else {
			checkResult.Successes++
		}

		results = append(results, checkResult)
	}

	return results, rawResults, nil
}
