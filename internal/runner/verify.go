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
	"github.com/open-policy-agent/opa/topdown/lineage"
)

// VerifyRunner is the runner for the Verify command, executing
// Rego policy unit-tests.
type VerifyRunner struct {
	Policy       []string
	Data         []string
	Output       string
	NoColor      bool `mapstructure:"no-color"`
	Trace        bool
	ExplainQuery string `mapstructure:"explain"`
}

const (
	ExplainQueryFull  = "full"
	ExplainQueryNotes = "notes"
	ExplainQueryFails = "fails"
)

func (r *VerifyRunner) isExplainSet() bool {
	return r.ExplainQuery == ExplainQueryFails ||
		r.ExplainQuery == ExplainQueryFull ||
		r.ExplainQuery == ExplainQueryNotes

}

func (r *VerifyRunner) filterTrace(trace []*topdown.Event) []*topdown.Event {
	ops := map[topdown.Op]struct{}{}

	if r.ExplainQuery == ExplainQueryFull {
		return trace
	}

	if !r.isExplainSet() && r.Trace {
		return trace
	}

	// If an explain mode was specified, filter based
	// on the mode. If no explain mode was specified,
	// default to show both notes and fail events

	if r.ExplainQuery == ExplainQueryNotes {
		ops[topdown.NoteOp] = struct{}{}
	}

	if r.ExplainQuery == ExplainQueryFails {
		ops[topdown.FailOp] = struct{}{}
	}

	return lineage.Filter(trace, func(event *topdown.Event) bool {
		_, relevant := ops[event.Op]
		return relevant
	})
}

func (r *VerifyRunner) IsTraceEnabled() bool {
	return r.Trace || r.isExplainSet()
}

// Run executes the Rego tests for the given policies.
func (r *VerifyRunner) Run(ctx context.Context) ([]output.CheckResult, error) {
	engine, err := policy.LoadWithData(ctx, r.Policy, r.Data)
	if err != nil {
		return nil, fmt.Errorf("load: %w", err)
	}

	runner := tester.NewRunner().
		SetCompiler(engine.Compiler()).
		SetStore(engine.Store()).
		SetModules(engine.Modules()).
		EnableTracing(r.IsTraceEnabled()).
		SetRuntime(engine.Runtime())

	ch, err := runner.RunTests(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("running tests: %w", err)
	}

	var results []output.CheckResult
	for result := range ch {
		if result.Error != nil {
			return nil, fmt.Errorf("run test: %w", result.Error)
		}

		var traces []string
		buf := new(bytes.Buffer)

		// If explain flag is set only output trace for failed tests otherwise
		// output trace for all tests
		if result.Fail && r.isExplainSet() ||
			r.Trace {
			topdown.PrettyTraceWithLocation(buf, r.filterTrace(result.Trace))
		}

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

	return results, nil
}
