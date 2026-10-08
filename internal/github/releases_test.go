package github

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
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestListReleasesPaginatesAndDecodes(t *testing.T) {
	pages := [][]byte{loadFixture(t, "releases_page1.json"), loadFixture(t, "releases_page2.json")}
	f := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/acme/widget/releases" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		page := pages[0]
		if r.URL.Query().Get("page") == "2" {
			page = pages[1]
			w.Header().Set("Link", "")
		} else {
			w.Header().Set("Link", `<https://example.invalid/releases?page=2>; rel="next"`)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(page)
	})
	c := newTestClient(t, f)

	releases, err := c.ListReleases(context.Background(), "acme", "widget")
	if err != nil {
		t.Fatalf("ListReleases: %v", err)
	}
	if len(releases) != 3 {
		t.Fatalf("releases = %d, want 3", len(releases))
	}
	if releases[0].TagName != "v1.1.0" || releases[0].Draft {
		t.Errorf("releases[0] = %+v", releases[0])
	}
	if !releases[2].Draft {
		t.Errorf("releases[2] (page 2) should be the draft: %+v", releases[2])
	}
}

func TestCreateReleaseSendsPayloadAndDecodesResult(t *testing.T) {
	var gotBody map[string]any
	f := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/repos/acme/widget/releases" {
			t.Fatalf("method/path = %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"id":42,"tag_name":"v1.2.0","name":"v1.2.0","draft":true,"prerelease":false}`)
	})
	c := newTestClient(t, f)

	name := "v1.2.0"
	tag := "v1.2.0"
	out, err := c.CreateRelease(context.Background(), "acme", "widget", ReleaseInput{
		Name: &name, TagName: &tag, TargetCommitish: "main", Body: "notes", Draft: true, MakeLatest: "false",
	})
	if err != nil {
		t.Fatalf("CreateRelease: %v", err)
	}
	if out.ID != 42 || out.TagName != "v1.2.0" || !out.Draft {
		t.Errorf("out = %+v", out)
	}
	if gotBody["target_commitish"] != "main" || gotBody["body"] != "notes" || gotBody["make_latest"] != "false" {
		t.Errorf("gotBody = %v", gotBody)
	}
}

func TestUpdateReleaseSendsPayloadAndDecodesResult(t *testing.T) {
	var gotBody map[string]any
	f := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/repos/acme/widget/releases/42" {
			t.Fatalf("method/path = %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"id":42,"tag_name":"v1.2.0","draft":false,"prerelease":false}`)
	})
	c := newTestClient(t, f)

	out, err := c.UpdateRelease(context.Background(), "acme", "widget", 42, ReleaseInput{Body: "updated notes", Draft: false})
	if err != nil {
		t.Fatalf("UpdateRelease: %v", err)
	}
	if out.ID != 42 || out.Draft {
		t.Errorf("out = %+v", out)
	}
	// Name and TagName were left nil (not overridden); the request must not
	// send them at all, so GitHub keeps the release's existing values.
	if _, ok := gotBody["name"]; ok {
		t.Errorf("gotBody sent name even though ReleaseInput.Name was nil: %v", gotBody)
	}
	if _, ok := gotBody["tag_name"]; ok {
		t.Errorf("gotBody sent tag_name even though ReleaseInput.TagName was nil: %v", gotBody)
	}
}
