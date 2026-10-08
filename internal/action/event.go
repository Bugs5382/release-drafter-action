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
	"encoding/json"
	"fmt"
	"os"
)

// pullRequestEvent is the subset of the pull_request / pull_request_target
// event payload the autolabeler needs: the number (to add labels to) and
// the fields the rules match against. Body is a pointer because the
// payload can carry a null body, which autolabel.Labels treats as empty
// rather than running the body patterns against it.
type pullRequestEvent struct {
	Number      int `json:"number"`
	PullRequest struct {
		Title string  `json:"title"`
		Body  *string `json:"body"`
		Head  struct {
			Ref string `json:"ref"`
		} `json:"head"`
	} `json:"pull_request"`
}

// loadPullRequestEvent reads and decodes the event payload GITHUB_EVENT_PATH
// points at.
func loadPullRequestEvent(path string) (pullRequestEvent, error) {
	var ev pullRequestEvent
	if path == "" {
		return ev, fmt.Errorf("action: GITHUB_EVENT_PATH is empty; autolabel mode needs the pull_request event payload")
	}
	data, err := os.ReadFile(path) // #nosec G304 -- path is GITHUB_EVENT_PATH, set by the Actions runner itself
	if err != nil {
		return ev, fmt.Errorf("action: reading the event payload %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &ev); err != nil {
		return ev, fmt.Errorf("action: decoding the event payload %s: %w", path, err)
	}
	if ev.Number == 0 {
		return ev, fmt.Errorf("action: the event payload %s has no pull request number", path)
	}
	return ev, nil
}
