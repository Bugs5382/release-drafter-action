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

func TestParseFlagsDefaultsAndOverrides(t *testing.T) {
	in, err := parseFlags(Flags{
		ConfigName: "release-drafter.yml",
		Name:       "v1.2.3", Latest: "true", Prerelease: "false", Publish: "yes", DryRun: "off",
	})
	if err != nil {
		t.Fatalf("parseFlags: %v", err)
	}
	if in.Inputs.Name == nil || *in.Inputs.Name != "v1.2.3" {
		t.Errorf("Name = %v, want v1.2.3", in.Inputs.Name)
	}
	if in.Inputs.Latest == nil || !*in.Inputs.Latest {
		t.Errorf("Latest = %v, want true", in.Inputs.Latest)
	}
	if in.Inputs.Prerelease == nil || *in.Inputs.Prerelease {
		t.Errorf("Prerelease = %v, want false", in.Inputs.Prerelease)
	}
	if !in.Inputs.Publish {
		t.Error("Publish = false, want true (parsed from \"yes\")")
	}
	if in.Inputs.DryRun {
		t.Error("DryRun = true, want false (parsed from \"off\")")
	}
	if in.Inputs.Tag != nil {
		t.Errorf("Tag = %v, want nil (unset)", in.Inputs.Tag)
	}
	if in.ExcludeBots {
		t.Error("ExcludeBots = true, want false (unset defaults to false)")
	}
}

func TestParseFlagsCarriesFirstVersionThrough(t *testing.T) {
	in, err := parseFlags(Flags{FirstVersion: "1.0.0"})
	if err != nil {
		t.Fatalf("parseFlags: %v", err)
	}
	if in.FirstVersion != "1.0.0" {
		t.Errorf("FirstVersion = %q, want 1.0.0", in.FirstVersion)
	}
}

func TestParseFlagsRejectsAnInvalidBoolean(t *testing.T) {
	if _, err := parseFlags(Flags{Latest: "maybe"}); err == nil {
		t.Fatal("expected an error for an invalid latest value")
	}
	if _, err := parseFlags(Flags{DryRun: "maybe"}); err == nil {
		t.Fatal("expected an error for an invalid dry-run value")
	}
	if _, err := parseFlags(Flags{ExcludeBots: "maybe"}); err == nil {
		t.Fatal("expected an error for an invalid exclude-bots value")
	}
}

func TestModeFor(t *testing.T) {
	cases := map[string]Mode{
		"pull_request":        ModeAutolabel,
		"pull_request_target": ModeAutolabel,
		"push":                ModeDraft,
		"workflow_dispatch":   ModeDraft,
		"release":             ModeDraft,
		"":                    ModeDraft,
	}
	for event, want := range cases {
		if got := modeFor(event); got != want {
			t.Errorf("modeFor(%q) = %v, want %v", event, got, want)
		}
	}
}
