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

import "testing"

func TestLoadPullRequestEventRejectsAnEmptyPath(t *testing.T) {
	if _, err := loadPullRequestEvent(""); err == nil {
		t.Fatal("expected an error for an empty GITHUB_EVENT_PATH")
	}
}

func TestLoadPullRequestEventParsesTheStandardPayload(t *testing.T) {
	path := writeEventFile(t, `{"number":5,"pull_request":{"title":"feat: x","body":"details","head":{"ref":"feature/x"}}}`)
	ev, err := loadPullRequestEvent(path)
	if err != nil {
		t.Fatalf("loadPullRequestEvent: %v", err)
	}
	if ev.Number != 5 || ev.PullRequest.Title != "feat: x" || ev.PullRequest.Head.Ref != "feature/x" {
		t.Errorf("ev = %+v", ev)
	}
	if ev.PullRequest.Body == nil || *ev.PullRequest.Body != "details" {
		t.Errorf("ev.PullRequest.Body = %v", ev.PullRequest.Body)
	}
}

func TestLoadPullRequestEventAllowsANullBody(t *testing.T) {
	path := writeEventFile(t, `{"number":5,"pull_request":{"title":"feat: x","body":null,"head":{"ref":"feature/x"}}}`)
	ev, err := loadPullRequestEvent(path)
	if err != nil {
		t.Fatalf("loadPullRequestEvent: %v", err)
	}
	if ev.PullRequest.Body != nil {
		t.Errorf("ev.PullRequest.Body = %v, want nil", ev.PullRequest.Body)
	}
}

func TestLoadPullRequestEventRejectsAPayloadWithNoPullRequest(t *testing.T) {
	path := writeEventFile(t, `{"action":"opened"}`)
	if _, err := loadPullRequestEvent(path); err == nil {
		t.Fatal("expected an error for a payload with no pull request number")
	}
}
