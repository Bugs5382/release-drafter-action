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

// Mode is which of the action's two jobs Run performs.
type Mode int

const (
	// ModeDraft creates or updates the draft release: push, workflow_dispatch
	// and release events, and anything else (a local run with no event name).
	ModeDraft Mode = iota
	// ModeAutolabel adds labels to a pull request: pull_request and
	// pull_request_target events.
	ModeAutolabel
)

// modeFor picks the mode from GITHUB_EVENT_NAME, the way action.yml's
// AGENTS.md documents it.
func modeFor(eventName string) Mode {
	switch eventName {
	case "pull_request", "pull_request_target":
		return ModeAutolabel
	default:
		return ModeDraft
	}
}
