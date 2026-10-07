<h1 align="center">MeuRPG</h1>

<p align="center"><a href="README.pt-BR.md">Português (Brasil)</a></p>

<p align="center">
  Where a D&amp;D 5e table prepares and plays its campaigns.<br>
  The Game Master runs the game, the players follow their sheets, and the rules do the math.
</p>

<p align="center">
  <a href="https://github.com/PuraFome/meuRPG/actions/workflows/backend.yml?query=branch%3Amain"><img alt="backend" src="https://github.com/PuraFome/meuRPG/actions/workflows/backend.yml/badge.svg?branch=main"></a>
  <a href="https://github.com/PuraFome/meuRPG/actions/workflows/web.yml?query=branch%3Amain"><img alt="web" src="https://github.com/PuraFome/meuRPG/actions/workflows/web.yml/badge.svg?branch=main"></a>
  <a href="https://github.com/PuraFome/meuRPG/actions/workflows/e2e.yml?query=branch%3Amain"><img alt="e2e" src="https://github.com/PuraFome/meuRPG/actions/workflows/e2e.yml/badge.svg?branch=main"></a>
</p>

<p align="center">
  <a href="backend/go.mod"><img alt="Go" src="https://img.shields.io/github/go-mod/go-version/PuraFome/meuRPG?filename=backend%2Fgo.mod&amp;logo=go&amp;logoColor=white&amp;label=Go"></a>
  <a href="web/package.json"><img alt="Angular" src="https://img.shields.io/github/package-json/dependency-version/PuraFome/meuRPG/%40angular%2Fcore?filename=web%2Fpackage.json&amp;logo=angular&amp;logoColor=white&amp;label=Angular&amp;color=DD0031"></a>
  <a href="proto/"><img alt="API: Connect + Protobuf" src="https://img.shields.io/badge/API-Connect%20%2B%20Protobuf-4B32C3"></a>
  <a href="docs/data.md"><img alt="Database: CockroachDB" src="https://img.shields.io/badge/database-CockroachDB-6933FF?logo=cockroachlabs&amp;logoColor=white"></a>
  <a href="docs/operations.md"><img alt="Infra: Cloud Run in São Paulo" src="https://img.shields.io/badge/infra-Cloud%20Run%20%C2%B7%20S%C3%A3o%20Paulo-4285F4?logo=googlecloud&amp;logoColor=white"></a>
</p>

<p align="center">
  <a href="e2e/tests/a11y.spec.ts"><img alt="Accessibility: axe, WCAG 2.1 AA" src="https://img.shields.io/badge/a11y-axe%20%C2%B7%20WCAG%202.1%20AA-1F6FEB"></a>
  <a href=".github/dependabot.yml"><img alt="Dependabot enabled" src="https://img.shields.io/badge/Dependabot-enabled-025E8C?logo=dependabot&amp;logoColor=white"></a>
  <a href="https://www.conventionalcommits.org/v1.0.0/"><img alt="Conventional Commits" src="https://img.shields.io/badge/Conventional%20Commits-1.0.0-FE5196?logo=conventionalcommits&amp;logoColor=white"></a>
  <a href="NOTICE"><img alt="Rules: SRD 5.1, CC BY 4.0" src="https://img.shields.io/badge/rules-SRD%205.1%20%C2%B7%20CC%20BY%204.0-555555"></a>
  <a href="LICENSE"><img alt="License: Apache 2.0" src="https://img.shields.io/github/license/PuraFome/meuRPG?label=license&amp;color=D22128"></a>
  <a href="https://github.com/PuraFome/meuRPG/commits/main"><img alt="Last commit" src="https://img.shields.io/github/last-commit/PuraFome/meuRPG/main?label=last%20commit"></a>
</p>

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/img/ficha-desktop-escuro.webp">
    <img alt="Pensantus, a level 3 Wizard, locked for the session: the ability medallions, the AC shield, hit points and the warning about armor without proficiency" src="docs/img/ficha-desktop-claro.webp" width="900">
  </picture>
</p>

## What it is

MeuRPG is a companion for one table's D&D 5e campaigns. The Game Master (*mestre*) prepares the world and runs the live session; players join by invite, follow their character sheet and act in roleplay (RP) and in combat within what the rules allow. The rules engine does the math, and the server is the authority: what is hidden never leaves it for a player. More in the [product vision](docs/product/vision.md).

It is a Go server that serves the Angular app and the API from the same origin, backed by CockroachDB and built to run on Cloud Run in São Paulo. The screens are in Portuguese for now; the English UI comes after the MVP. The old app is discontinued and documented in [Legacy app](docs/legacy-app.md).

## Current stage

**Pre-MVP.** Every feature planned for the MVP is built (Stages 1 to 10 of the [roadmap](docs/roadmap.md)). What comes next: hardening and a rehearsal before the first real table session, then internationalization (an English UI), then the post-MVP stories.

## What works

- **Sign-in and campaigns.** OpenID Connect sign-in (Google in production), server-side sessions of up to 30 days. The Game Master creates campaigns and invites (with an expiry and a number of uses, optionally needing approval); invitees join signed in or signing in on the way.
- **Characters.** The official sheet layout, computed by the rules engine from the SRD 5.1: modifiers, skills, AC, HP, spells and rule warnings. NPCs on a short sheet. Ability scores and HP can be rolled in the app. Sheets lock when a session starts.
- **Live session.** Players are notified when the session starts; HP, spell slots and hit dice update on screen when the Game Master corrects them. A summary with highlights closes the session.
- **Maps without spoilers.** Maps built from the gallery, with points of interest (battle, submap, RP scene), tokens, layers (walls, difficult terrain, cover, light), traps, treasures and per-player fog of war; hidden things never reach players. Printable at table scale.
- **Combat.** Initiative, turn order and movement on a 1.5 m grid (or without a grid, "theatre of the mind"). On their turn players see what they can do with action, bonus action, reaction and movement, and roll with the app's dice or physical ones. Circle movement, jumping, cover and opportunity attacks as in the official rules; the Game Master applies damage, conditions and death saves; a log tells the fight.
- **RP scenes, clues and puzzles.** Scene actions with skill checks and saves, clues and hooks, NPCs on stage with portraits, six kinds of puzzles with hints, split information and consequences.
- **Progression.** XP by combat, gold, milestone or free-form; guided level-up on the sheet.
- **Creatures.** The 334 SRD creatures as a bestiary, a character's familiar, undead, summoned animals and Wild Shape.
- **The table's own content and rules.** Custom spells, races, backgrounds, classes and subclasses; table rules (level-up HP, ability score methods, critical hits, hidden death saves); switches for what players may use.
- **Generators.** Dungeons (doors, stairs, numbered rooms), an encounter builder against the party's budget, and a treasure generator.
- **AI images.** Scene art, an isometric view and a textured map, made only from what the players have already seen.
- **Gallery and campaign document.** Image upload with metadata stripped; the Game Master's notes in Markdown.

## The look: the paper sheet

The screens mimic the official D&D 5e sheet: ability medallions, the AC shield, proficiency dots. The game number is the most visible thing on screen. Light and dark themes follow the operating system with AA contrast, and every screen works on a phone at the table. Principles, tokens and how a screen is designed and reviewed: [Design](docs/design.md).

<p align="center">
  <img alt="The Mirathel campaign on a phone, as the Game Master, light theme: the session in progress, Pensantus locked and the NPCs" src="docs/img/campanha-celular-claro.webp" width="300">
  &nbsp;&nbsp;
  <img alt="Pensantus's sheet on a phone, dark theme" src="docs/img/ficha-celular-escuro.webp" width="300">
</p>

## Stack

| Part | What it uses |
| --- | --- |
| Backend | Go, a modular monolith (`identity`, `authz`, `campaigns`, `rules`, `characters`, `play`, `maps`, and more) that is also the app's BFF |
| API | Protobuf + [Connect](https://connectrpc.com) in `proto/`; the TypeScript client in `web/` is generated from the same `.proto` files |
| Database | CockroachDB, with queries in sqlc and migrations in goose |
| Web | Angular 21 (standalone, zoneless) with Angular Material, in the [paper sheet](docs/design.md) look |
| Sign-in | OpenID Connect with PKCE; the browser only gets a `__Host-` cookie with an opaque session |
| Rules | A pure engine over the SRD 5.1, with effects in JSON and formulas in a sandbox |
| Infra | Cloud Run and CockroachDB on Google Cloud, in São Paulo |

The full design, with diagrams, is in [Architecture](docs/architecture.md).

## Running it locally

You need Docker, Go 1.27, Node 22 and `make`. The tools that only generate code or lint (buf, sqlc, golangci-lint) are listed in [CONTRIBUTING](CONTRIBUTING.md#local-environment).

```bash
make up      # CockroachDB, migrations, devidp and the app at http://localhost:8080
make test    # backend tests (go test -race)
make e2e     # UI tests (Playwright) and accessibility tests (axe)
make down    # tear everything down
```

On a Mac you can run everything without Docker: `make db-native-start`, then `make up LOCAL_STACK=native` (see [All native](CONTRIBUTING.md#everything-native-mac-optional)).

Open `http://localhost:8080` in Chrome and click "Entrar": sign-in goes to **devidp**, a test provider that exists only on your machine and in CI, and one click on "Mestre Teste" signs you in. Safari does not accept `Secure` cookies on `http://localhost`.

To work on screens only, with live reload, leave `make up` running and start Angular in dev mode at `http://localhost:4200`:

```bash
cd web
npm ci --ignore-scripts   # never runs third-party install scripts
npm start
```

All commands are in `make help` and in [CONTRIBUTING](CONTRIBUTING.md).

## Repository layout

| Folder | What it holds |
| --- | --- |
| `backend/` | The Go server: `cmd/` (the API, migrations, devidp and the SRD importer), `internal/<module>` and `migrations/` |
| `proto/` | The API contracts (`meurpg/<module>/v1/*.proto`) |
| `web/` | The Angular app |
| `e2e/` | The Playwright tests for each acceptance criterion and `a11y.spec.ts` |
| `deploy/local/` | The Docker Compose of the local environment |
| `docs/` | The documentation, with Mermaid diagrams ([index](docs/README.md)) |
| `.github/` | CI workflows, Dependabot and the PR template |
| `src/`, `server/` | The [legacy app](docs/legacy-app.md), discontinued |

## Quality

All code goes in through a PR with green CI. What each CI job checks is in [CONTRIBUTING](CONTRIBUTING.md#what-ci-checks).

- **Every acceptance criterion is a test:** `go test` for server-side rules, Playwright for what shows on screen, tagged with the story (`@MR-001`).
- **Accessibility:** [axe](https://github.com/dequelabs/axe-core) runs on the main screens, in light and dark themes, on desktop and phone; CI fails on any serious or critical WCAG 2.1 A and AA violation.
- **Contract and generated code:** `buf lint`, `buf format` and `buf breaking` on the `.proto` files; CI regenerates the buf and sqlc code and fails on any diff.
- **A real database in tests:** integration tests run against CockroachDB, the same image as the local environment.
- **Everything pinned:** actions by commit SHA, images by digest, npm packages at exact versions installed without scripts. Dependabot opens the PRs that keep this current every week, and `govulncheck` checks the Go dependencies.
- **Privacy:** the stricter of LGPD and GDPR, data item by data item, in [Privacy](docs/privacy.md). The app stores nothing in `localStorage` or `sessionStorage`.

## Documentation

| Document | Answers |
| --- | --- |
| [docs/README.md](docs/README.md) | The index of all documentation |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to run, test and open a PR |
| [Vision](docs/product/vision.md), [Stories](docs/product/stories.md), [Rules](docs/product/rules.md), [Glossary](docs/product/glossary.md) | What the app does, for whom, and how the game behaves in it |
| [Roadmap](docs/roadmap.md) | The stages, the MVP gate and what comes after |
| [Architecture](docs/architecture.md), [Data](docs/data.md) | How the system is built, where it runs, the tables |
| [Design](docs/design.md) | The app's look and how a screen is designed and reviewed |
| [Privacy](docs/privacy.md), [Operations](docs/operations.md) | The personal data we keep; deploy, secrets and costs |

The product docs and privacy are also available in Portuguese under [`docs/pt-BR/`](docs/pt-BR/README.md).

## Rules content and license

The rules come from the System Reference Document 5.1 (SRD 5.1), under the Creative Commons Attribution 4.0 license. The app is 5E compatible and uses no trademark of the publisher. The attribution the license requires, with the exact text, is in the [NOTICE](NOTICE) and on the app's "Créditos" page:

> This work includes material taken from the System Reference Document 5.1 ("SRD 5.1") by Wizards of the Coast LLC and available at https://dnd.wizards.com/resources/systems-reference-document. The SRD 5.1 is licensed under the Creative Commons Attribution 4.0 International License available at https://creativecommons.org/licenses/by/4.0/legalcode.

- The content is embedded in the binary, in `backend/internal/rules/srd51`, generated from the 5e-database (MIT) at a pinned commit. Nothing is fetched at runtime.
- Three tables come from the SRD 5.2.1 (the 2024 rules, also CC BY 4.0): the ability score methods, the encounter XP budget and the magic item values. The app labels each one "SRD 5.2.1 (regras de 2024)", and the NOTICE carries their attribution.
- The Portuguese names and the structured effects are ours. SRD descriptions stay in English for now.
- No text from books outside the SRD enters the repository: what a table uses from other books, it registers in its own words.
- How the engine works: [Architecture → rules module](docs/architecture.md#rules-module-rules-as-data). How to update the SRD: [CONTRIBUTING.md](CONTRIBUTING.md#rules-content-srd).

The fonts (Alegreya and Alegreya Sans, OFL 1.1) and icons (Material Symbols, Apache 2.0) are also in the [NOTICE](NOTICE), with licenses in [`third_party/licenses/`](third_party/licenses/).

MeuRPG is distributed under the [Apache License 2.0](LICENSE). Third-party content listed in the [NOTICE](NOTICE) keeps its own license: the SRD 5.1 and the SRD 5.2.1 (CC BY 4.0), the 5e-database data (MIT), the fonts (OFL 1.1) and the icons (Apache 2.0). Anyone who redistributes MeuRPG, with or without changes, includes the `LICENSE` and `NOTICE` files.

## Legacy app (discontinued)

The Angular app in `src/` and the NestJS server in `server/` are the old app. They receive no changes and both leave the repository after the MVP. What they did and how they ran is in [Legacy app](docs/legacy-app.md).
