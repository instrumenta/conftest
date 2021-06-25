package runner

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
)

func TestTrace(t *testing.T) {
	cases := []struct {
		PolicyPath          []string
		ExplainMode         string
		Trace               bool
		ExpFailures         int
		ExpectedTraceOutput string
		Description         string
	}{
		{
			PolicyPath:          []string{"fixtures/verify_with_tracing/main.rego"},
			ExplainMode:         "fails",
			ExpFailures:         1,
			Description:         "explain_mode_fails",
			ExpectedTraceOutput: "fixtures/verify_with_tracing/testdata/expected_fails_trace.txt",
		},
		{
			PolicyPath:          []string{"fixtures/verify_with_tracing/main.rego"},
			ExplainMode:         "notes",
			ExpFailures:         1,
			Description:         "explain_mode_notes",
			ExpectedTraceOutput: "fixtures/verify_with_tracing/testdata/expected_notes_trace.txt",
		},
		{
			PolicyPath:          []string{"fixtures/verify_with_tracing/main.rego"},
			ExplainMode:         "full",
			ExpFailures:         1,
			Description:         "explain_mode_full",
			ExpectedTraceOutput: "fixtures/verify_with_tracing/testdata/expected_full_trace.txt",
		},
		{
			PolicyPath:          []string{"fixtures/verify_with_tracing/main.rego"},
			ExplainMode:         "not-supported",
			ExpFailures:         1,
			Description:         "no_trace",
			ExpectedTraceOutput: "fixtures/verify_with_tracing/testdata/expected_no_trace.txt",
		},
		{
			PolicyPath:          []string{"fixtures/verify_with_tracing/main.rego"},
			Trace:               true,
			ExpFailures:         1,
			Description:         "trace_enabled",
			ExpectedTraceOutput: "fixtures/verify_with_tracing/testdata/expected_full_trace.txt",
		},
	}

	for _, c := range cases {
		t.Run(c.Description, func(t *testing.T) {
			ctx := context.Background()
			r := VerifyRunner{
				Policy:       c.PolicyPath,
				Trace:        c.Trace,
				ExplainQuery: c.ExplainMode,
			}

			outputs, err := r.Run(ctx)
			if err != nil {
				t.Fatalf("running verify runner: %v", err)
			}

			expTrace, err := ioutil.ReadFile(c.ExpectedTraceOutput)
			if err != nil {
				t.Fatalf("reading ExpectedTraceOutput: %s", err)
			}

			result := outputs[0]
			if len(result.Failures) != c.ExpFailures {
				t.Errorf("Got %v failures, expected %v", len(result.Failures), c.ExpFailures)
			}

			query := result.Queries[0]
			actTrace := strings.Join(query.Traces, "\n")

			// Remove newline in the expected fixtures
			if actTrace != strings.TrimSuffix(string(expTrace), "\n") {
				actualTraceFile := fmt.Sprintf("%s.%s", c.ExpectedTraceOutput, "act")
				err := ioutil.WriteFile(actualTraceFile, []byte(actTrace), 0600)
				if err != nil {
					t.Fatalf("writing file: %s", err)
				}

				t.Errorf("expected:\n%s\ngot:\n%s", expTrace, actTrace)
			}
		})
	}
}
