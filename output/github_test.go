package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestGitHub(t *testing.T) {
	tests := []struct {
		name     string
		input    []CheckResult
		expected []string
	}{
		{
			name: "no warnings or errors",
			input: []CheckResult{
				{
					FileName:  "examples/kubernetes/service.yaml",
					Namespace: "namespace",
				},
			},
			expected: []string{
				"::group::Testing 'examples/kubernetes/service.yaml' against 0 policies in namespace 'namespace'",
				"::endgroup::",
				"0 tests, 0 passed, 0 warnings, 0 failures, 0 exceptions",
				"",
			},
		},
		{
			name: "records failure and warnings",
			input: []CheckResult{
				{
					FileName:  "examples/kubernetes/service.yaml",
					Namespace: "namespace",
					Warnings:  []Result{{Message: "first warning"}},
					Failures:  []Result{{Message: "first failure"}},
				},
			},
			expected: []string{
				"::group::Testing 'examples/kubernetes/service.yaml' against 2 policies in namespace 'namespace'",
				"::error file=examples/kubernetes/service.yaml::first failure",
				"::warning file=examples/kubernetes/service.yaml::first warning",
				"::endgroup::",
				"2 tests, 0 passed, 1 warning, 1 failure, 0 exceptions",
				"",
			},
		},
		{
			name: "mixed failure, warnings and skipped",
			input: []CheckResult{
				{
					FileName:  "examples/kubernetes/service.yaml",
					Namespace: "namespace",
					Failures:  []Result{{Message: "first failure"}},
					Skipped:   []Result{{Message: "first skipped"}},
				},
			},
			expected: []string{
				"::group::Testing 'examples/kubernetes/service.yaml' against 2 policies in namespace 'namespace'",
				"::error file=examples/kubernetes/service.yaml::first failure",
				"skipped file=examples/kubernetes/service.yaml first skipped",
				"::endgroup::",
				"2 tests, 0 passed, 0 warnings, 1 failure, 0 exceptions",
				"",
			},
		},
		{
			name: "handles stdin input",
			input: []CheckResult{
				{
					FileName:  "-",
					Namespace: "namespace",
					Failures:  []Result{{Message: "first failure"}},
				},
			},
			expected: []string{
				"::group::Testing '-' against 1 policies in namespace 'namespace'",
				"::error file=-::first failure",
				"::endgroup::",
				"1 test, 0 passed, 0 warnings, 1 failure, 0 exceptions",
				"",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expected := strings.Join(tt.expected, "\n")

			buf := new(bytes.Buffer)
			if err := NewGitHub(buf).Output(tt.input); err != nil {
				t.Fatal("output GitHub:", err)
			}

			actual := buf.String()

			if expected != actual {
				t.Errorf("unexpected output. expected %v actual %v", expected, actual)
			}
		})
	}
}
