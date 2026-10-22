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

package conformaplayground_test

import (
	"context"
	"os"
	"testing"

	clipg "github.com/conforma/cli/pkg/playground"
	"github.com/stretchr/testify/require"
)

func TestConformaCLIPlaygroundEvaluatorFixture(t *testing.T) {
	response, err := clipg.Evaluate(context.Background(), clipg.Request{
		Policy: readFixture(t, "fixtures/conforma-shaped/policy.rego"),
		Input:  readFixture(t, "fixtures/conforma-shaped/input-fail.yaml"),
		Data:   readFixture(t, "fixtures/conforma-shaped/data.yaml"),
	})

	require.NoError(t, err)
	require.False(t, response.Success)
	require.Equal(t, "main", response.Package)
	require.Equal(t, []string{"main.rego", "lib/pipeline.rego", "lib/release.rego"}, response.Modules)
	require.Equal(t, 0, response.SuccessCount)
	require.Len(t, response.Violations, 2)
	require.Len(t, response.Warnings, 1)
	requireResult(t, response.Violations, "main.required_task_missing", "required task build-container is missing")
	requireResult(t, response.Violations, "main.signer_not_allowed", "attestation signer \"https://untrusted.example/signer\" is not allowed")
	requireResult(t, response.Warnings, "main.application_label_missing", "application label is recommended")
}

func TestConformaCLIPlaygroundEvaluatorPassingFixture(t *testing.T) {
	response, err := clipg.Evaluate(context.Background(), clipg.Request{
		Policy: readFixture(t, "fixtures/conforma-shaped/policy.rego"),
		Input:  readFixture(t, "fixtures/conforma-shaped/input-pass.yaml"),
		Data:   readFixture(t, "fixtures/conforma-shaped/data.yaml"),
	})

	require.NoError(t, err)
	require.True(t, response.Success)
	require.Empty(t, response.Violations)
	require.Empty(t, response.Warnings)
	require.Equal(t, 3, response.SuccessCount)
	require.Len(t, response.Successes, 3)
}

func readFixture(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(content)
}

func requireResult(t *testing.T, results []clipg.Result, code string, msg string) {
	t.Helper()
	for _, result := range results {
		if result.Metadata["code"] == code {
			require.Equal(t, msg, result.Message)
			return
		}
	}
	t.Fatalf("result with code %q not found in %#v", code, results)
}
