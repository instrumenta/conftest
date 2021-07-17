package output

import "fmt"

// Result describes the result of a single rule evaluation.
type Result struct {
	Message  string                 `json:"msg"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// NewResult creates a new result. An error is returned if the
// metadata could not be successfully parsed.
func NewResult(metadata map[string]interface{}) (Result, error) {
	if _, ok := metadata["msg"]; !ok {
		return Result{}, fmt.Errorf("rule missing msg field: %v", metadata)
	}
	if _, ok := metadata["msg"].(string); !ok {
		return Result{}, fmt.Errorf("msg field must be string: %v", metadata)
	}

	result := Result{
		Message:  metadata["msg"].(string),
		Metadata: make(map[string]interface{}),
	}

	for k, v := range metadata {
		if k != "msg" {
			result.Metadata[k] = v
		}
	}

	return result, nil
}

// Passed returns true if the result did not fail a policy.
func (r Result) Passed() bool {
	return r.Message == ""
}

// QueryResult describes the result of evaluting a query.
type QueryResult struct {

	// Query is the fully qualified query that was used
	// to determine the result. Ex: (data.main.deny)
	Query string `json:"query"`

	// Results are the individual results of the query.
	// When querying data.main.deny, multiple deny rules can
	// exist, producing multiple results.
	Results []Result `json:"results"`

	// Traces represents a single trace of how the query was
	// evaluated. Each trace value is a trace line.
	Traces []string `json:"traces"`
}

// Passed returns true if all of the results in the query
// passed and no failures were found.
func (q QueryResult) Passed() bool {
	for _, result := range q.Results {
		if !result.Passed() {
			return false
		}
	}

	return true
}

// CheckResult describes the result of a conftest policy evaluation.
// Errors produced by rego should be considered separate
// from other classes of exceptions.
type CheckResult struct {
	FileName   string        `json:"filename"`
	Namespace  string        `json:"namespace"`
	Successes  int           `json:"successes"`
	Skipped    []Result      `json:"skipped,omitempty"`
	Warnings   []Result      `json:"warnings,omitempty"`
	Failures   []Result      `json:"failures,omitempty"`
	Exceptions []Result      `json:"exceptions,omitempty"`
	Excludes   []Result      `json:"excludes,omitempty"`
	Queries    []QueryResult `json:"queries,omitempty"`
}

// ExitCode returns the exit code that should be returned
// given all of the returned results.
func ExitCode(results []CheckResult) int {
	var hasFailure bool
	for _, result := range results {
		if len(result.Failures) > 0 {
			hasFailure = true
		}
	}

	if hasFailure {
		return 1
	}

	return 0
}

// ExitCodeFailOnWarn returns the exit code that should be returned
// given all of the returned results, and will consider warnings
// as failures.
func ExitCodeFailOnWarn(results []CheckResult) int {
	var hasFailure bool
	var hasWarning bool
	for _, result := range results {
		if len(result.Failures) > 0 {
			hasFailure = true
		}

		if len(result.Warnings) > 0 {
			hasWarning = true
		}
	}

	if hasFailure {
		return 2
	}

	if hasWarning {
		return 1
	}

	return 0
}
