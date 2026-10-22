// Copyright The Conforma Contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package playground

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
	"sigs.k8s.io/yaml"
)

type Report struct {
	Success      bool           `json:"success"`
	SuccessCount int            `json:"successCount,omitempty"`
	Engine       string         `json:"engine"`
	Package      string         `json:"package,omitempty"`
	Modules      []string       `json:"modules,omitempty"`
	Rules        []RuleMetadata `json:"rules,omitempty"`
	Violations   []Result       `json:"violations,omitempty"`
	Warnings     []Result       `json:"warnings,omitempty"`
	Errors       []Error        `json:"errors,omitempty"`
}

type Result struct {
	Rule    string `json:"rule"`
	Message string `json:"msg"`
	Value   any    `json:"value,omitempty"`
}

type RuleMetadata struct {
	Name  string `json:"name"`
	Kind  string `json:"kind"`
	Code  string `json:"code,omitempty"`
	Title string `json:"title,omitempty"`
}

type Error struct {
	Stage   string `json:"stage"`
	Message string `json:"message"`
}

type playgroundModule struct {
	Filename string
	Source   string
	Package  string
	AST      *ast.Module
}

func Evaluate(ctx context.Context, policyText string, inputText string) Report {
	return EvaluateWithData(ctx, policyText, inputText, "")
}

func EvaluateWithData(ctx context.Context, policyText string, inputText string, dataText string) Report {
	report := Report{
		Engine: "opa",
	}

	input, err := parseDocument(inputText)
	if err != nil {
		report.Errors = append(report.Errors, Error{Stage: "input", Message: err.Error()})
		return report
	}

	data, err := parseOptionalDocument(dataText)
	if err != nil {
		report.Errors = append(report.Errors, Error{Stage: "data", Message: err.Error()})
		return report
	}

	modules, err := parsePolicyModules(policyText)
	if err != nil {
		report.Errors = append(report.Errors, Error{Stage: "rego", Message: err.Error()})
		return report
	}

	pkg := modules[0].Package
	report.Package = pkg
	for _, module := range modules {
		report.Modules = append(report.Modules, module.Filename)
	}

	rules, err := collectRuleMetadata(modules, pkg)
	if err != nil {
		report.Errors = append(report.Errors, Error{Stage: "metadata", Message: err.Error()})
		return report
	}
	report.Rules = rules

	for _, rule := range []string{"deny", "violation"} {
		values, err := evalRule(ctx, modules, input, data, pkg, rule)
		if err != nil {
			report.Errors = append(report.Errors, Error{Stage: rule, Message: err.Error()})
			continue
		}
		report.Violations = append(report.Violations, normalizeResults(rule, values)...)
	}

	values, err := evalRule(ctx, modules, input, data, pkg, "warn")
	if err != nil {
		report.Errors = append(report.Errors, Error{Stage: "warn", Message: err.Error()})
	} else {
		report.Warnings = append(report.Warnings, normalizeResults("warn", values)...)
	}

	report.Success = len(report.Errors) == 0 && len(report.Violations) == 0
	report.SuccessCount = countSuccessfulRules(report.Rules, report.Violations, report.Warnings)
	return report
}

func parseDocument(inputText string) (any, error) {
	inputText = strings.TrimSpace(inputText)
	if inputText == "" {
		return map[string]any{}, nil
	}

	var input any
	if err := json.Unmarshal([]byte(inputText), &input); err == nil {
		return input, nil
	}

	jsonBytes, err := yaml.YAMLToJSON([]byte(inputText))
	if err != nil {
		return nil, fmt.Errorf("document is not valid JSON or YAML: %w", err)
	}
	if err := json.Unmarshal(jsonBytes, &input); err != nil {
		return nil, fmt.Errorf("document is not valid JSON or YAML: %w", err)
	}
	return input, nil
}

func parseOptionalDocument(dataText string) (map[string]any, error) {
	if strings.TrimSpace(dataText) == "" {
		return nil, nil
	}
	data, err := parseDocument(dataText)
	if err != nil {
		return nil, err
	}
	dataMap, ok := data.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("data must be a JSON or YAML object")
	}
	return dataMap, nil
}

func parsePolicyModules(policyText string) ([]playgroundModule, error) {
	var modules []playgroundModule
	var current strings.Builder
	currentName := "playground.rego"

	flush := func() {
		source := strings.TrimSpace(current.String())
		if source == "" {
			return
		}
		modules = append(modules, playgroundModule{
			Filename: currentName,
			Source:   source,
		})
		current.Reset()
	}

	for _, line := range strings.Split(policyText, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# file:") {
			flush()
			currentName = strings.TrimSpace(strings.TrimPrefix(trimmed, "# file:"))
			if currentName == "" {
				return nil, fmt.Errorf("empty module filename in # file marker")
			}
			continue
		}
		current.WriteString(line)
		current.WriteByte('\n')
	}
	flush()

	if len(modules) == 0 {
		return nil, fmt.Errorf("no Rego modules provided")
	}

	for i := range modules {
		module, err := ast.ParseModuleWithOpts(modules[i].Filename, modules[i].Source, ast.ParserOptions{
			ProcessAnnotation: true,
		})
		if err != nil {
			return nil, err
		}
		modules[i].Package = strings.TrimPrefix(module.Package.Path.String(), "data.")
		modules[i].AST = module
	}

	return modules, nil
}

func collectRuleMetadata(modules []playgroundModule, pkg string) ([]RuleMetadata, error) {
	parsedModules := make([]*ast.Module, 0, len(modules))
	for _, module := range modules {
		parsedModules = append(parsedModules, module.AST)
	}

	annotations, errs := ast.BuildAnnotationSet(parsedModules)
	if len(errs) > 0 {
		return nil, errs
	}

	var rules []RuleMetadata
	for _, module := range modules {
		if module.Package != pkg {
			continue
		}
		for _, rule := range module.AST.Rules {
			name := rule.Head.Name.String()
			kind, ok := resultKind(name)
			if !ok {
				continue
			}

			metadata := RuleMetadata{
				Name: name,
				Kind: kind,
			}
			applyRuleAnnotation(&metadata, pkg, annotations, rule)
			rules = append(rules, metadata)
		}
	}
	return rules, nil
}

func resultKind(rule string) (string, bool) {
	switch {
	case rule == "deny", rule == "violation", strings.HasPrefix(rule, "deny_"), strings.HasPrefix(rule, "violation_"):
		return "violation", true
	case rule == "warn", strings.HasPrefix(rule, "warn_"):
		return "warning", true
	default:
		return "", false
	}
}

func applyRuleAnnotation(metadata *RuleMetadata, pkg string, annotations *ast.AnnotationSet, rule *ast.Rule) {
	if annotations == nil {
		return
	}

	for _, ref := range annotations.Chain(rule) {
		if ref.GetRule() == nil || ref.Annotations == nil {
			continue
		}

		if metadata.Title == "" {
			metadata.Title = ref.Annotations.Title
		}
		if metadata.Code == "" {
			metadata.Code = ruleCode(pkg, ref.Annotations.Custom)
		}
		if metadata.Code != "" && metadata.Title != "" {
			return
		}
	}
}

func ruleCode(pkg string, custom map[string]any) string {
	shortName, ok := custom["short_name"].(string)
	if !ok || shortName == "" {
		return ""
	}
	if strings.Contains(shortName, ".") {
		return shortName
	}
	return pkg + "." + shortName
}

func countSuccessfulRules(rules []RuleMetadata, violations []Result, warnings []Result) int {
	count := len(rules) - len(violations) - len(warnings)
	if count < 0 {
		return 0
	}
	return count
}

func evalRule(ctx context.Context, modules []playgroundModule, input any, data map[string]any, pkg string, rule string) ([]any, error) {
	query := fmt.Sprintf("data.%s.%s", pkg, rule)
	opts := []func(*rego.Rego){
		rego.Query(query),
		rego.Input(input),
	}
	for _, module := range modules {
		opts = append(opts, rego.Module(module.Filename, module.Source))
	}
	if data != nil {
		opts = append(opts, rego.Store(inmem.NewFromObject(data)))
	}

	resultSet, err := rego.New(opts...).Eval(ctx)
	if err != nil {
		return nil, err
	}

	values := make([]any, 0, len(resultSet))
	for _, result := range resultSet {
		for _, expr := range result.Expressions {
			values = append(values, expr.Value)
		}
	}
	return values, nil
}

func normalizeResults(rule string, values []any) []Result {
	var results []Result
	for _, value := range values {
		results = appendValue(results, rule, value)
	}
	return results
}

func appendValue(results []Result, rule string, value any) []Result {
	switch v := value.(type) {
	case nil:
		return results
	case string:
		return append(results, Result{Rule: rule, Message: v})
	case bool:
		if v {
			return append(results, Result{Rule: rule, Message: fmt.Sprintf("%s matched", rule)})
		}
	case []any:
		for _, item := range v {
			results = appendValue(results, rule, item)
		}
	case map[string]any:
		results = appendMapValues(results, rule, v)
	default:
		return append(results, Result{Rule: rule, Message: fmt.Sprint(v), Value: value})
	}
	return results
}

func appendMapValues(results []Result, rule string, values map[string]any) []Result {
	if msg, ok := values["msg"].(string); ok {
		return append(results, Result{Rule: rule, Message: msg, Value: values})
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := values[key]
		if matched, ok := value.(bool); ok && matched {
			results = append(results, Result{Rule: rule, Message: key})
			continue
		}
		results = append(results, Result{
			Rule:    rule,
			Message: fmt.Sprintf("%s: %v", key, value),
			Value:   map[string]any{key: value},
		})
	}
	return results
}

func MarshalReport(report Report) string {
	out, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fallback, _ := json.Marshal(Report{
			Engine: "opa",
			Errors: []Error{{
				Stage:   "marshal",
				Message: err.Error(),
			}},
		})
		return string(fallback)
	}
	return string(out)
}
