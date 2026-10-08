package config

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
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// The config sources, in the order Load tries them. The first one that is
// set wins and the rest are ignored; configs are never merged.
const (
	KindInput   = "input"
	KindExtends = "extends"
	KindFile    = "file"
	KindDefault = "default"
)

// Source says where the config text came from.
type Source struct {
	// Text is the raw YAML or JSON.
	Text []byte
	// Origin names the source in logs and errors: "input config", the
	// extends reference, the file path that was read, or "built-in default".
	Origin string
	// Kind is one of KindInput, KindExtends, KindFile and KindDefault.
	Kind string
}

// DefaultConfigName is config-name's default, as a repository path.
const DefaultConfigName = ".github/release-drafter.yml"

// ErrNotFound is returned when the local config file does not exist.
var ErrNotFound = errors.New("no config found")

// ReadLocal reads the config-name file inside workspace, the checked-out
// repository. config-name follows v7's rules: a relative name lives under
// .github/ unless it already starts with .github/, a leading / is the
// repository root, and a file: prefix is allowed. Remote targets
// (owner/repo:path, github:) are rejected: a shared config comes through the
// extends input instead.
func ReadLocal(configName, workspace string) (Source, error) {
	rel, err := NormalizeConfigName(configName)
	if err != nil {
		return Source{}, err
	}
	full := filepath.Join(workspace, filepath.FromSlash(rel))
	data, err := os.ReadFile(filepath.Clean(full))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Source{}, fmt.Errorf("%w: %s does not exist (did the workflow check out the repository?)", ErrNotFound, full)
		}
		return Source{}, fmt.Errorf("reading %s: %w", full, err)
	}
	return Source{Text: data, Origin: rel, Kind: KindFile}, nil
}

// NormalizeConfigName maps config-name to a path relative to the
// repository root.
func NormalizeConfigName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "release-drafter.yml"
	}
	name = strings.TrimPrefix(name, "file:")
	if strings.HasPrefix(name, "github:") || strings.Contains(name, ":") || strings.Contains(name, "@") {
		return "", fmt.Errorf("config-name %q points at another repository or ref; use the extends input for a shared config", name)
	}
	switch ext := strings.ToLower(path.Ext(name)); ext {
	case ".yml", ".yaml", ".json":
	default:
		return "", fmt.Errorf("config-name %q has unsupported extension %q; use .yml, .yaml or .json", name, ext)
	}
	var rel string
	switch {
	case strings.HasPrefix(name, "/"):
		rel = strings.TrimPrefix(path.Clean(name), "/")
	case strings.HasPrefix(path.Clean(name), ".github/"):
		rel = path.Clean(name)
	default:
		rel = path.Join(".github", name)
	}
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return "", fmt.Errorf("config-name %q leaves the repository", name)
	}
	return rel, nil
}
