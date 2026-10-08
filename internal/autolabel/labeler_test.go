package autolabel

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
	"testing"

	"github.com/Bugs5382/release-drafter-action/internal/config"
)

const testAutolabelConfig = `
template: '$CHANGES'
autolabeler:
  - label: dependencies
    files: ['package.json', 'package-lock.json']
  - label: dependencies
    branch: ['/^dependabot\//', '/^renovate\//']
  - label: enhancement
    title: ['/^feat(\(.+\))?:/']
  - label: breaking
    title: ['/^[a-z]+(\(.+\))?!:/']
    body: ['/^BREAKING[ -]CHANGE:/m']
`

func compiledRules(t *testing.T) []config.CompiledAutolabel {
	t.Helper()
	cfg, err := config.Parse([]byte(testAutolabelConfig), "test")
	if err != nil {
		t.Fatalf("config.Parse: %v", err)
	}
	rules, err := config.CompileAutolabeler(cfg)
	if err != nil {
		t.Fatalf("config.CompileAutolabeler: %v", err)
	}
	return rules
}

func TestLabelsMatchesByChangedFiles(t *testing.T) {
	labels, err := Labels(compiledRules(t), PullRequest{Title: "chore: bump", ChangedFiles: []string{"package.json"}})
	if err != nil {
		t.Fatalf("Labels: %v", err)
	}
	if len(labels) != 1 || labels[0] != "dependencies" {
		t.Fatalf("labels = %v, want [dependencies]", labels)
	}
}

func TestLabelsMatchesByBranchWhenFilesDoNotMatch(t *testing.T) {
	labels, err := Labels(compiledRules(t), PullRequest{Title: "chore: bump", BranchRef: "dependabot/npm_and_yarn/widget-2.0.0"})
	if err != nil {
		t.Fatalf("Labels: %v", err)
	}
	if len(labels) != 1 || labels[0] != "dependencies" {
		t.Fatalf("labels = %v, want [dependencies]", labels)
	}
}

func TestLabelsMatchesByTitle(t *testing.T) {
	labels, err := Labels(compiledRules(t), PullRequest{Title: "feat: add widget"})
	if err != nil {
		t.Fatalf("Labels: %v", err)
	}
	if len(labels) != 1 || labels[0] != "enhancement" {
		t.Fatalf("labels = %v, want [enhancement]", labels)
	}
}

func TestLabelsMatchesByBodyOnlyWhenTitleDoesNotMatchAndBodyIsNonEmpty(t *testing.T) {
	labels, err := Labels(compiledRules(t), PullRequest{Title: "fix: patch", Body: "BREAKING-CHANGE: removed the old flag"})
	if err != nil {
		t.Fatalf("Labels: %v", err)
	}
	if len(labels) != 1 || labels[0] != "breaking" {
		t.Fatalf("labels = %v, want [breaking]", labels)
	}
}

func TestLabelsSkipsBodyCheckWhenBodyIsEmpty(t *testing.T) {
	labels, err := Labels(compiledRules(t), PullRequest{Title: "fix: patch", Body: ""})
	if err != nil {
		t.Fatalf("Labels: %v", err)
	}
	if len(labels) != 0 {
		t.Fatalf("labels = %v, want none", labels)
	}
}

func TestLabelsCollectsAndDedupesAcrossMultipleRules(t *testing.T) {
	// A breaking, dependency-touching feature: matches "enhancement" via
	// title... but "breaking" wins priority in v7's own default config via
	// exclusivity; this package has no opinion on that, it just reports
	// every rule that matched. Here feat! matches both enhancement's and
	// breaking's title patterns.
	labels, err := Labels(compiledRules(t), PullRequest{
		Title:        "feat!: rewrite the engine",
		ChangedFiles: []string{"package.json"},
	})
	if err != nil {
		t.Fatalf("Labels: %v", err)
	}
	want := map[string]bool{"dependencies": true, "breaking": true}
	if len(labels) != 2 {
		t.Fatalf("labels = %v, want 2 labels", labels)
	}
	for _, l := range labels {
		if !want[l] {
			t.Errorf("unexpected label %s", l)
		}
	}
}

func TestLabelsDedupesWhenTwoRulesShareALabel(t *testing.T) {
	labels, err := Labels(compiledRules(t), PullRequest{
		Title:        "chore: bump deps",
		BranchRef:    "renovate/widget-2.0.0",
		ChangedFiles: []string{"package.json"},
	})
	if err != nil {
		t.Fatalf("Labels: %v", err)
	}
	if len(labels) != 1 || labels[0] != "dependencies" {
		t.Fatalf("labels = %v, want a single deduped [dependencies]", labels)
	}
}

func TestLabelsReturnsNoneWhenNothingMatches(t *testing.T) {
	labels, err := Labels(compiledRules(t), PullRequest{Title: "random change with no convention"})
	if err != nil {
		t.Fatalf("Labels: %v", err)
	}
	if len(labels) != 0 {
		t.Fatalf("labels = %v, want none", labels)
	}
}
