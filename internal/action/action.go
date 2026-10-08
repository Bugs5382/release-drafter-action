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
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/Bugs5382/release-drafter-action/internal/github"
	"github.com/Bugs5382/release-drafter-action/internal/logging"
)

// Options is everything Run needs beyond the process logger (logging.L(),
// set up once by cmd/action before calling Run).
type Options struct {
	Flags Flags
	Env   Env
	// Stdout carries dry-run output; Stderr is unused today but kept for
	// symmetry and future diagnostics. Both only ever receive plain text,
	// never a secret.
	Stdout, Stderr io.Writer
	// ExtendsCacheDir holds downloaded extends assets, keyed by tag; ""
	// turns the cache off. cmd/action points this at RUNNER_TEMP; tests
	// point it at a throwaway directory or leave it off.
	ExtendsCacheDir string
}

// Run dispatches to the draft or autolabel mode and returns the process
// exit code: 0 on success, 1 when anything fails. Every failure is also
// written to Stdout as a workflow ::error annotation before Run returns.
func Run(ctx context.Context, o Options) int {
	ann := logging.Annotator{W: o.Stdout}

	in, err := parseFlags(o.Flags)
	if err != nil {
		return fail(ann, err)
	}
	owner, repo, err := splitRepository(o.Env.Repository)
	if err != nil {
		return fail(ann, err)
	}

	client := github.New(github.Options{
		RESTBaseURL: o.Env.APIURL,
		GraphQLURL:  o.Env.GraphQLURL,
		Token:       o.Env.Token,
		Log:         logging.L(),
	})

	switch modeFor(o.Env.EventName) {
	case ModeAutolabel:
		return runAutolabel(ctx, client, o, in, owner, repo, ann)
	default:
		return runDraft(ctx, client, o, in, owner, repo, ann)
	}
}

func fail(ann logging.Annotator, err error) int {
	ann.Error("release-drafter-action", err.Error())
	return 1
}

func splitRepository(repository string) (owner, repo string, err error) {
	o, r, ok := strings.Cut(repository, "/")
	if !ok || o == "" || r == "" {
		return "", "", fmt.Errorf("action: GITHUB_REPOSITORY %q is not owner/repo", repository)
	}
	return o, r, nil
}
