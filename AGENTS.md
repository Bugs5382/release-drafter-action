# AGENTS.md - release-drafter-action

Guide for AI agents working in this repository. Pair with `CLAUDE.md` (the working agreement and
hook-enforced rules). Keep this file current when the build, layout, or public API changes.

## What this is

Drafts GitHub release notes from merged PRs: a Go rewrite of release-drafter v7 with piped-in config and a fixed first release.

It ships a GitHub Action (Docker runtime, `action.yml` plus `Dockerfile`) that builds a Go binary
from this checkout. Two things matter before changing it:

- **Status: implemented.** Every package (config, regex, pathmatch, semver, render, github,
  history, autolabel, action) is wired up: `cmd/action` dispatches draft and autolabel mode through
  `internal/action`, which the integration workflow (`job-docker-build.yaml`, running `uses: ./`
  with `dry-run: true` against this repository's own config) and `job-release-drafter.yaml`
  (running `uses: ./` for real, on every push to `main`) both exercise.
- **Parity is the contract.** Drafts must be byte-identical to release-drafter v7 for every
  existing config, except on a first release, which gets no warning banner, the full history, and
  `$RESOLVED_VERSION` pinned to `first-version` (default `1.0.0`) instead of v7's 0.0.0-plus-
  increment baseline, unless a `version`/`tag`/`name` input overrides it. `internal/action`'s own
  tests prove this end to end, not just at the render layer: a fake GitHub server serves the same
  fixture `internal/render`'s golden tests use, and the body `action.Run` produces through config
  load, the GraphQL client and history is asserted byte-identical to it.

## Using release-drafter-action

Consumers call `uses: Bugs5382/release-drafter-action@v1`. The public surface is `action.yml`:

- **Inputs:** every release-drafter v7 input, plus `config` (YAML text, takes precedence over
  `config-name`), `extends`/`extends-asset` (a shared config as a release asset at a pinned tag),
  `exclude-bots` (built-in default config only), `since` (first-release start point) and
  `first-version` (first-release resolved-version baseline, default `1.0.0`). `dry-run` makes no
  API writes.
- **Outputs:** the v7 set: `id`, `name`, `tag_name`, `body`, `html_url`, `upload_url`,
  `resolved_version`, `major_version`, `minor_version`, `patch_version`. On a dry run, or when
  there is nothing to draft, the release-only fields (`id`, `html_url`, `upload_url`) are empty and
  the rest come from the rendered payload.
- **Mode:** picked from `GITHUB_EVENT_NAME`. push, `workflow_dispatch`, `release` and anything else
  (a local run) draft; `pull_request` and `pull_request_target` autolabel (add labels only, never
  remove).
- Inputs reach the binary as `--name=value` flags, always present even when empty. The token
  reaches it as `GITHUB_TOKEN` in the environment, never as a flag, and is never logged.
- Adding or renaming an input means changing `action.yml` args, the flag set in
  `cmd/action/main.go` and `internal/action.Flags`/`parseFlags` together.

## Layout

- `action.yml` - action metadata: inputs, outputs, branding, Docker runtime and flag args.
- `Dockerfile` - builds `./cmd/action` with a digest-pinned `golang:1.26-alpine`, runs it on `alpine`.
- `cmd/action/` - entry point: flag parsing, reads the process environment, calls
  `internal/action.Run`, `os.Exit`s its result. Nothing here is tested beyond that glue; the
  actual behavior lives in `internal/action` and is tested there.
- `internal/action/` - wires every other package into the two modes: `draft.go` (config load,
  history, render, upsert) and `autolabel_mode.go` (config load, the event payload, `AddLabels`).
  Owns `$GITHUB_OUTPUT` (`outputs.go`) and the event payload (`event.go`). Its own tests run the
  whole thing against a fake GitHub server; `fake_github_test.go` and `hub_fixture_test.go` hold
  that harness and the v7-parity fixture.
- `internal/config/` - YAML loading, validation of every v7 key, defaults, input overrides.
- `internal/github/` - go-github and GraphQL client with retries.
- `internal/history/` - finds the last release and any existing draft, the commit range (last
  release or first release plus `since`), the pull requests merged in it, and the create/update/
  skip-if-empty upsert of the draft itself.
- `internal/render/` - pure rendering: categories, templates, replacers, version resolver.
- `internal/regex/` - JavaScript regex literals on regexp2 (ECMAScript) with match timeouts.
- `internal/autolabel/` - branch, title, body and files rules to labels.
- `internal/logging/` - go-log setup.

## Build, test, lint

- Build: `task build` (`go build ./...`); the action binary: `task build:action`; the image:
  `task docker` (CI builds it in `job-docker-build.yaml`)
- Run locally: `task run -- --dry-run=true` (sets `LOG_LEVEL=trace LOG_FORMAT=console`)
- Test: `task test` (`go test ./...`)
- Lint: `task lint` (gofmt check, golangci-lint, yamllint); workflows: `actionlint`
- License headers: `task license` verifies ISC headers with golic, `task license:fix` adds them

## Logging

Follow the logging rules in `CLAUDE.md`. In short:

- Log generously: entry and exit of significant operations, decisions and branches, retries, state
  changes, external calls (target, duration, outcome), and every error with its context.
- Levels: `trace` for step-by-step detail, `debug` for flow, `info` for lifecycle, `warn` and
  `error` for problems. The environment filters the volume, so err on the side of too much.
- Environments: local dev `trace` with `LOG_FORMAT=console` (never JSON), dev cluster `debug`,
  qa/staging `info`, production `error`. Every cluster environment logs JSON. Set levels through
  `LOG_LEVEL` and `LOG_FORMAT`, never in code; local settings live in the run target or
  `.env.example`.
- Never log secrets, tokens, or personal data, not even at `trace`. Log an opaque or keyed ID.

## Conventions and gotchas

- See `CLAUDE.md` for the branch/commit/PR rules; they are enforced by the git hooks in
  `.claude/hooks` (run `bash .claude/hooks/install.sh` once per clone).
- Open every PR as a draft. CI skips drafts, so run the full checks locally, push once they pass,
  and mark the PR ready when the work is finished; see CLAUDE.md "CI and Actions minutes".
- ISC license, holder `2026 Shane & Contributors`, like changelog-updater-action (not the
  template's MIT).
- No emoji in Go source or `action.yml`; the PR hygiene check blocks them outside Markdown and
  `.github`.
- Regexes come from JavaScript configs. Use regexp2 in ECMAScript mode with a timeout, never the
  standard `regexp` package, which lacks lookaround.
- Out of scope: `_extends`, reading config from another repo, release asset uploads, non-Linux
  runners.
- The repo is private until the first Marketplace release. The owner publishes every release by
  hand and moves the v1 tag.
