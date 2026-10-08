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
	"fmt"

	"github.com/Bugs5382/release-drafter-action/internal/config"
	"github.com/Bugs5382/release-drafter-action/internal/logging"
	"github.com/Bugs5382/release-drafter-action/internal/regex"
)

// PullRequest is what Labels needs from the pull_request event payload.
type PullRequest struct {
	Title        string
	Body         string
	BranchRef    string
	ChangedFiles []string
}

// Labels ports v7's autolabeler loop: for every rule, the first of files,
// branch, title and body that matches adds the rule's label. Body is only
// checked when it is non-empty (the event payload may carry a null body).
// Labels are returned in rule order, deduplicated; the result only ever
// grows, matching the caller's "add, never remove" contract.
func Labels(rules []config.CompiledAutolabel, pr PullRequest) ([]string, error) {
	logging.L().Debug().Int("rules", len(rules)).Int("changed_files", len(pr.ChangedFiles)).
		Msg("autolabel: matching pull request against the autolabeler rules")

	seen := map[string]bool{}
	var out []string
	add := func(label, reason string) {
		logging.L().Info().Str("label", label).Str("reason", reason).Msg("autolabel: rule matched")
		if !seen[label] {
			seen[label] = true
			out = append(out, label)
		}
	}

	for i, rule := range rules {
		if rule.Files != nil && rule.Files.MatchAny(pr.ChangedFiles) {
			add(rule.Label, "files")
			continue
		}
		matched, err := matchAny(rule.Branch, pr.BranchRef)
		if err != nil {
			return nil, fmt.Errorf("autolabel: rule %d (%s): matching branch: %w", i, rule.Label, err)
		}
		if matched {
			add(rule.Label, "branch")
			continue
		}
		matched, err = matchAny(rule.Title, pr.Title)
		if err != nil {
			return nil, fmt.Errorf("autolabel: rule %d (%s): matching title: %w", i, rule.Label, err)
		}
		if matched {
			add(rule.Label, "title")
			continue
		}
		if pr.Body == "" {
			continue
		}
		matched, err = matchAny(rule.Body, pr.Body)
		if err != nil {
			return nil, fmt.Errorf("autolabel: rule %d (%s): matching body: %w", i, rule.Label, err)
		}
		if matched {
			add(rule.Label, "body")
		}
	}

	logging.L().Debug().Int("labels", len(out)).Msg("autolabel: finished matching")
	return out, nil
}

func matchAny(patterns []*regex.Regex, s string) (bool, error) {
	for _, p := range patterns {
		ok, err := p.MatchString(s)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}
