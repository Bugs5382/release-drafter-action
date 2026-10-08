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
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// FieldError is one problem found while validating the config.
type FieldError struct {
	// Path locates the value, for example "categories[2].collapse-after".
	Path string
	// Line is the 1-based line in the config text, 0 when unknown.
	Line int
	// Msg says what was wrong and what was expected.
	Msg string
}

func (e FieldError) String() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s (line %d): %s", e.Path, e.Line, e.Msg)
	}
	return fmt.Sprintf("%s: %s", e.Path, e.Msg)
}

// ValidationError lists every problem in a config, in document order.
type ValidationError struct {
	Origin string
	Errors []FieldError
}

func (e *ValidationError) Error() string {
	lines := make([]string, len(e.Errors))
	for i, fe := range e.Errors {
		lines[i] = fe.String()
	}
	if e.Origin == "" {
		return "invalid config:\n" + strings.Join(lines, "\n")
	}
	return fmt.Sprintf("invalid config in %s:\n%s", e.Origin, strings.Join(lines, "\n"))
}

type walker struct{ errs []FieldError }

func (w *walker) fail(n *yaml.Node, path, format string, args ...any) {
	line := 0
	if n != nil {
		line = n.Line
	}
	w.errs = append(w.errs, FieldError{Path: path, Line: line, Msg: fmt.Sprintf(format, args...)})
}

func describe(n *yaml.Node) string {
	switch n.Kind {
	case yaml.MappingNode:
		return "a mapping"
	case yaml.SequenceNode:
		return "a list"
	case yaml.AliasNode:
		return describe(n.Alias)
	}
	if n.Tag == "!!null" {
		return "null"
	}
	return fmt.Sprintf("%s %q", strings.TrimPrefix(n.Tag, "!!"), n.Value)
}

func child(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

func deref(n *yaml.Node) *yaml.Node {
	for n != nil && n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	return n
}

// fields walks a mapping and hands each known key to its handler. Unknown
// and repeated keys are errors that name the key and where it is; v7's yaml
// parse rejects a repeated key, so only the first occurrence is used.
func (w *walker) fields(n *yaml.Node, path string, handlers map[string]func(*yaml.Node, string)) {
	n = deref(n)
	if n.Kind != yaml.MappingNode {
		w.fail(n, pathOrRoot(path), "expected a mapping, got %s", describe(n))
		return
	}
	seen := map[string]int{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		if first, dup := seen[k.Value]; dup {
			w.fail(k, child(path, k.Value), "duplicate key %q (first at line %d)", k.Value, first)
			continue
		}
		seen[k.Value] = k.Line
		h, ok := handlers[k.Value]
		if !ok {
			w.fail(k, child(path, k.Value), "unknown key %q", k.Value)
			continue
		}
		h(deref(v), child(path, k.Value))
	}
}

func pathOrRoot(p string) string {
	if p == "" {
		return "(root)"
	}
	return p
}

func isString(n *yaml.Node) bool {
	return n.Kind == yaml.ScalarNode && (n.Tag == "!!str" || n.Tag == "!!timestamp")
}

func (w *walker) str(n *yaml.Node, path string, dst *string) {
	if !isString(n) {
		w.fail(n, path, "expected a string, got %s", describe(n))
		return
	}
	*dst = n.Value
}

func (w *walker) strMin1(n *yaml.Node, path string, dst *string) {
	if !isString(n) || n.Value == "" {
		w.fail(n, path, "expected a non-empty string, got %s", describe(n))
		return
	}
	*dst = n.Value
}

func (w *walker) optStr(n *yaml.Node, path string, dst **string) {
	var s string
	before := len(w.errs)
	w.str(n, path, &s)
	if len(w.errs) == before {
		*dst = &s
	}
}

func (w *walker) boolean(n *yaml.Node, path string, dst *bool) {
	if n.Kind != yaml.ScalarNode || n.Tag != "!!bool" {
		w.fail(n, path, "expected true or false, got %s", describe(n))
		return
	}
	*dst = n.Value == "true" || n.Value == "True" || n.Value == "TRUE"
}

// stringBool accepts a YAML boolean or a zod stringbool string.
func (w *walker) stringBool(n *yaml.Node, path string, dst **bool) {
	if n.Kind == yaml.ScalarNode && n.Tag == "!!bool" {
		var b bool
		w.boolean(n, path, &b)
		*dst = &b
		return
	}
	if isString(n) {
		b, err := ParseBool(n.Value)
		if err != nil {
			w.fail(n, path, "%v", err)
			return
		}
		*dst = &b
		return
	}
	w.fail(n, path, "expected a boolean, got %s", describe(n))
}

// maxSafeInteger is JavaScript's Number.MAX_SAFE_INTEGER, the largest value
// zod's .int() accepts. It also keeps the float-to-int conversion in range.
const maxSafeInteger = 1<<53 - 1

func (w *walker) integer(n *yaml.Node, path string, min int, dst *int) {
	if n.Kind == yaml.ScalarNode && (n.Tag == "!!int" || n.Tag == "!!float") {
		var f float64
		var err error
		if n.Tag == "!!int" {
			var i int64
			i, err = strconv.ParseInt(strings.ReplaceAll(n.Value, "_", ""), 0, 64)
			f = float64(i)
		} else {
			f, err = strconv.ParseFloat(n.Value, 64)
		}
		if err == nil && f == math.Trunc(f) && f >= float64(min) && f <= maxSafeInteger {
			*dst = int(f)
			return
		}
	}
	w.fail(n, path, "expected an integer >= %d, got %s", min, describe(n))
}

func (w *walker) enum(n *yaml.Node, path string, allowed []string, dst *string) {
	if isString(n) {
		for _, a := range allowed {
			if n.Value == a {
				*dst = a
				return
			}
		}
	}
	w.fail(n, path, "expected one of %s, got %s", strings.Join(allowed, ", "), describe(n))
}

func (w *walker) list(n *yaml.Node, path string, each func(*yaml.Node, string)) {
	if n.Kind != yaml.SequenceNode {
		w.fail(n, path, "expected a list, got %s", describe(n))
		return
	}
	for i, item := range n.Content {
		each(deref(item), fmt.Sprintf("%s[%d]", path, i))
	}
}

func (w *walker) strList(n *yaml.Node, path string, min1 bool, dst *[]string) {
	out := []string{}
	w.list(n, path, func(item *yaml.Node, p string) {
		var s string
		before := len(w.errs)
		if min1 {
			w.strMin1(item, p, &s)
		} else {
			w.str(item, p, &s)
		}
		if len(w.errs) == before {
			out = append(out, s)
		}
	})
	*dst = out
}

var modes = []string{"any", "all", "only", "exactly"}

func (w *walker) condition(n *yaml.Node, path string) Condition {
	c := Condition{Labels: []string{}, Paths: []string{}, LabelsMode: "any", PathsMode: "any"}
	w.fields(n, path, map[string]func(*yaml.Node, string){
		"conventional": func(v *yaml.Node, p string) { c.Conventional = w.conventional(v, p) },
		"label":        func(v *yaml.Node, p string) { w.strMin1(v, p, &c.Label) },
		"labels":       func(v *yaml.Node, p string) { w.strList(v, p, true, &c.Labels) },
		"labels-mode":  func(v *yaml.Node, p string) { w.enum(v, p, modes, &c.LabelsMode) },
		"path":         func(v *yaml.Node, p string) { w.strMin1(v, p, &c.Path) },
		"paths":        func(v *yaml.Node, p string) { w.strList(v, p, true, &c.Paths) },
		"paths-mode":   func(v *yaml.Node, p string) { w.enum(v, p, modes, &c.PathsMode) },
	})
	return c
}

func (w *walker) conventional(n *yaml.Node, path string) *Conventional {
	if n.Kind == yaml.ScalarNode && n.Tag == "!!bool" {
		if n.Value == "true" || n.Value == "True" || n.Value == "TRUE" {
			return &Conventional{Any: true, Types: []string{}, Scopes: []string{}}
		}
		w.fail(n, path, "expected true or a mapping, got %s", describe(n))
		return nil
	}
	c := &Conventional{Types: []string{}, Scopes: []string{}}
	w.fields(n, path, map[string]func(*yaml.Node, string){
		"type":   func(v *yaml.Node, p string) { w.strMin1(v, p, &c.Type) },
		"types":  func(v *yaml.Node, p string) { w.strList(v, p, true, &c.Types) },
		"scope":  func(v *yaml.Node, p string) { w.strMin1(v, p, &c.Scope) },
		"scopes": func(v *yaml.Node, p string) { w.strList(v, p, true, &c.Scopes) },
		"breaking": func(v *yaml.Node, p string) {
			var b bool
			w.boolean(v, p, &b)
			c.Breaking = &b
		},
	})
	return c
}

func (w *walker) category(n *yaml.Node, path string) Category {
	c := Category{Type: "changelog", CollapseAfter: -1, SemverIncrement: "patch", Labels: []string{}, When: []Condition{}}
	w.fields(n, path, map[string]func(*yaml.Node, string){
		"title": func(v *yaml.Node, p string) { w.strMin1(v, p, &c.Title) },
		"type": func(v *yaml.Node, p string) {
			w.enum(v, p, []string{"changelog", "pre-include", "pre-exclude", "version-resolver"}, &c.Type)
		},
		"exclusive":        func(v *yaml.Node, p string) { w.boolean(v, p, &c.Exclusive) },
		"collapse-after":   func(v *yaml.Node, p string) { w.integer(v, p, -1, &c.CollapseAfter) },
		"semver-increment": func(v *yaml.Node, p string) { w.enum(v, p, []string{"major", "minor", "patch"}, &c.SemverIncrement) },
		"labels":           func(v *yaml.Node, p string) { w.strList(v, p, true, &c.Labels) },
		"label":            func(v *yaml.Node, p string) { w.strMin1(v, p, &c.Label) },
		"when": func(v *yaml.Node, p string) {
			if v.Kind == yaml.SequenceNode {
				w.list(v, p, func(item *yaml.Node, ip string) { c.When = append(c.When, w.condition(item, ip)) })
				return
			}
			c.When = []Condition{w.condition(v, p)}
		},
	})
	return c
}

func (w *walker) labelsBlock(n *yaml.Node, path string, dst *[]string) {
	w.fields(n, path, map[string]func(*yaml.Node, string){
		"labels": func(v *yaml.Node, p string) { w.strList(v, p, true, dst) },
	})
}

func (w *walker) autolabel(n *yaml.Node, path string) Autolabel {
	a := Autolabel{Files: []string{}, Branch: []string{}, Title: []string{}, Body: []string{}}
	hasLabel := false
	w.fields(n, path, map[string]func(*yaml.Node, string){
		"label":  func(v *yaml.Node, p string) { hasLabel = true; w.strMin1(v, p, &a.Label) },
		"files":  func(v *yaml.Node, p string) { w.strList(v, p, true, &a.Files) },
		"branch": func(v *yaml.Node, p string) { w.strList(v, p, true, &a.Branch) },
		"title":  func(v *yaml.Node, p string) { w.strList(v, p, true, &a.Title) },
		"body":   func(v *yaml.Node, p string) { w.strList(v, p, true, &a.Body) },
	})
	if !hasLabel && deref(n).Kind == yaml.MappingNode {
		w.fail(n, child(path, "label"), "is required")
	}
	return a
}

// Parse validates YAML (or JSON) config text against every v7 key and
// returns it with v7's defaults applied. origin names the source in errors.
func Parse(data []byte, origin string) (*Config, error) {
	var doc yaml.Node
	dec := yaml.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&doc); err != nil && !errors.Is(err, io.EOF) {
		return nil, &ValidationError{Origin: origin, Errors: []FieldError{{Path: "(root)", Msg: fmt.Sprintf("not valid YAML: %v", err)}}}
	}
	// v7's yaml parse throws on more than one document instead of using
	// the first, so a second document is an error, never silently dropped.
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		msg := "the config must be a single YAML document, but it holds more than one (remove the extra --- separator)"
		if err != nil {
			msg = fmt.Sprintf("not valid YAML: %v", err)
		}
		return nil, &ValidationError{Origin: origin, Errors: []FieldError{{Path: "(root)", Line: extra.Line, Msg: msg}}}
	}
	if len(doc.Content) == 0 {
		return nil, &ValidationError{Origin: origin, Errors: []FieldError{{Path: "(root)", Msg: "the config is empty"}}}
	}
	cfg := Defaults()
	w := &walker{}
	s := func(dst *string) func(*yaml.Node, string) {
		return func(v *yaml.Node, p string) { w.str(v, p, dst) }
	}
	sl := func(dst *[]string) func(*yaml.Node, string) {
		return func(v *yaml.Node, p string) { w.strList(v, p, false, dst) }
	}
	w.fields(doc.Content[0], "", map[string]func(*yaml.Node, string){
		"_extends": func(v *yaml.Node, p string) {
			w.fail(v, p, "is not supported: use the extends input to load a shared config from a release asset")
		},
		"change-template":                s(&cfg.ChangeTemplate),
		"change-author-template":         s(&cfg.ChangeAuthorTemplate),
		"change-authors-separator":       s(&cfg.ChangeAuthorsSeparator),
		"change-authors-final-separator": func(v *yaml.Node, p string) { w.optStr(v, p, &cfg.ChangeAuthorsFinalSeparator) },
		"change-title-escapes":           s(&cfg.ChangeTitleEscapes),
		"no-changes-template":            s(&cfg.NoChangesTemplate),
		"version-template":               s(&cfg.VersionTemplate),
		"name-template":                  s(&cfg.NameTemplate),
		"tag-prefix":                     s(&cfg.TagPrefix),
		"tag-template":                   s(&cfg.TagTemplate),
		"exclude-labels":                 sl(&cfg.ExcludeLabels),
		"include-labels":                 sl(&cfg.IncludeLabels),
		"include-paths":                  sl(&cfg.IncludePaths),
		"exclude-paths":                  sl(&cfg.ExcludePaths),
		"exclude-contributors":           sl(&cfg.ExcludeContributors),
		"new-contributor-template":       s(&cfg.NewContributorTemplate),
		"no-new-contributor-template":    s(&cfg.NoNewContributorTemplate),
		"no-contributors-template":       s(&cfg.NoContributorsTemplate),
		"sort-by":                        func(v *yaml.Node, p string) { w.enum(v, p, []string{"merged_at", "title"}, &cfg.SortBy) },
		"sort-direction":                 func(v *yaml.Node, p string) { w.enum(v, p, []string{"ascending", "descending"}, &cfg.SortDirection) },
		"filter-by-commitish":            func(v *yaml.Node, p string) { w.boolean(v, p, &cfg.FilterByCommitish) },
		"pull-request-limit":             func(v *yaml.Node, p string) { w.integer(v, p, 1, &cfg.PullRequestLimit) },
		"history-limit":                  func(v *yaml.Node, p string) { w.integer(v, p, 1, &cfg.HistoryLimit) },
		"replacers": func(v *yaml.Node, p string) {
			w.list(v, p, func(item *yaml.Node, ip string) {
				r := Replacer{}
				hasSearch, hasReplace := false, false
				w.fields(item, ip, map[string]func(*yaml.Node, string){
					"search":  func(v *yaml.Node, p string) { hasSearch = true; w.strMin1(v, p, &r.Search) },
					"replace": func(v *yaml.Node, p string) { hasReplace = true; w.str(v, p, &r.Replace) },
				})
				if item.Kind == yaml.MappingNode {
					if !hasSearch {
						w.fail(item, child(ip, "search"), "is required")
					}
					if !hasReplace {
						w.fail(item, child(ip, "replace"), "is required")
					}
				}
				cfg.Replacers = append(cfg.Replacers, r)
			})
		},
		"categories": func(v *yaml.Node, p string) {
			w.list(v, p, func(item *yaml.Node, ip string) { cfg.Categories = append(cfg.Categories, w.category(item, ip)) })
		},
		"version-resolver": func(v *yaml.Node, p string) {
			w.fields(v, p, map[string]func(*yaml.Node, string){
				"major": func(v *yaml.Node, p string) { w.labelsBlock(v, p, &cfg.VersionResolver.Major) },
				"minor": func(v *yaml.Node, p string) { w.labelsBlock(v, p, &cfg.VersionResolver.Minor) },
				"patch": func(v *yaml.Node, p string) { w.labelsBlock(v, p, &cfg.VersionResolver.Patch) },
				"default": func(v *yaml.Node, p string) {
					w.enum(v, p, []string{"major", "minor", "patch"}, &cfg.VersionResolver.Default)
				},
			})
		},
		"category-template":     s(&cfg.CategoryTemplate),
		"template":              s(&cfg.Template),
		"latest":                func(v *yaml.Node, p string) { w.stringBool(v, p, &cfg.Latest) },
		"prerelease":            func(v *yaml.Node, p string) { w.stringBool(v, p, &cfg.Prerelease) },
		"prerelease-identifier": s(&cfg.PrereleaseIdentifier),
		"include-pre-releases":  func(v *yaml.Node, p string) { w.stringBool(v, p, &cfg.IncludePreReleases) },
		"commitish":             s(&cfg.Commitish),
		"header":                s(&cfg.Header),
		"footer":                s(&cfg.Footer),
		"filter-by-range":       s(&cfg.FilterByRange),
		"autolabeler": func(v *yaml.Node, p string) {
			cfg.HasAutolabeler = true
			cfg.Autolabeler = []Autolabel{}
			w.list(v, p, func(item *yaml.Node, ip string) { cfg.Autolabeler = append(cfg.Autolabeler, w.autolabel(item, ip)) })
		},
	})
	if len(w.errs) > 0 {
		return nil, &ValidationError{Origin: origin, Errors: w.errs}
	}
	return &cfg, nil
}
