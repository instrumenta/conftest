package policy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/open-policy-agent/conftest/output"
	"github.com/open-policy-agent/conftest/parser"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/version"
)

// Engine represents the policy engine.
type Engine struct {
	trace    bool
	modules  map[string]*ast.Module
	compiler *ast.Compiler
	store    storage.Store
	policies map[string]string
	docs     map[string]string
}

// Load returns an Engine after loading all of the specified policies.
func Load(ctx context.Context, policyPaths []string) (*Engine, error) {
	policies, err := loader.AllRegos(policyPaths)
	if err != nil {
		return nil, fmt.Errorf("load: %w", err)
	} else if len(policies.Modules) == 0 {
		return nil, fmt.Errorf("no policies found in %v", policyPaths)
	}

	compiler, err := policies.Compiler()
	if err != nil {
		return nil, fmt.Errorf("get compiler: %w", err)
	}

	policyContents := make(map[string]string)
	for path, module := range policies.ParsedModules() {
		path = filepath.Clean(path)
		path = filepath.ToSlash(path)

		policyContents[path] = module.String()
	}

	engine := Engine{
		modules:  policies.ParsedModules(),
		compiler: compiler,
		policies: policyContents,
	}

	return &engine, nil
}

// LoadWithData returns an Engine after loading all of the specified policies and data paths.
func LoadWithData(ctx context.Context, policyPaths []string, dataPaths []string) (*Engine, error) {
	engine, err := Load(ctx, policyPaths)
	if err != nil {
		return nil, fmt.Errorf("loading policies: %w", err)
	}

	// FilteredPaths will recursively find all file paths that contain a valid document
	// extension from the given list of data paths.
	allDocumentPaths, err := loader.FilteredPaths(dataPaths, func(abspath string, info os.FileInfo, depth int) bool {
		if info.IsDir() {
			return false
		}
		return !contains([]string{".yaml", ".yml", ".json"}, filepath.Ext(info.Name()))
	})
	if err != nil {
		return nil, fmt.Errorf("filter data paths: %w", err)
	}

	documents, err := loader.NewFileLoader().All(allDocumentPaths)
	if err != nil {
		return nil, fmt.Errorf("load documents: %w", err)
	}
	store, err := documents.Store()
	if err != nil {
		return nil, fmt.Errorf("get documents store: %w", err)
	}

	documentContents := make(map[string]string)
	for _, documentPath := range allDocumentPaths {
		contents, err := ioutil.ReadFile(documentPath)
		if err != nil {
			return nil, fmt.Errorf("read file: %w", err)
		}

		documentPath = filepath.Clean(documentPath)
		documentPath = filepath.ToSlash(documentPath)
		documentContents[documentPath] = string(contents)
	}

	engine.store = store
	engine.docs = documentContents

	return engine, nil
}

func (e *Engine) EnableTracing() {
	e.trace = true
}

// Check executes all of the loaded policies against the input and returns the results.
func (e *Engine) Check(ctx context.Context, configs map[string]interface{}, namespace string) ([]output.CheckResult, error) {
	var checkResults []output.CheckResult
	for path, config := range configs {

		// It is possible for a configuration to have multiple configurations. An example of this
		// are multi-document yaml files where a single filepath represents multiple configs.
		//
		// If the current configuration contains multiple configurations, evaluate each policy
		// independent from one another and aggregate the results under the same file name.
		if subconfigs, exist := config.([]interface{}); exist {

			checkResult := output.CheckResult{
				FileName:  path,
				Namespace: namespace,
			}
			for _, subconfig := range subconfigs {
				result, err := e.check(ctx, path, subconfig, namespace)
				if err != nil {
					return nil, fmt.Errorf("check: %w", err)
				}

				checkResult.Successes = checkResult.Successes + result.Successes
				checkResult.Failures = append(checkResult.Failures, result.Failures...)
				checkResult.Warnings = append(checkResult.Warnings, result.Warnings...)
				checkResult.Exceptions = append(checkResult.Exceptions, result.Exceptions...)
				checkResult.Excludes = append(checkResult.Excludes, result.Excludes...)
				checkResult.Queries = append(checkResult.Queries, result.Queries...)
			}
			checkResults = append(checkResults, checkResult)
			continue
		}

		checkResult, err := e.check(ctx, path, config, namespace)
		if err != nil {
			return nil, fmt.Errorf("check: %w", err)
		}

		checkResults = append(checkResults, checkResult)
	}

	return checkResults, nil
}

// CheckCombined combines the input and evaluates the policies against the combined result.
func (e *Engine) CheckCombined(ctx context.Context, configs map[string]interface{}, namespace string) (output.CheckResult, error) {
	combinedConfigs := parser.CombineConfigurations(configs)

	result, err := e.check(ctx, "Combined", combinedConfigs["Combined"], namespace)
	if err != nil {
		return output.CheckResult{}, fmt.Errorf("check: %w", err)
	}

	return result, nil
}

// Namespaces returns all of the namespaces in the engine.
func (e *Engine) Namespaces() []string {
	var namespaces []string
	for _, module := range e.Modules() {
		namespace := strings.Replace(module.Package.Path.String(), "data.", "", 1)
		if contains(namespaces, namespace) {
			continue
		}

		namespaces = append(namespaces, namespace)
	}

	return namespaces
}

// Documents returns all of the documents loaded into the engine.
// The result is a map where the key is the filepath of the document
// and its value is the raw contents of the loaded document.
func (e *Engine) Documents() map[string]string {
	return e.docs
}

// Policies returns all of the policies loaded into the engine.
// The result is a map where the key is the filepath of the policy
// and its value is the raw contents of the loaded policy.
func (e *Engine) Policies() map[string]string {
	return e.policies
}

// Compiler returns the compiler from the loaded policies.
func (e *Engine) Compiler() *ast.Compiler {
	return e.compiler
}

// Store returns the store from the loaded documents.
func (e *Engine) Store() storage.Store {
	return e.store
}

// Modules returns the modules from the loaded policies.
func (e *Engine) Modules() map[string]*ast.Module {
	return e.modules
}

// Runtime returns the runtime of the engine.
func (e *Engine) Runtime() *ast.Term {
	env := ast.NewObject()
	for _, pair := range os.Environ() {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 1 {
			env.Insert(ast.StringTerm(parts[0]), ast.NullTerm())
		} else if len(parts) > 1 {
			env.Insert(ast.StringTerm(parts[0]), ast.StringTerm(parts[1]))
		}
	}

	obj := ast.NewObject()
	obj.Insert(ast.StringTerm("env"), ast.NewTerm(env))
	obj.Insert(ast.StringTerm("version"), ast.StringTerm(version.Version))
	obj.Insert(ast.StringTerm("commit"), ast.StringTerm(version.Vcs))

	return ast.NewTerm(obj)
}

func (e *Engine) check(ctx context.Context, path string, config interface{}, namespace string) (output.CheckResult, error) {
	var rules []string
	var ruleCount int

	for _, module := range e.Modules() {
		currentNamespace := strings.Replace(module.Package.Path.String(), "data.", "", 1)
		if currentNamespace != namespace {
			continue
		}

		// When performing policy evaluation using Check, there are a few rules that are special (e.g. warn and deny).
		// In order to validate the inputs against the policies, these rules need to be identified and how often
		// they appear in the policies.
		for r := range module.Rules {
			currentRule := module.Rules[r].Head.Name.String()

			if !isFailure(currentRule) && !isWarning(currentRule) {
				continue
			}

			// When checking the policies we want a unique list of rules to evaluate them one by one, but we also want
			// to keep track of how many rules we will be evaluating so we can calculate the final result.
			//
			// For example, a policy can have two deny rules that both contain different bodies. In this case the list
			// of rules will only contain deny, but the rule count would be two.
			ruleCount++

			if !contains(rules, currentRule) {
				rules = append(rules, currentRule)
			}
		}
	}

	checkResult := output.CheckResult{
		FileName:  path,
		Namespace: namespace,
	}
	var successes int
	for _, rule := range rules {

		// When matching rules for exceptions, only the name of the rule
		// is queried, so the severity prefix must be removed.
		exceptionQuery := fmt.Sprintf("data.%s.exception[_][_] == %q", namespace, removeRulePrefix(rule))

		exceptionQueryResult, err := e.query(ctx, config, exceptionQuery)
		if err != nil {
			return output.CheckResult{}, fmt.Errorf("query exception: %w", err)
		}

		var exceptions []output.Result
		for _, exceptionResult := range exceptionQueryResult.Results {

			// When an exception is found, set the message of the exception
			// to the query that triggered the exception so that it is known
			// which exception was trigged.
			if exceptionResult.Passed() {
				exceptionResult.Message = exceptionQuery
				exceptions = append(exceptions, exceptionResult)
			}
		}

		ruleQuery := fmt.Sprintf("data.%s.%s", namespace, rule)
		ruleQueryResult, err := e.query(ctx, config, ruleQuery)
		if err != nil {
			return output.CheckResult{}, fmt.Errorf("query rule: %w", err)
		}

		var failures []output.Result
		var warnings []output.Result
		var excludes []output.Result
		for _, ruleResult := range ruleQueryResult.Results {

			// Exceptions have already been accounted for in the exception query so
			// we skip them here to avoid doubling the result.
			if len(exceptions) > 0 {
				continue
			}

			if ruleResult.Passed() {
				successes++
				continue
			}

			result, err := json.Marshal(ruleResult.Metadata)
			if err != nil {
				return output.CheckResult{}, fmt.Errorf("json marshal: %w", err)
			}

			// If we have a non-null metadata response, then we are eligible to exclude the policy.
			// Otherwise we can just skip & process the policy violation
			if string(result) != "null" {
				localExcludeQuery := fmt.Sprintf("data.%s.exclude_%s[_][_] = %s", namespace, removeRulePrefix(rule), result)
				localExcludeQueryResult, err := e.query(ctx, config, localExcludeQuery)
				if err != nil {
					return output.CheckResult{}, fmt.Errorf("query exception: %w", err)
				}

				// If the query was a failure, let's have a look & see if an exception was written for it.
				if len(localExcludeQueryResult.Results) > 0 {
					// append an exception & continue
					localExcludeResult := localExcludeQueryResult.Results[0]
					localExcludeResult.Message = localExcludeQuery
					excludes = append(excludes, localExcludeResult)
					continue
				}

			}
			if isFailure(rule) {
				failures = append(failures, ruleResult)
			} else {
				warnings = append(warnings, ruleResult)
			}
		}

		checkResult.Failures = append(checkResult.Failures, failures...)
		checkResult.Warnings = append(checkResult.Warnings, warnings...)
		checkResult.Exceptions = append(checkResult.Exceptions, exceptions...)
		checkResult.Excludes = append(checkResult.Excludes, excludes...)

		checkResult.Queries = append(checkResult.Queries, exceptionQueryResult)
		checkResult.Queries = append(checkResult.Queries, ruleQueryResult)
	}

	// Only a single success result is returned when a given rule succeeds, even if there are multiple occurrences
	// of that rule.
	//
	// In the event that the total number of results is less than the total number of rules, we can safely assume
	// that the difference were successful results.
	resultCount := len(checkResult.Failures) + len(checkResult.Warnings) + len(checkResult.Exceptions) + successes
	if resultCount < ruleCount {
		successes += ruleCount - resultCount
	}

	checkResult.Successes = successes
	return checkResult, nil
}

// query is a low-level method that returns the result of executing a single query against the input.
//
// Example queries could include:
// data.main.deny to query the deny rule in the main namespace
// data.main.warn to query the warn rule in the main namespace
func (e *Engine) query(ctx context.Context, input interface{}, query string) (output.QueryResult, error) {
	options := []func(r *rego.Rego){
		rego.Input(input),
		rego.Query(query),
		rego.Compiler(e.Compiler()),
		rego.Store(e.Store()),
		rego.Runtime(e.Runtime()),
		rego.Trace(e.trace),
	}

	regoInstance := rego.New(options...)
	resultSet, err := regoInstance.Eval(ctx)
	if err != nil {
		return output.QueryResult{}, fmt.Errorf("evaluating policy: %w", err)
	}

	// After the evaluation of the policy, the results of the trace (stdout) will be populated
	// for the query. Once populated, format the trace results into a human readable format.
	buf := new(bytes.Buffer)
	rego.PrintTrace(buf, regoInstance)

	var traces []string
	for _, line := range strings.Split(buf.String(), "\n") {
		if len(line) > 0 {
			traces = append(traces, line)
		}
	}

	var results []output.Result
	for _, result := range resultSet {
		for _, expression := range result.Expressions {

			// Rego rules that are intended for evaluation should return a slice of values.
			// For example, deny[msg] or violation[{"msg": msg}].
			//
			// When an expression does not have a slice of values, the expression did not
			// evaluate to true, and no message was returned.
			var expressionValues []interface{}
			if _, ok := expression.Value.([]interface{}); ok {
				expressionValues = expression.Value.([]interface{})
			}
			if len(expressionValues) == 0 {
				results = append(results, output.Result{})
				continue
			}

			for _, v := range expressionValues {
				switch val := v.(type) {

				// Policies that only return a single string (e.g. deny[msg])
				case string:
					result := output.Result{
						Message: val,
					}
					results = append(results, result)

				// Policies that return metadata (e.g. deny[{"msg": msg}])
				case map[string]interface{}:
					result, err := output.NewResult(val)
					if err != nil {
						return output.QueryResult{}, fmt.Errorf("new result: %w", err)
					}

					results = append(results, result)
				}
			}
		}
	}

	queryResult := output.QueryResult{
		Query:   query,
		Results: results,
		Traces:  traces,
	}

	return queryResult, nil
}

func isWarning(rule string) bool {
	warningRegex := regexp.MustCompile("^warn(_[a-zA-Z0-9]+)*$")
	return warningRegex.MatchString(rule)
}

func isFailure(rule string) bool {
	failureRegex := regexp.MustCompile("^(deny|violation)(_[a-zA-Z0-9]+)*$")
	return failureRegex.MatchString(rule)
}

func contains(collection []string, item string) bool {
	for _, value := range collection {
		if strings.EqualFold(value, item) {
			return true
		}
	}

	return false
}

func removeRulePrefix(rule string) string {
	rule = strings.TrimPrefix(rule, "violation_")
	rule = strings.TrimPrefix(rule, "deny_")
	rule = strings.TrimPrefix(rule, "warn_")

	return rule
}
