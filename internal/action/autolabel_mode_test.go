package action

/*
ISC License

Copyright (c) 2026 Shane & Contributors

Permission to use, copy, modify, and/or distribute this software for any
purpose with or without fee is hereby granted, provided that the above
copyright notice and this permission notice appear in all copies.

THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
*/

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeEventFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "event.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestRunAutolabelAddsMatchingLabel drives autolabel mode with the hub
// config's own autolabeler rules: a Conventional Commits "feat:" title
// matches only the "enhancement" rule, and the action adds just that label.
func TestRunAutolabelAddsMatchingLabel(t *testing.T) {
	cfgData, err := os.ReadFile(filepath.Join("testdata", "hub-plain.yml"))
	if err != nil {
		t.Fatal(err)
	}
	eventPath := writeEventFile(t, `{"number":42,"pull_request":{"title":"feat(api): add thing","body":null,"head":{"ref":"feature/add-thing"}}}`)

	f := newFakeGitHub(t)
	var gotNumber string
	var gotBody []byte
	f.addLabels = func(number string, body []byte) {
		gotNumber, gotBody = number, body
	}

	var stdout bytes.Buffer
	code := Run(context.Background(), Options{
		Flags: Flags{Config: string(cfgData)},
		Env: Env{
			EventName: "pull_request", EventPath: eventPath, Token: "test-token", Repository: "o/r",
			ServerURL: "https://github.com", APIURL: f.srv.URL, GraphQLURL: f.srv.URL + "/graphql",
		},
		Stdout: &stdout,
	})
	if code != 0 {
		t.Fatalf("Run = %d, want 0; stdout: %s", code, stdout.String())
	}
	if gotNumber != "42" {
		t.Errorf("AddLabels called for pull request %q, want 42", gotNumber)
	}
	if !strings.Contains(string(gotBody), "enhancement") {
		t.Errorf("AddLabels body = %s, want it to hold enhancement", gotBody)
	}
	if strings.Contains(string(gotBody), "breaking") || strings.Contains(string(gotBody), "fix") {
		t.Errorf("AddLabels body = %s, only enhancement should match", gotBody)
	}
}

// TestRunAutolabelDryRunNeverCallsAddLabels proves dry-run mode never
// writes, even though a rule matches.
func TestRunAutolabelDryRunNeverCallsAddLabels(t *testing.T) {
	cfgData, err := os.ReadFile(filepath.Join("testdata", "hub-plain.yml"))
	if err != nil {
		t.Fatal(err)
	}
	eventPath := writeEventFile(t, `{"number":7,"pull_request":{"title":"fix: crash on startup","body":null,"head":{"ref":"fix/crash"}}}`)

	f := newFakeGitHub(t)
	called := false
	f.addLabels = func(string, []byte) { called = true }

	var stdout bytes.Buffer
	code := Run(context.Background(), Options{
		Flags: Flags{Config: string(cfgData), DryRun: "true"},
		Env: Env{
			EventName: "pull_request_target", EventPath: eventPath, Token: "test-token", Repository: "o/r",
			ServerURL: "https://github.com", APIURL: f.srv.URL, GraphQLURL: f.srv.URL + "/graphql",
		},
		Stdout: &stdout,
	})
	if code != 0 {
		t.Fatalf("Run = %d, want 0; stdout: %s", code, stdout.String())
	}
	if called {
		t.Error("dry run must not call AddLabels")
	}
	if !strings.Contains(stdout.String(), "fix") {
		t.Errorf("dry run stdout = %q, want it to report the fix label", stdout.String())
	}
}
