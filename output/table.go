package output

import (
	"fmt"
	"io"

	"github.com/olekukonko/tablewriter"
	"github.com/open-policy-agent/opa/tester"
)

// Table represents an Outputter that outputs
// results in a tabular format.
type Table struct {
	Writer io.Writer
}

// NewTable creates a new Table with the given writer.
func NewTable(w io.Writer) *Table {
	table := Table{
		Writer: w,
	}

	return &table
}

// Output outputs the results.
func (t *Table) Output(checkResults []CheckResult) error {
	table := tablewriter.NewWriter(t.Writer)
	table.SetHeader([]string{"result", "file", "namespace", "message"})

	var tableData [][]string
	for _, checkResult := range checkResults {
		for r := 0; r < checkResult.Successes; r++ {
			tableData = append(tableData, []string{"success", checkResult.FileName, checkResult.Namespace, "SUCCESS"})
		}

		for _, result := range checkResult.Exceptions {
			tableData = append(tableData, []string{"exception", checkResult.FileName, checkResult.Namespace, result.Message})
		}

		for _, result := range checkResult.Warnings {
			tableData = append(tableData, []string{"warning", checkResult.FileName, checkResult.Namespace, result.Message})
		}

		for _, result := range checkResult.Skipped {
			tableData = append(tableData, []string{"skipped", checkResult.FileName, checkResult.Namespace, result.Message})
		}

		for _, result := range checkResult.Failures {
			tableData = append(tableData, []string{"failure", checkResult.FileName, checkResult.Namespace, result.Message})
		}
	}

	if len(tableData) > 0 {
		table.AppendBulk(tableData)
		table.Render()
	}

	return nil
}

func (t *Table) Report(results []*tester.Result, flag string) error {
	return fmt.Errorf("report is not supported in table output")
}
