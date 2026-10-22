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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEvaluatePlaygroundViolation(t *testing.T) {
	policy := `package main

deny contains msg if {
	input.kind != "Deployment"
	msg := "kind must be Deployment"
}
`

	report := Evaluate(context.Background(), policy, `{"kind":"Service"}`)

	require.True(t, report.Success == false)
	require.Empty(t, report.Errors)
	require.Len(t, report.Violations, 1)
	require.Equal(t, "kind must be Deployment", report.Violations[0].Message)
	require.Equal(t, 0, report.SuccessCount)
}

func TestEvaluatePlaygroundSuccess(t *testing.T) {
	policy := `package main

deny contains msg if {
	input.kind != "Deployment"
	msg := "kind must be Deployment"
}
`

	report := Evaluate(context.Background(), policy, `kind: Deployment`)

	require.True(t, report.Success)
	require.Empty(t, report.Errors)
	require.Empty(t, report.Violations)
	require.Equal(t, 1, report.SuccessCount)
}

func TestEvaluatePlaygroundSyntaxError(t *testing.T) {
	report := Evaluate(context.Background(), `package main

deny contains msg if {
	input.kind =
}
`, `{"kind":"Service"}`)

	require.False(t, report.Success)
	require.NotEmpty(t, report.Errors)
	require.Equal(t, "rego", report.Errors[0].Stage)
}

func TestEvaluatePlaygroundWithHelperModuleAndData(t *testing.T) {
	policy := `# file: main.rego
package main

import data.lib

deny contains msg if {
	not lib.allowed_kind(input.kind)
	msg := sprintf("kind must be one of %v", [data.allowed_kinds])
}

# file: lib/kinds.rego
package lib

allowed_kind(kind) if {
	data.allowed_kinds[_] == kind
}
`

	report := EvaluateWithData(context.Background(), policy, `kind: Service`, `allowed_kinds: ["Deployment"]`)

	require.False(t, report.Success)
	require.Empty(t, report.Errors)
	require.Equal(t, []string{"main.rego", "lib/kinds.rego"}, report.Modules)
	require.Len(t, report.Violations, 1)
	require.Equal(t, "kind must be one of [\"Deployment\"]", report.Violations[0].Message)
	require.Equal(t, 0, report.SuccessCount)
}

func TestEvaluatePlaygroundWithHelperModuleAndDataSuccess(t *testing.T) {
	policy := `# file: main.rego
package main

import data.lib

deny contains msg if {
	not lib.allowed_kind(input.kind)
	msg := sprintf("kind must be one of %v", [data.allowed_kinds])
}

# file: lib/kinds.rego
package lib

allowed_kind(kind) if {
	data.allowed_kinds[_] == kind
}
`

	report := EvaluateWithData(context.Background(), policy, `kind: Deployment`, `allowed_kinds: ["Deployment"]`)

	require.True(t, report.Success)
	require.Empty(t, report.Errors)
	require.Empty(t, report.Violations)
	require.Equal(t, 1, report.SuccessCount)
}

func TestEvaluatePlaygroundConformaShapedFixture(t *testing.T) {
	policy := readFixture(t, "fixtures/conforma-shaped/policy.rego")
	input := readFixture(t, "fixtures/conforma-shaped/input-fail.yaml")
	data := readFixture(t, "fixtures/conforma-shaped/data.yaml")

	report := EvaluateWithData(context.Background(), policy, input, data)

	require.False(t, report.Success)
	require.Empty(t, report.Errors)
	require.Equal(t, []string{"main.rego", "lib/pipeline.rego", "lib/release.rego"}, report.Modules)
	require.Len(t, report.Rules, 3)
	require.Equal(t, 0, report.SuccessCount)
	require.Len(t, report.Violations, 2)
	require.Len(t, report.Warnings, 1)
	requireRuleMetadata(t, report.Rules, "main.required_task_missing", "Required build task is present", "violation")
	requireRuleMetadata(t, report.Rules, "main.signer_not_allowed", "Attestation signer is allowed", "violation")
	requireRuleMetadata(t, report.Rules, "main.application_label_missing", "Application label is present", "warning")
	requireResult(t, report.Violations, "main.required_task_missing", "required task build-container is missing")
	requireResult(t, report.Violations, "main.signer_not_allowed", "attestation signer \"https://untrusted.example/signer\" is not allowed")
	requireResult(t, report.Warnings, "main.application_label_missing", "application label is recommended")
}

func TestEvaluatePlaygroundConformaShapedFixturePasses(t *testing.T) {
	policy := readFixture(t, "fixtures/conforma-shaped/policy.rego")
	input := readFixture(t, "fixtures/conforma-shaped/input-pass.yaml")
	data := readFixture(t, "fixtures/conforma-shaped/data.yaml")

	report := EvaluateWithData(context.Background(), policy, input, data)

	require.True(t, report.Success)
	require.Empty(t, report.Errors)
	require.Empty(t, report.Violations)
	require.Empty(t, report.Warnings)
	require.Len(t, report.Rules, 3)
	require.Equal(t, 3, report.SuccessCount)
}

func readFixture(t *testing.T, path string) string {
	t.Helper()
	path = filepath.Join("..", "..", path)
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(content)
}

func requireRuleMetadata(t *testing.T, rules []RuleMetadata, code string, title string, kind string) {
	t.Helper()
	for _, rule := range rules {
		if rule.Code == code {
			require.Equal(t, title, rule.Title)
			require.Equal(t, kind, rule.Kind)
			return
		}
	}
	t.Fatalf("rule metadata with code %q not found in %#v", code, rules)
}

func requireResult(t *testing.T, results []Result, code string, msg string) {
	t.Helper()
	for _, result := range results {
		value, ok := result.Value.(map[string]any)
		if !ok {
			continue
		}
		if value["code"] == code {
			require.Equal(t, msg, result.Message)
			return
		}
	}
	t.Fatalf("result with code %q not found in %#v", code, results)
}
