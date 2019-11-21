package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"

	"github.com/instrumenta/conftest/parser"
	"github.com/instrumenta/conftest/policy"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	denyQ                 = regexp.MustCompile("^(deny|violation)(_[a-zA-Z]+)*$")
	warnQ                 = regexp.MustCompile("^warn(_[a-zA-Z]+)*$")
	combineConfigFlagName = "combine"
)

// CheckResult describes the result of a conftest evaluation.
// warning and failure "errors" produced by rego should be considered separate
// from other classes of exceptions.
type CheckResult struct {
	FileName  string
	Warnings  []error
	Failures  []error
	Successes []error
}

// NewTestCommand creates a new test command
func NewTestCommand(ctx context.Context) *cobra.Command {
	cmd := cobra.Command{
		Use:   "test <file> [file...]",
		Short: "Test your configuration files using Open Policy Agent",
		Args:  cobra.MinimumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			flagNames := []string{"fail-on-warn", "update", combineConfigFlagName, "trace", "output", "input", "namespace"}
			for _, name := range flagNames {
				if err := viper.BindPFlag(name, cmd.Flags().Lookup(name)); err != nil {
					return fmt.Errorf("bind flag: %w", err)
				}
			}

			return nil
		},

		RunE: func(cmd *cobra.Command, fileList []string) error {
			out := GetOutputManager()

			// Remove any blank files from the array
			var nonBlankFileList []string
			for _, name := range fileList {
				if name != "" {
					nonBlankFileList = append(nonBlankFileList, name)
				}
			}

			if len(nonBlankFileList) < 1 {
				return fmt.Errorf("no file specified")
			}

			if viper.GetBool("update") {
				NewUpdateCommand(ctx).Run(cmd, nonBlankFileList)
			}

			policyPath := viper.GetString("policy")
			regoFiles, err := policy.ReadFiles(policyPath)
			if err != nil {
				return fmt.Errorf("read rego files: %w", err)
			}

			compiler, err := policy.BuildCompiler(regoFiles)
			if err != nil {
				return fmt.Errorf("build compiler: %w", err)
			}

			configurations, err := parser.GetConfigurations(ctx, viper.GetString("input"), nonBlankFileList)
			if err != nil {
				return fmt.Errorf("get configurations: %w", err)
			}

			namespace := viper.GetString("namespace")

			var failures int
			if viper.GetBool(combineConfigFlagName) {
				result, err := GetResult(ctx, namespace, configurations, compiler)
				if err != nil {
					return fmt.Errorf("get combined test result: %w", err)
				}

				if isResultFailure(result) {
					failures++
				}

				if err := out.Put("Combined", result); err != nil {
					return fmt.Errorf("writing combined error: %w", err)
				}
			} else {
				for fileName, config := range configurations {
					result, err := GetResult(ctx, namespace, config, compiler)
					if err != nil {
						return fmt.Errorf("get test result: %w", err)
					}

					if isResultFailure(result) {
						failures++
					}

					if err := out.Put(fileName, result); err != nil {
						return fmt.Errorf("writing error: %w", err)
					}
				}
			}

			if err := out.Flush(); err != nil {
				return fmt.Errorf("flushing output: %w", err)
			}

			if failures > 0 {
				os.Exit(1)
			}

			return nil
		},
	}

	cmd.Flags().BoolP("fail-on-warn", "", false, "return a non-zero exit code if only warnings are found")
	cmd.Flags().BoolP("update", "", false, "update any policies before running the tests")
	cmd.Flags().BoolP(combineConfigFlagName, "", false, "combine all given config files to be evaluated together")
	cmd.Flags().BoolP("trace", "", false, "enable more verbose trace output for rego queries")

	cmd.Flags().StringP("output", "o", "", fmt.Sprintf("output format for conftest results - valid options are: %s", ValidOutputs()))
	cmd.Flags().StringP("input", "i", "", fmt.Sprintf("input type for given source, especially useful when using conftest with stdin, valid options are: %s", parser.ValidInputs()))
	cmd.Flags().StringP("namespace", "", "main", "namespace in which to find deny and warn rules")

	return &cmd
}

// GetResult returns the result of testing the structured data against their policies
func GetResult(ctx context.Context, namespace string, input interface{}, compiler *ast.Compiler) (CheckResult, error) {
	warnings, err := runRules(ctx, namespace, input, warnQ, compiler)
	if err != nil {
		return CheckResult{}, err
	}

	failures, err := runRules(ctx, namespace, input, denyQ, compiler)
	if err != nil {
		return CheckResult{}, err
	}

	result := CheckResult{
		Warnings: warnings,
		Failures: failures,
	}

	return result, nil
}

func isResultFailure(result CheckResult) bool {
	return len(result.Failures) > 0 || (len(result.Warnings) > 0 && viper.GetBool("fail-on-warn"))
}

func runRules(ctx context.Context, namespace string, input interface{}, regex *regexp.Regexp, compiler *ast.Compiler) ([]error, error) {
	var totalErrors []error
	var errors []error
	var err error

	rules := getRules(ctx, regex, compiler)
	for _, rule := range rules {

		query := fmt.Sprintf("data.%s.%s", namespace, rule)

		switch input.(type) {
		case []interface{}:
			errors, err = runMultipleQueries(ctx, query, input, compiler)
		default:
			errors, err = runQuery(ctx, query, input, compiler)
		}

		if err != nil {
			return nil, err
		}

		totalErrors = append(totalErrors, errors...)
	}

	return totalErrors, nil
}

func getRules(ctx context.Context, re *regexp.Regexp, compiler *ast.Compiler) []string {
	var rules []string
	for _, module := range compiler.Modules {
		for _, rule := range module.Rules {
			ruleName := rule.Head.Name.String()

			// the same rule names can be used multiple times, but
			// we only want to run the query and report results once
			if re.MatchString(ruleName) && !stringInSlice(ruleName, rules) {
				rules = append(rules, ruleName)
			}
		}
	}

	return rules
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}

	return false
}

func runMultipleQueries(ctx context.Context, query string, inputs interface{}, compiler *ast.Compiler) ([]error, error) {
	var totalViolations []error
	for _, input := range inputs.([]interface{}) {
		violations, err := runQuery(ctx, query, input, compiler)
		if err != nil {
			return nil, fmt.Errorf("run query: %w", err)
		}

		totalViolations = append(totalViolations, violations...)
	}

	return totalViolations, nil
}

func runQuery(ctx context.Context, query string, input interface{}, compiler *ast.Compiler) ([]error, error) {
	rego, stdout := buildRego(viper.GetBool("trace"), query, input, compiler)
	resultSet, err := rego.Eval(ctx)
	if err != nil {
		return nil, fmt.Errorf("evaluating policy: %w", err)
	}

	topdown.PrettyTrace(os.Stdout, *stdout)

	hasResults := func(expression interface{}) bool {
		if v, ok := expression.([]interface{}); ok {
			return len(v) > 0
		}

		return false
	}

	var errs []error
	for _, result := range resultSet {
		for _, expression := range result.Expressions {
			value := expression.Value

			if hasResults(value) {
				for _, v := range value.([]interface{}) {
					errs = append(errs, errors.New(v.(string)))
				}
			}
		}
	}

	return errs, nil
}

func buildRego(trace bool, query string, input interface{}, compiler *ast.Compiler) (*rego.Rego, *topdown.BufferTracer) {
	var regoObj *rego.Rego
	var regoFunc []func(r *rego.Rego)
	buf := topdown.NewBufferTracer()

	regoFunc = append(regoFunc, rego.Query(query), rego.Compiler(compiler), rego.Input(input))
	if trace {
		regoFunc = append(regoFunc, rego.Tracer(buf))
	}

	regoObj = rego.New(regoFunc...)

	return regoObj, buf
}
