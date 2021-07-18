package output

import (
	"os"

	"github.com/open-policy-agent/opa/tester"
)

// Outputter controls how results of an evaluation will
// be recorded and reported to the end user.
type Outputter interface {
	Output([]CheckResult) error
	Report([]*tester.Result, string) error
}

// Options represents the options available when configuring
// an Outputter.
type Options struct {
	Tracing            bool
	NoColor            bool
	SuppressExceptions bool
	ShowSkipped        bool
}

// The defined output formats represent all of the supported formats
// that can be used to format and render results.
const (
	OutputStandard = "stdout"
	OutputJSON     = "json"
	OutputTAP      = "tap"
	OutputTable    = "table"
	OutputJUnit    = "junit"
)

// Get returns a type that can render output in the given format.
func Get(format string, options Options) Outputter {
	switch format {
	case OutputStandard:
		return &Standard{Writer: os.Stdout, NoColor: options.NoColor, SuppressExceptions: options.SuppressExceptions, Tracing: options.Tracing, ShowSkipped: options.ShowSkipped}
	case OutputJSON:
		return NewJSON(os.Stdout)
	case OutputTAP:
		return NewTAP(os.Stdout)
	case OutputTable:
		return NewTable(os.Stdout)
	case OutputJUnit:
		return NewJUnit(os.Stdout)
	default:
		return NewStandard(os.Stdout)
	}
}

// Outputs returns the available output formats.
func Outputs() []string {
	return []string{
		OutputStandard,
		OutputJSON,
		OutputTAP,
		OutputTable,
		OutputJUnit,
	}
}
