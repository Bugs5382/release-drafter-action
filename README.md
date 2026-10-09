# release-drafter-action 📝

> 📝 Drafts GitHub release notes from merged pull requests: a Go rewrite of release-drafter v7 with piped-in config and a fixed first release.

release-drafter v7 has two rough edges. With no earlier published release it drafts the wrong
range, adds a "no previous release" warning block to the notes, and resolves the version from a
0.0.0 baseline, so it drifts to whatever the first batch of pull requests happens to bump (often
`v0.1.0`, not a sensible first release). It also reads its config only from a file on the default
branch. 🔌 **This action takes the config from any of four sources**
(an input, a release asset at a pinned tag, a local file or a built-in default), so one org
variable, or one shared release, can serve every repo.

## ✨ Highlights

- 🎯 **Full v7 parity**: every v7 config key, input and output, with drafts byte-identical to v7's.
- 🌱 **Clean first release**: every merged PR reachable from the target, no warning banner, an
  optional `since` start point, and `first-version` (default `1.0.0`) instead of a 0.0.0 baseline.
- 📥 **Four config sources, one used at a time**: the `config` input, `extends` (another
  repository's config at a branch, tag or SHA), the local `config-name` file, or a built-in default.
- 🧩 **JavaScript-compatible regexes**: lookahead, lookbehind and backreferences, with a timeout on every match.
- 🏷️ **Autolabeler built in**: on pull request events it adds labels from the same config, and never removes any.
- 🧪 **Dry run**: prints the rendered body and outputs without writing anything.

## 🚀 Usage

The mode comes from the event: push, `workflow_dispatch` and `release` update the draft, while
`pull_request` and `pull_request_target` run the autolabeler. One workflow covers both:

```yaml
name: Release Drafter
on:
  push:
    branches: [main]
  pull_request:
    types: [opened, reopened, synchronize, edited]
permissions:
  contents: write
  pull-requests: write
jobs:
  draft:
    name: 📝 Draft release
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4
      - name: Draft release notes
        uses: Bugs5382/release-drafter-action@v1
```

### Config sources

Exactly one source is used; nothing is merged between them. The first one that is set wins:

1. **`config`** - YAML (or JSON) text, for example an org variable so one config serves every repo:

   ```yaml
   - uses: Bugs5382/release-drafter-action@v1
     with:
       config: ${{ vars.RELEASE_DRAFTER_CONFIG }}
   ```

2. **`extends`** - `owner/repo@ref`, where `ref` is a branch, tag or SHA of another repository:

   ```yaml
   - uses: Bugs5382/release-drafter-action@v1
     with:
       extends: my-org/release-drafter-config@v1.0.0
       # extends-asset: release-drafter.yml   # default; set only if the file or asset uses another name
   ```

   A tag with a published release carrying `extends-asset` as an asset is downloaded and checked
   against the sha256 digest GitHub reports. Any other ref - a branch, a SHA, or a tag with no
   matching release asset - reads `.github/<extends-asset>` from that repository at that ref
   instead, with no digest to verify: trust it the same way `actions/checkout` would. A branch (or
   an unpinned tag) follows every change made to it; pin a tag or a full commit SHA for stability.

3. **`config-name`** - a file under `.github` on the checked-out ref (so `actions/checkout` has to
   run first); unlike v7, that ref does not have to be the default branch:

   ```yaml
   - uses: actions/checkout@v4
   - uses: Bugs5382/release-drafter-action@v1
     with:
       config-name: release-drafter.yml   # the default; set to pick another file
   ```

4. **Built-in default** - nothing set, and no `.github/release-drafter.yml` in the checkout (or no
   checkout at all): a Conventional Commits config drafts the notes. `exclude-bots: true` drops
   bot-opened pull requests from it; the input is ignored for every other source.

## ⚙️ Inputs

Every release-drafter v7 input is supported, plus `config`, `extends`, `extends-asset`,
`exclude-bots`, `since` and `first-version`.

| Input | Description |
|---|---|
| `config` | Config as YAML text. Takes precedence over `extends` and `config-name`. |
| `extends` | Shared config: `owner/repo@ref` (a branch, tag or SHA). A tag with a matching release asset is digest-verified; any other ref reads the file straight from the repository. Takes precedence over `config-name`. |
| `extends-asset` | Release asset name `extends` downloads at a pinned tag, or the file name read from `.github/` at any other ref. Default `release-drafter.yml`. |
| `config-name` | Config file under `.github` on the checked-out ref. Default `release-drafter.yml`. |
| `exclude-bots` | Drop pull requests opened by a bot account. Only applies to the built-in default config. |
| `token` | GitHub token. Default `github.token`. |
| `name` | Release name, overrides `name-template`. |
| `tag` | Release tag, overrides `tag-template`. |
| `version` | Release version, overrides the resolved version. |
| `publish` | Publish the release instead of leaving a draft. |
| `latest` | Mark the release as latest. |
| `prerelease` | Draft a prerelease. |
| `prerelease-identifier` | Prerelease identifier (`alpha`, `beta`, `rc`). |
| `include-pre-releases` | Count prereleases when finding the last release. |
| `commitish` | Release target branch, SHA or ref. |
| `header` | Text added before the body. |
| `footer` | Text added after the body. |
| `filter-by-range` | Semver range the last release must satisfy. |
| `since` | First-release start point: a tag, SHA or date. |
| `first-version` | Resolved version for a first release (no version/tag/name override), instead of incrementing from 0.0.0. Default `1.0.0`. |
| `dry-run` | Print the body and outputs, write nothing. |

## 📤 Outputs

The same as v7: `id`, `name`, `tag_name`, `body`, `html_url`, `upload_url`, `resolved_version`,
`major_version`, `minor_version` and `patch_version`. On a dry run, or when there are no merged
pull requests to draft, the release-only fields (`id`, `html_url`, `upload_url`) are empty and the
rest still report what was (or would have been) rendered.

## 🔁 Migrating from release-drafter v7

1. Store your `.github/release-drafter.yml` contents in an org or repo variable, for example
   `RELEASE_DRAFTER_CONFIG`, or publish it as a release asset and use `extends` instead.
2. Replace `release-drafter/release-drafter@v7` with `Bugs5382/release-drafter-action@v1` and add
   `config: ${{ vars.RELEASE_DRAFTER_CONFIG }}` (or `extends`).
3. Drop the separate `release-drafter/release-drafter/autolabeler` step: the same action labels
   pull requests when it runs on a pull request event.

A local `.github/release-drafter.yml` keeps working as the fallback. `_extends` (v7's own config
key) and reading config from another repository through `config-name` (`owner/repo:path`,
`github:`) are not supported; use the `extends` input instead.

## 🚦 Release candidates

The recommended prerelease flow is `rc` only: `rc.1`, `rc.2`, ... until the line is stable, then
the plain release. Skip alpha and beta stages; they add steps without adding information an `rc`
doesn't already carry.

A first release can start directly on `rc.1` by combining `first-version` with `prerelease` and
`prerelease-identifier`:

```yaml
on:
  push:
    branches: [main]
  workflow_dispatch:
    inputs:
      publish:
        description: Publish the release instead of leaving it a draft
        type: boolean
        default: false
permissions:
  contents: write
jobs:
  draft:
    name: 📝 Draft release
    runs-on: ubuntu-latest
    steps:
      - uses: Bugs5382/release-drafter-action@v1
        with:
          first-version: 0.1.0
          prerelease: true
          prerelease-identifier: rc
          publish: ${{ inputs.publish || false }}
```

Every push to `main` redrafts `v0.1.0-rc.1`; publishing it (`publish: true`) and merging more pull
requests bumps the next draft to `v0.1.0-rc.2`, `v0.1.0-rc.3` and so on, each covering only what
merged since the published rc.

To promote the line to the real release, drop `prerelease` and `prerelease-identifier` and add
`include-pre-releases` for that one run, so the resolver still anchors on the published rc instead
of starting over from `first-version`:

```yaml
      - uses: Bugs5382/release-drafter-action@v1
        with:
          include-pre-releases: true
          publish: true
```

The draft comes out as the plain `v0.1.0` - the prerelease dropped, not bumped again on top of it.
Setting `prerelease-identifier` on this run would switch `prerelease` back on (v7's own
identifier-implies-prerelease behavior, with a warning), so leave it unset for the promotion.

## 🛠 Develop

```bash
task build        # go build ./...
task build:action # build the action binary into bin/
task run -- --dry-run=true   # run locally with trace console logs
task docker       # build the action image
task test         # go test ./...
task lint         # gofmt check + golangci-lint + yamllint
task license      # verify ISC headers (golic); task license:fix adds them
```

## ⚖️ License

ISC (c) 2026 Shane & Contributors
