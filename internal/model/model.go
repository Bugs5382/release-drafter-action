package model

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

import "strconv"

// Actor is a pull request author. Typename is the GraphQL __typename
// ("User", "Bot", "Organization" and so on).
type Actor struct {
	Typename string
	Login    string
	URL      string
}

// CommitAuthor is a git author. Login is "" when the author has no GitHub
// user.
type CommitAuthor struct {
	Name  string
	Login string
}

// PullRequest is a merged pull request with the fields v7's
// PullRequestFields fragment requests. Optional fields are nil when the
// query did not ask for them, so templates leave their variable in place.
type PullRequest struct {
	Number            int
	Title             string
	URL               *string
	Body              *string
	Author            *Actor
	BaseRepository    string
	MergedAt          *string
	IsCrossRepository bool
	Labels            []string
	Merged            bool
	BaseRefName       *string
	HeadRefName       *string
	// MergeCommitOID is only set on pull requests recovered from the
	// recent-merged query.
	MergeCommitOID string
	// ChangedFiles is only loaded when a category needs paths.
	ChangedFiles []string
}

// Key identifies a pull request the way v7 does: "owner/repo#number".
func (p PullRequest) Key() string {
	return p.BaseRepository + "#" + strconv.Itoa(p.Number)
}

// Commit is one commit in the release range.
type Commit struct {
	OID           string
	CommittedDate string
	Message       string
	Author        *CommitAuthor
	// Authors lists every author including co-authors. v7 falls back to
	// Author only when this is nil.
	Authors      []CommitAuthor
	PullRequests []PullRequest
}

// Release is a GitHub release.
type Release struct {
	ID              int64
	TagName         string
	Name            string
	Draft           bool
	Prerelease      bool
	TargetCommitish string
	CreatedAt       string
	HTMLURL         string
	UploadURL       string
}
