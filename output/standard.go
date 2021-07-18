package output

import (
	"fmt"
	"io"
	"os"

	"github.com/logrusorgru/aurora"
	"github.com/open-policy-agent/opa/tester"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/topdown/lineage"
)

// Standard represents an Outputter that outputs
// results in a human readable format.
type Standard struct {
	Writer io.Writer

	// Tracing will render the trace results of the
	// queries when set to true.
	Tracing bool

	// NoColor will disable all coloring when
	// set to true.
	NoColor bool

	// SuppressExceptions will disable output for exceptions when set to true.
	SuppressExceptions bool

	// ShowSkipped whether to show skipped tests
	// in the output.
	ShowSkipped bool
}

// NewStandard creates a new Standard with the given writer.
func NewStandard(w io.Writer) *Standard {
	standard := Standard{
		Writer: w,
	}

	return &standard
}

// Output outputs the results.
func (s *Standard) Output(results []CheckResult) error {
	colorizer := aurora.NewAurora(true)
	if s.NoColor {
		colorizer = aurora.NewAurora(false)
	}

	if s.Tracing {
		s.outputTrace(results, colorizer)
		return nil
	}

	var totalFailures int
	var totalExceptions int
	var totalWarnings int
	var totalSuccesses int
	var totalSkipped int
	for _, result := range results {
		var indicator string
		var namespace string
		if result.FileName == "-" {
			indicator = "-"
		} else {
			indicator = fmt.Sprintf("- %s", result.FileName)
		}

		if result.Namespace == "-" {
			namespace = "-"
		} else {
			namespace = fmt.Sprintf("- %s -", result.Namespace)
		}

		totalPolicies := result.Successes + len(result.Warnings) + len(result.Failures) + len(result.Exceptions) + len(result.Skipped)
		if totalPolicies == 0 {
			fmt.Fprintln(s.Writer, colorizer.Colorize("?", aurora.WhiteFg), indicator, namespace, "no policies found")
			continue
		}

		for _, warning := range result.Warnings {
			fmt.Fprintln(s.Writer, colorizer.Colorize("WARN", aurora.YellowFg), indicator, namespace, warning.Message)
		}

		for _, failure := range result.Failures {
			fmt.Fprintln(s.Writer, colorizer.Colorize("FAIL", aurora.RedFg), indicator, namespace, failure.Message)
		}

		if !s.SuppressExceptions {
			for _, exception := range result.Exceptions {
				fmt.Fprintln(s.Writer, colorizer.Colorize("EXCP", aurora.CyanFg), indicator, namespace, exception.Message)
			}
		}

		totalFailures += len(result.Failures)
		totalExceptions += len(result.Exceptions)
		totalWarnings += len(result.Warnings)
		totalSkipped += len(result.Skipped)
		totalSuccesses += result.Successes
	}

	totalTests := totalFailures + totalExceptions + totalWarnings + totalSuccesses + totalSkipped

	var pluralSuffixTests string
	if totalTests != 1 {
		pluralSuffixTests = "s"
	}

	var pluralSuffixWarnings string
	if totalWarnings != 1 {
		pluralSuffixWarnings = "s"
	}

	var pluralSuffixFailures string
	if totalFailures != 1 {
		pluralSuffixFailures = "s"
	}

	var pluralSuffixExceptions string
	if totalExceptions != 1 {
		pluralSuffixExceptions = "s"
	}

	outputText := fmt.Sprintf("%v test%s, %v passed, %v warning%s, %v failure%s, %v exception%s",
		totalTests, pluralSuffixTests,
		totalSuccesses,
		totalWarnings, pluralSuffixWarnings,
		totalFailures, pluralSuffixFailures,
		totalExceptions, pluralSuffixExceptions,
	)

	if s.ShowSkipped {
		outputText += fmt.Sprintf(", %v skipped", totalSkipped)
	}

	var outputColor aurora.Color
	if totalFailures > 0 {
		outputColor = aurora.RedFg
	} else if totalWarnings > 0 {
		outputColor = aurora.YellowFg
	} else if totalExceptions > 0 {
		outputColor = aurora.CyanFg
	} else {
		outputColor = aurora.GreenFg
	}

	fmt.Fprintln(s.Writer)
	fmt.Fprintln(s.Writer, colorizer.Colorize(outputText, outputColor))
	return nil
}

func (s *Standard) outputTrace(results []CheckResult, colorizer aurora.Aurora) {
	for _, result := range results {
		for _, query := range result.Queries {
			var color aurora.Color
			if query.Passed() {
				color = aurora.GreenFg
			} else {
				color = aurora.RedFg
			}

			fmt.Fprintln(s.Writer, colorizer.Colorize("file: "+result.FileName+" | query: "+query.Query, color))

			for _, t := range query.Traces {
				fmt.Fprintln(s.Writer, colorizer.Colorize("TRAC ", aurora.BlueFg), "", t)
			}
		}
	}
}

// outputs results as a report - similar to OPA test output
func (s *Standard) Report(results []*tester.Result, flag string) error {
	reporter := tester.PrettyReporter{
		Verbose:     true,
		Output:      os.Stdout,
		FailureLine: true}

	dup := make(chan *tester.Result)

	go func() {
		defer close(dup)
		for i := 0; i < len(results); i++ {
			results[i].Trace = filterTrace(results[i].Trace, flag)
			dup <- results[i]
		}
	}()

	if err := reporter.Report(dup); err != nil {
		return fmt.Errorf("report results: %w", err)
	}
	return nil
}

// Filter traces - returns only failed traces
func filterTrace(trace []*topdown.Event, flag string) []*topdown.Event {
	if flag == "full" {
		return trace
	}
	ops := map[topdown.Op]struct{}{}

	if flag == "fails" {
		ops[topdown.FailOp] = struct{}{}
	}

	if flag == "notes" {
		ops[topdown.NoteOp] = struct{}{}
	}

	return lineage.Filter(trace, func(event *topdown.Event) bool {
		_, relevant := ops[event.Op]
		return relevant
	})
}
