package render

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
	"sync"
	"testing"

	"github.com/Bugs5382/release-drafter-action/internal/config"
)

// TestAuthorsSentenceAndNewContributorsConcurrent pins that AuthorsSentence
// and NewContributors are safe to call concurrently. Both used a shared
// package-level collate.Collator, whose CompareString mutates internal
// buffers; run with -race to catch the regression.
func TestAuthorsSentenceAndNewContributorsConcurrent(t *testing.T) {
	f := newFixture()
	cfg, err := config.Parse([]byte("template: \"$CHANGES\"\n"), "test")
	if err != nil {
		t.Fatal(err)
	}
	p, err := config.Merge(cfg, config.Inputs{}, "refs/heads/main")
	if err != nil {
		t.Fatal(err)
	}
	newLogins := map[string]bool{"Bob": true, "carol": true}

	const goroutines = 8
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = AuthorsSentence(AuthorsOptions{
				Commits:      f.commits,
				PullRequests: f.prs,
				ServerURL:    "https://github.com",
			})
			_ = NewContributors(f.prs, newLogins, p)
		}()
	}
	wg.Wait()
}
