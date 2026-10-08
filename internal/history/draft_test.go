package history

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
	"testing"

	"github.com/Bugs5382/release-drafter-action/internal/github"
	"github.com/Bugs5382/release-drafter-action/internal/model"
	"github.com/Bugs5382/release-drafter-action/internal/render"
)

type fakeUpsertClient struct {
	createCalls []github.ReleaseInput
	updateCalls []struct {
		id int64
		in github.ReleaseInput
	}
	result model.Release
	err    error
}

func (f *fakeUpsertClient) CreateRelease(_ context.Context, _, _ string, in github.ReleaseInput) (model.Release, error) {
	f.createCalls = append(f.createCalls, in)
	return f.result, f.err
}

func (f *fakeUpsertClient) UpdateRelease(_ context.Context, _, _ string, id int64, in github.ReleaseInput) (model.Release, error) {
	f.updateCalls = append(f.updateCalls, struct {
		id int64
		in github.ReleaseInput
	}{id, in})
	return f.result, f.err
}

func TestUpsertCreatesWhenThereIsNoExistingDraft(t *testing.T) {
	f := &fakeUpsertClient{result: model.Release{ID: 1, TagName: "v1.1.0"}}
	out, err := Upsert(context.Background(), f, UpsertInput{
		Owner: "acme", Repo: "widget", TargetCommitish: "main",
		Payload: render.Payload{Name: "v1.1.0", Tag: "v1.1.0", Body: "notes", Draft: true, MakeLatest: true},
	})
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if out == nil || out.ID != 1 {
		t.Fatalf("out = %+v", out)
	}
	if len(f.createCalls) != 1 || len(f.updateCalls) != 0 {
		t.Fatalf("createCalls = %d, updateCalls = %d", len(f.createCalls), len(f.updateCalls))
	}
	call := f.createCalls[0]
	if call.Name == nil || *call.Name != "v1.1.0" || call.TargetCommitish != "main" || call.MakeLatest != "true" {
		t.Fatalf("createCalls[0] = %+v", call)
	}
}

func TestUpsertUpdatesAnExistingDraft(t *testing.T) {
	f := &fakeUpsertClient{result: model.Release{ID: 7}}
	draft := &model.Release{ID: 7, TagName: "v1.1.0-draft"}
	_, err := Upsert(context.Background(), f, UpsertInput{
		Owner: "acme", Repo: "widget", DraftRelease: draft,
		Payload: render.Payload{Name: "v1.1.0", Tag: "v1.1.0", Body: "updated notes"},
	})
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if len(f.updateCalls) != 1 || len(f.createCalls) != 0 {
		t.Fatalf("createCalls = %d, updateCalls = %d", len(f.createCalls), len(f.updateCalls))
	}
	if f.updateCalls[0].id != 7 {
		t.Fatalf("updateCalls[0].id = %d, want 7", f.updateCalls[0].id)
	}
}

func TestUpsertLeavesNameAndTagUnsetOnUpdateWhenThePayloadHasNone(t *testing.T) {
	f := &fakeUpsertClient{}
	draft := &model.Release{ID: 7}
	_, err := Upsert(context.Background(), f, UpsertInput{
		Owner: "acme", Repo: "widget", DraftRelease: draft, Payload: render.Payload{Body: "notes"},
	})
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if f.updateCalls[0].in.Name != nil || f.updateCalls[0].in.TagName != nil {
		t.Fatalf("in = %+v, want Name and TagName left nil", f.updateCalls[0].in)
	}
}

func TestUpsertSetsMakeLatestFalseForAPrereleaseRegardlessOfLatest(t *testing.T) {
	f := &fakeUpsertClient{}
	_, err := Upsert(context.Background(), f, UpsertInput{
		Owner: "acme", Repo: "widget",
		Payload: render.Payload{Name: "v2.0.0-beta.1", Tag: "v2.0.0-beta.1", Prerelease: true, MakeLatest: true},
	})
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if f.createCalls[0].MakeLatest != "false" {
		t.Fatalf("MakeLatest = %s, want false for a prerelease", f.createCalls[0].MakeLatest)
	}
}

func TestUpsertSkipsTheCallWhenEmpty(t *testing.T) {
	f := &fakeUpsertClient{}
	out, err := Upsert(context.Background(), f, UpsertInput{Owner: "acme", Repo: "widget", Empty: true})
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if out != nil {
		t.Fatalf("out = %+v, want nil", out)
	}
	if len(f.createCalls) != 0 || len(f.updateCalls) != 0 {
		t.Fatalf("expected no API calls when empty, got create=%d update=%d", len(f.createCalls), len(f.updateCalls))
	}
}

func TestUpsertSkipsTheCallOnDryRun(t *testing.T) {
	f := &fakeUpsertClient{}
	out, err := Upsert(context.Background(), f, UpsertInput{
		Owner: "acme", Repo: "widget", DryRun: true,
		Payload: render.Payload{Name: "v1.1.0", Tag: "v1.1.0"},
	})
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if out != nil {
		t.Fatalf("out = %+v, want nil", out)
	}
	if len(f.createCalls) != 0 || len(f.updateCalls) != 0 {
		t.Fatalf("expected no API calls on a dry run, got create=%d update=%d", len(f.createCalls), len(f.updateCalls))
	}
}
