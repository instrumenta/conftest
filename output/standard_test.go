package output

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestStandard(t *testing.T) {
	tests := []struct {
		name        string
		input       []CheckResult
		expected    []string
		showSkipped bool
	}{
		{
			name: "records failures, warnings and skipped",
			input: []CheckResult{
				{
					FileName:  "foo.yaml",
					Namespace: "namespace",
					Warnings:  []Result{{Message: "first warning"}},
					Failures:  []Result{{Message: "first failure"}},
				},
			},
			expected: []string{
				"WARN - foo.yaml - namespace - first warning",
				"FAIL - foo.yaml - namespace - first failure",
				"",
				"2 tests, 0 passed, 1 warning, 1 failure, 0 exceptions, 0 exclusions",
				"",
			},
		},
		{
			name: "skips filenames for stdin",
			input: []CheckResult{
				{
					FileName:  "-",
					Namespace: "namespace",
					Warnings:  []Result{{Message: "first warning"}},
					Failures:  []Result{{Message: "first failure"}},
				},
			},
			expected: []string{
				"WARN - - namespace - first warning",
				"FAIL - - namespace - first failure",
				"",
				"2 tests, 0 passed, 1 warning, 1 failure, 0 exceptions, 0 exclusions",
				"",
			},
		},
		{
			name: "records failures, warnings and skipped",
			input: []CheckResult{
				{
					FileName:  "foo.yaml",
					Namespace: "namespace",
					Warnings:  []Result{{Message: "first warning"}},
					Failures:  []Result{{Message: "first failure"}},
					Skipped:   []Result{{Message: "first skipped"}},
				},
			},
			showSkipped: true,
			expected: []string{
				"WARN - foo.yaml - namespace - first warning",
				"FAIL - foo.yaml - namespace - first failure",
				"",
				"3 tests, 0 passed, 1 warning, 1 failure, 0 exceptions, 0 exclusions, 1 skipped",
				"",
			},
		},
		{
			name: "skips filenames for stdin",
			input: []CheckResult{
				{
					FileName:  "-",
					Namespace: "namespace",
					Warnings:  []Result{{Message: "first warning"}},
					Failures:  []Result{{Message: "first failure"}},
				},
			},
			showSkipped: true,
			expected: []string{
				"WARN - - namespace - first warning",
				"FAIL - - namespace - first failure",
				"",
				"2 tests, 0 passed, 1 warning, 1 failure, 0 exceptions, 0 exclusions, 0 skipped",
				"",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expected := strings.Join(tt.expected, "\n")

			buf := new(bytes.Buffer)
			standard := Standard{Writer: buf, NoColor: true, ShowSkipped: tt.showSkipped}
			if err := standard.Output(tt.input); err != nil {
				t.Fatal("output standard:", err)
			}

			actual := buf.String()

			if !reflect.DeepEqual(expected, actual) {
				t.Errorf("Unexpected output. expected %v actual %v", expected, actual)
			}
		})
	}
}
