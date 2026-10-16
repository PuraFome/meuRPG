# CI

All code enters through a pull request with green CI; the workflows are in `.github/workflows/`.
This page says what each one checks, which gates only move one way, and how dependencies are pinned and updated.
How to run the same checks locally is in [CONTRIBUTING.md](../CONTRIBUTING.md); the tests themselves are in [Testing](testing.md).

## Workflows

| Workflow | Runs on | Jobs |
| --- | --- | --- |
| `backend` | PRs and pushes to `main` touching `backend/`, `proto/`, `web/`, the `Makefile`, `.node-version` or the Go CI files; nightly; manual | `changes`, `proto`, `go`, `go-db` (nine groups plus a gate), `go-coverage`, `report` |
| `web` | `web/`, `.node-version`, the workflow | `test and build`, `coverage` |
| `e2e` | `backend/`, `proto/`, `web/`, `e2e/`, `deploy/local/`, the `Makefile`, `.node-version`, the workflow; never for a Markdown-only change; nightly; manual | `plan`, `lint and typecheck`, `images`, `playwright` shards, `report` |
| `codeql` | PRs touching `backend/`, `web/src/`, `e2e/` or workflows; every push to `main`; weekly | Go (built from `backend/`), JavaScript/TypeScript and GitHub Actions, with the `security-extended` queries; findings go to the Security tab |
| `workflows-lint` | Changes under `.github/` | `actionlint` and `zizmor`, at versions pinned by number and sha256 |

- A new run of the same PR cancels the previous one. A push to `main` never cancels a run in progress, but GitHub keeps only one waiting run per workflow, so in a burst of merges an intermediate commit may get no run of its own. Deploy only a commit whose own run passed.
- The nightly and manual runs ignore the path filters and test everything. The `backend` workflow's `changes` job lets a pull request that touches no Go, proto or Go-CI file skip the Go jobs.
- Every job has a `timeout-minutes`, so a stuck test cannot burn the six-hour default.
- **When a run outside a pull request fails** (`main` or the nightly), the `report` job of `backend` and `e2e` opens a GitHub issue labelled `ci-red`, or comments on the one already open.

## Backend

| Job | Checks |
| --- | --- |
| `proto` | `buf lint`, `buf format`, `buf breaking` against `main`, and that `buf generate` and `make sqlc` leave the committed generated code unchanged |
| `go (lint, vulncheck)` | `golangci-lint` as a ratchet, the [complexity totals](#complexity-totals) and `govulncheck` (version pinned). It runs no `go test`: `go-db` runs every package |
| `go-db <group>` | `go test -race -covermode=atomic -coverpkg=./...` for one group of packages against its own in-memory CockroachDB (the image `COCKROACH_IMAGE`, pinned by digest, the same as `compose.yaml`), with `MEURPG_REQUIRE_DB=1`. Each package has a 25-minute `-timeout` inside the job's 30 |
| `go (integration tests against CockroachDB)` | Waits for every group; the stable check for a branch protection rule |
| `go (coverage)` | Merges the groups' profiles and enforces the [coverage floor](#coverage-floors) |

**The groups** are defined in `.github/scripts/go-db-shard.sh`: `play-1` to `play-3`, `maps`, `characters`, `pure`, `leak`, `camp` and `rest`.
The heavy packages are named by hand and `rest` holds everything else, so a new package lands in `rest` and is never skipped.
The `play` package is split by test: each part runs the tests whose name hashes (`cksum`) to it, so a new test lands in a part by itself and adding one never moves the others.
`go-db-shard.sh --check` (run by every group) fails unless the groups together are exactly `go list ./...` and the play parts together are exactly the tests `go test -list` prints.
If a group nears its timeout, move a package between groups in that script.
The architecture test (`internal/archtest`) runs in `rest`.

## Go linters and the ratchet

New findings fail the build; old ones do not.
`backend/.golangci.yml` enables more linters than the existing code satisfies, so CI judges only the lines a change adds: on a pull request, the diff against the merge base (`--new-from-merge-base=origin/<base>`); on a push to `main`, the pushed commits (`--new-from-rev=<before>`).
The job checks out the whole history for that.
Fixing old findings is welcome and never required.

- `make lint` runs `buf lint`, `buf format` and `golangci-lint` (the version in the `Makefile`, which must equal the `golangci-lint-action` in `backend.yml` and `golangci_version` in `.github/scripts/go-complexity.sh`) as the same ratchet, against `origin/main` (`git fetch origin main` first; `LINT_BASE=<ref>` changes the base).
- `make lint-all` runs them on the whole backend with every finding, to pick what to clean up next.
- The linters, their settings and the reason for each one are in `backend/.golangci.yml`. Highlights: `sloglint` (a fixed message per call site, `snake_case` keys; see [Architecture](architecture/logs.md#logs)), `nolintlint` (every `//nolint` names its linter, says why and still suppresses something), `gocyclo` and `gocognit` at 15, `maintidx` at 20, `mnd` (a bare number becomes a named `const` with a comment) and `goconst`.
- `wrapcheck`, `gochecknoglobals`, `funlen`, `nilnil` and `exhaustive` are off on purpose. `wrapcheck` is off because authorization errors are `*connect.Error` and stay unwrapped for `rpclog`'s type switch.

### Complexity totals

The number of functions over each limit, and each one's score, may only go down.
The ratchet above never sees a change to the body of an old function, so `.github/scripts/go-complexity.sh` measures every hand-written, non-test Go file of `backend/` (generated files are left out) and compares the lists with `.github/go-complexity-baseline.txt`.
The three lists use the limits of `backend/.golangci.yml`: cyclomatic complexity above 15 (gocyclo), cognitive complexity above 15 (gocognit) and maintainability index below 20 (golangci-lint's `maintidx`).
It needs `jq` and runs in seconds.

- **Worse fails:** a function that enters a list, or whose score gets worse even when the count stays the same. The message names it with its file and line.
- **Better fails too, until the baseline is lowered:** a function that improved or left a list. Run `.github/scripts/go-complexity.sh --update` and commit the smaller baseline in the same PR, so the improvement cannot be given back.
- A function is named by its package directory and its name (`internal/play (*Service).takeBack`), never by its line, so moving it to another file of the same package changes nothing. A rename leaves the list under the old name and enters under the new one; `--update` records it.
- Never grow the baseline. If a function really has to get worse, `--update` accepts it, and the PR says why. A `//nolint` hides a function from golangci-lint, not from the totals.
- A pure move of an old function, or a change to its `func` line, trips the ratchet's `gocyclo`, `gocognit` and `maintidx` on it: split it first, or for a move that changes nothing add `//nolint:gocyclo,gocognit,maintidx // moved unchanged; held by the complexity totals` on its `func` line.

### Coverage floors

`.github/scripts/go-coverage.sh` merges the groups' profiles (a block is covered if any group ran it), prints the total and a per-package table in the job summary, and fails if a package in `FLOOR_PACKAGES` is under `FLOOR_PERCENT` (90).
The floor packages are the pure rules packages: `rules`, `combat`, `grid`, `vision`, `dungeon`, `encounter` and `puzzle`.
Other packages are reported without a floor.
Each group uploads its profile as the artifact `go-cover-<group>`.
Locally:

```bash
cd backend
go test -covermode=atomic -coverpkg=./... -coverprofile=c.out ./internal/rules/...
../.github/scripts/go-coverage.sh c.out
```

The web floors are `coverageThresholds` in `web/angular.json` (lines, statements, branches and functions).
The `web / coverage` job is blocking: raise the floors when the numbers rise, never lower them to get green.
The report goes to the job summary and the `web-coverage` artifact.

## Web and e2e linters

The web and e2e linters use the same ratchet: new violations fail, old ones are a recorded backlog.
ESLint's bulk suppressions list the old violations as a count per file and rule in `web/eslint-suppressions.json` and `e2e/eslint-suppressions.json`, so CI fails only when a file gets a violation it did not have.

- Never grow a suppressions file. Fix the new violation; if it is really unavoidable, write `// eslint-disable-next-line <rule> -- why` on that line and explain it in the PR.
- When you touch a suppressed spot, fix it, run `npx eslint --prune-suppressions` and commit the smaller file.
- `eslint --suppress-rule <rule>` is only for turning on a new rule: enable it, record what exists, fix it over time.
- `web/eslint.config.js`: typescript-eslint with the type-aware promise rules and no `any`; angular-eslint with the template and accessibility rules, OnPush, standalone components and `inject()`; `complexity` 15 and `max-depth` 4; no magic numbers in app code (-1, 0, 1, 2, the radix 10, 100 and 1000, default values, enum members, readonly fields and array indexes are allowed; specs are exempt); a `setTimeout(` in a `*.spec.ts` is forbidden (see [Testing](testing.md#web-unit-tests)); and no `.replace(/ /g, ' ')` (a space replaced by itself: write a non-breaking space as ` `, which CodeQL also flags as `js/identity-replacement`).
- **Layers and imports (dependency-cruiser).** `cd web && npm run depcruise` applies `web/.dependency-cruiser.cjs` to `web/src/app` (specs and `src/gen` are not cruised): no import cycles; `core/` imports neither `pages/` nor `shell/`; `shared/` does not import `pages/`; a page (a direct child folder of `pages/`) does not import another page (its own sub-folders are fine). Type-only imports count. `web/.dependency-cruiser-known-violations.json` is empty and must stay empty: move what two layers or pages share down to `core/` or `shared/`. dependency-cruiser is a dev dependency used only by this check.
- **Formatting.** `cd web && npm run format:check` (Prettier, `web/.prettierrc`) checks TypeScript, SCSS and JSON. Angular templates are left out, because Prettier's HTML printer changes whitespace between elements and text, which changes what renders when CSS makes an element inline; format templates by hand. `.git-blame-ignore-revs` lists the one formatting commit (`git config blame.ignoreRevsFile .git-blame-ignore-revs`, once, locally).
- **e2e.** `cd e2e && npm run lint:install && npm run lint` (see [Testing](testing.md#end-to-end-tests-playwright)). `expect-expect` is off in `e2e/`: the tests assert through helpers.

### The web bundle budget

`web/angular.json` fails the production build when the initial chunks pass 500 kB (the "Initial total" line, raw size) and warns from 490 kB; a component's styles warn from 4 kB and fail from 8 kB.
What a first screen needs stays in the initial chunks (Angular, the router, protobuf-es and Connect, the shell); everything else is a lazy route, a `@defer` or a dynamic `import()`.
To see what grew, `cd web && npx ng build --stats-json` writes `dist/web/stats.json` (esbuild's metafile): follow the `imports` of `src/main.ts` to the module.
A new import in `app.ts`, `app.config.ts`, `app.routes.ts` or `shell/` is the usual cause, in particular a generated `*_pb` file or `@angular/cdk/overlay`.

## Web

`web / test and build` runs, in `web/`: `npm ci --ignore-scripts`, `npm run lint`, `npm run format:check`, `npm run depcruise`, `npm test` and `npm run build`.
`web / coverage` runs `ng test --coverage` against the floors in its own job so it cannot cancel or skip the build.

## End to end

The `e2e` workflow runs the Playwright suite against the local stack.

- `plan` chooses the shards: three plain shards (`--grep-invert @a11y`) always, and six accessibility shards (`--grep @a11y`) on `main`, nightly, manual runs, and pull requests that touch `web/`, `e2e/`, `deploy/local/` or the workflow.
- `lint and typecheck` needs no Docker, so a mistake fails in about a minute.
- `images` builds the production image and the devidp image once (with the GitHub Actions layer cache) and hands both over as an artifact; each shard runs `docker load`, so a broken Dockerfile fails before any shard.
- Each shard starts the stack with `docker compose -f deploy/local/compose.yaml -f deploy/local/compose.ci.yaml -f deploy/local/compose.ci-images.yaml up --no-build` (`compose.ci.yaml` keeps the database in memory; `compose.ci-images.yaml` points at the loaded images), checks on one shard that the production image does not contain the devidp, and runs the tests on Chromium. A failed shard shows the stack logs and keeps its Playwright report as the artifact `playwright-report-<n>`.

## Pinning and dependencies

- **Every GitHub action is pinned by commit SHA**, not by tag: whoever controls an action can move a tag to malicious code, but cannot change a SHA. The `# vX.Y.Z` comment is for humans.
- **Every job's machine is pinned to an Ubuntu version** (`runs-on: ubuntu-26.04`), never `ubuntu-latest`. Changing it is done in every job at once, tested in a PR first.
- **Dockerfiles pin their images by digest**, and must match `backend/go.mod` (Go) and `.node-version` (Node).
- **npm installs never run third-party install scripts** (`npm ci --ignore-scripts`).
- **Workflow permissions** are `contents: read`; a job that needs more (`report`) asks for it by name.

Dependabot (`.github/dependabot.yml`) opens weekly PRs for the actions, the Go modules, the npm packages of `web/`, `e2e/` and `e2e/lint/`, the Docker base images and the CockroachDB image of `compose.yaml`.
Each new version waits 7 days (`cooldown`), so a compromised release can be pulled before it reaches us; a security update skips the wait.
Minor and patch updates arrive together in one PR per group.
Majors that need a deliberate step are ignored by Dependabot and done by hand:

- a major of Angular, TypeScript, vitest or angular-eslint in `web/`, which comes with `ng update` and its migrations (the builder accepts one major at a time);
- a major of `@types/node`, which follows the Node version the code runs on;
- a new Node or Go minor in the images, which changes together with `.node-version` and `go.mod`;
- a new CockroachDB release line: only "Regular" lines (26.2, 26.4...), never "Innovation" ones, which have no long-term support. It changes `compose.yaml`, `compose.ci.yaml`, `COCKROACH_IMAGE` in `backend.yml`, the `Makefile` and the Homebrew formula together.

A CockroachDB patch PR only changes `compose.yaml`: change `COCKROACH_IMAGE` and the `image` of `deploy/local/compose.ci.yaml` in the same PR, because Dependabot ignores the copy in `compose.ci.yaml` (it has another name there).
Those two pull the same digest from Cockroach Labs' registry on Google Cloud, and the e2e image build pulls every Docker Hub image through Google's mirror (`mirror.gcr.io`), because GitHub's runners share Docker Hub's anonymous pull limit.

## Reading a failure

Never rerun first. Read the annotations, the logs, the Playwright report and the trace, decide whether the application, the test or the infrastructure failed, fix it, and rerun only as confirmation.
Before merging, read CodeQL's comments on the PR and the open code-scanning alerts: a green check ignores warnings.
