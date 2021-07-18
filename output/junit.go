package output

import (
	"fmt"
	"io"
	"runtime"
	"strings"

	"github.com/jstemmer/go-junit-report/formatter"
	"github.com/jstemmer/go-junit-report/parser"
	"github.com/open-policy-agent/opa/tester"
)

// JUnit represents an Outputter that outputs
// results in JUnit format.
type JUnit struct {
	Writer io.Writer
}

// NewJUnit creates a new JUnit with the given writer.
func NewJUnit(w io.Writer) *JUnit {
	jUnit := JUnit{
		Writer: w,
	}

	return &jUnit
}

// Output outputs the results.
func (j *JUnit) Output(results []CheckResult) error {
	var tests []*parser.Test
	for _, result := range results {
		for _, warning := range result.Warnings {
			warningTest := parser.Test{
				Name:   getTestName(result.FileName, result.Namespace, warning.Message),
				Result: parser.FAIL,
				Output: []string{warning.Message},
			}

			tests = append(tests, &warningTest)
		}

		for _, failure := range result.Failures {
			failingTest := parser.Test{
				Name:   getTestName(result.FileName, result.Namespace, failure.Message),
				Result: parser.FAIL,
				Output: []string{failure.Message},
			}

			tests = append(tests, &failingTest)
		}

		for _, skipped := range result.Skipped {
			skippedTest := parser.Test{
				Name:   getTestName(result.FileName, result.Namespace, skipped.Message),
				Result: parser.SKIP,
				Output: []string{skipped.Message},
			}

			tests = append(tests, &skippedTest)
		}

		for s := 0; s < result.Successes; s++ {
			successfulTest := parser.Test{
				Name:   getTestName(result.FileName, result.Namespace, ""),
				Result: parser.PASS,
				Output: []string{},
			}

			tests = append(tests, &successfulTest)
		}
	}

	report := parser.Report{
		Packages: []parser.Package{
			{
				Name:  "conftest",
				Tests: tests,
			},
		},
	}

	if err := formatter.JUnitReportXML(&report, false, runtime.Version(), j.Writer); err != nil {
		return fmt.Errorf("format junit: %w", err)
	}

	return nil
}

func getTestName(fileName string, namespace string, message string) string {
	if len(message) > 0 {
		return fmt.Sprintf("%s - %s - %s", fileName, namespace, strings.Split(message, "\n")[0])
	}

	return fmt.Sprintf("%s - %s", fileName, namespace)
}

func (j *JUnit) Report(results []*tester.Result, flag string) error {
	return fmt.Errorf("report is not supported in JUnit output")
}
