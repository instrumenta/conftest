package output

import (
	"fmt"
	"io"

	"github.com/open-policy-agent/opa/tester"
)

// TAP represents an Outputter that outputs
// results in TAP format.
type TAP struct {
	Writer io.Writer
}

// NewTAP creates a new TAP with the given writer.
func NewTAP(w io.Writer) *TAP {
	tap := TAP{
		Writer: w,
	}

	return &tap
}

// Output outputs the results.
func (t *TAP) Output(checkResults []CheckResult) error {
	for _, result := range checkResults {
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

		totalTests := result.Successes + len(result.Failures) + len(result.Warnings) + len(result.Exceptions) + len(result.Skipped)
		if totalTests == 0 {
			return nil
		}

		counter := 1
		fmt.Fprintf(t.Writer, "1..%d\n", totalTests)

		for _, failure := range result.Failures {
			fmt.Fprintf(t.Writer, "not ok %v %v %v %v\n", counter, indicator, namespace, failure.Message)
			counter++
		}

		if len(result.Warnings) > 0 {
			fmt.Fprintln(t.Writer, "# warnings")
			for _, warning := range result.Warnings {
				fmt.Fprintf(t.Writer, "not ok %v %v %v %v\n", counter, indicator, namespace, warning.Message)
				counter++
			}
		}

		if len(result.Exceptions) > 0 {
			fmt.Fprintln(t.Writer, "# exceptions")
			for _, exception := range result.Exceptions {
				fmt.Fprintf(t.Writer, "ok %v %v %v %v\n", counter, indicator, namespace, exception.Message)
				counter++
			}
		}

		if len(result.Skipped) > 0 {
			fmt.Fprintln(t.Writer, "# skip")
			for _, skipped := range result.Skipped {
				fmt.Fprintf(t.Writer, "ok %v %v %v %v\n", counter, indicator, namespace, skipped.Message)
				counter++
			}
		}

		if result.Successes > 0 {
			fmt.Fprintln(t.Writer, "# successes")
			for i := 0; i < result.Successes; i++ {
				fmt.Fprintf(t.Writer, "ok %v %v %v %v\n", counter, indicator, namespace, "SUCCESS")
				counter++
			}
		}
	}

	return nil
}

func (t *TAP) Report(results []*tester.Result, flag string) error {
	return fmt.Errorf("report is not supported in TAP output")
}
