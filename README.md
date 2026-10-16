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
  <a href="e2e/tests/a11y.spec.ts"><img alt="Accessibility: axe, WCAG 2.1 AA" src="https://img.shields.io/badge/a11y-axe%20%C2%B7%20WCAG%202.1%20AA-1F6FEB"></a>
  <a href="NOTICE"><img alt="Rules: SRD 5.1, CC BY 4.0" src="https://img.shields.io/badge/rules-SRD%205.1%20%C2%B7%20CC%20BY%204.0-555555"></a>
  <a href="LICENSE"><img alt="License: Apache 2.0" src="https://img.shields.io/github/license/PuraFome/meuRPG?label=license&amp;color=D22128"></a>
</p>

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/img/ficha-desktop-escuro.webp">
    <img alt="Pensantus, a level 3 Wizard, locked for the session: the ability medallions, the AC shield, hit points and the warning about armor without proficiency" src="docs/img/ficha-desktop-claro.webp" width="900">
  </picture>
</p>

## What it is

MeuRPG is a companion for one table's D&D 5e campaigns.
The Game Master (*mestre*) prepares the world and runs the live session.
Players join by invite, follow their character sheet and act in roleplay (RP) and in combat within what the rules allow.
The rules engine does the math, and the server is the authority: what the Game Master hides never reaches a player.
More in the [product vision](docs/product/vision.md).

The screens are in Portuguese; the English UI comes after the MVP.
What is built and what comes next is in the [roadmap](docs/roadmap.md).

## What a table can do

Every item links to the stories with their acceptance criteria.

- **Sign in and run campaigns.** The Game Master creates a campaign and invites players, who join signed in or signing in on the way ([MR-001](docs/product/stories.md#mr-001-create-a-campaign), [MR-002](docs/product/stories.md#mr-002-generate-an-invite), [MR-003](docs/product/stories.md#mr-003-join-through-the-invite)). A whole campaign can be exported and imported ([MR-050](docs/product/stories.md#mr-050-campaign-package-export-and-import)).
- **Characters.** The official sheet layout, computed from the SRD 5.1, with NPCs, a sheet that locks when a session starts, reserved characters and the Game Master's approval ([MR-004](docs/product/stories.md#mr-004-character-sheet-in-the-pdf-format), [MR-005](docs/product/stories.md#mr-005-create-npcs), [MR-006](docs/product/stories.md#mr-006-locked-sheet), [MR-024](docs/product/stories.md#mr-024-approve-the-character-from-the-invite), [MR-049](docs/product/stories.md#mr-049-reserved-characters-and-claim-links)).
- **The live session.** Players follow the session on their phones, with the Game Master's choices of what to show, and a summary closes it ([MR-011](docs/product/stories.md#mr-011-start-the-session), [MR-012](docs/product/stories.md#mr-012-follow-the-session), [MR-028](docs/product/stories.md#mr-028-show-an-image-to-the-players), [MR-032](docs/product/stories.md#mr-032-combat-highlights)).
- **Maps without spoilers.** Points of interest, tokens, layers, traps, treasure and per-player fog of war, printable at table scale ([MR-008](docs/product/stories.md#mr-008-points-of-interest), [MR-009](docs/product/stories.md#mr-009-map-without-spoilers), [MR-033](docs/product/stories.md#mr-033-print-the-map-with-the-grid), [MR-035](docs/product/stories.md#mr-035-traps), [MR-036](docs/product/stories.md#mr-036-fog-of-war-by-sight)).
- **Combat.** Initiative, turns and movement on a grid or in the theatre of the mind. On their turn players see what they can do and roll with the app's dice or physical ones ([MR-013](docs/product/stories.md#mr-013-turn-order), [MR-014](docs/product/stories.md#mr-014-your-turn), [MR-034](docs/product/stories.md#mr-034-special-movement), [MR-048](docs/product/stories.md#mr-048-cast-spells-outside-combat)).
- **RP, clues and puzzles.** Scene actions with skill checks, hooks and clues, players' notes, NPCs on stage and puzzles ([MR-015](docs/product/stories.md#mr-015-rp-scene-actions), [MR-029](docs/product/stories.md#mr-029-scene-hooks-and-clues), [MR-030](docs/product/stories.md#mr-030-player-notes), [MR-031](docs/product/stories.md#mr-031-npcs-in-the-scene), [MR-038](docs/product/stories.md#mr-038-puzzles)).
- **Progression.** XP by combat, gold, milestone or free-form, and a guided level-up on the sheet ([MR-016](docs/product/stories.md#mr-016-award-xp), [MR-040](docs/product/stories.md#mr-040-level-up-from-the-sheet), [MR-041](docs/product/stories.md#mr-041-treasure-and-xp-by-gold)).
- **Creatures and spells.** The SRD bestiary, the character's creatures and spell look-up ([MR-037](docs/product/stories.md#mr-037-creatures-of-the-character), [MR-042](docs/product/stories.md#mr-042-bestiary), [MR-045](docs/product/stories.md#mr-045-look-up-spells)).
- **The table's own content and rules.** Custom classes, races, backgrounds and spells, and the switches for what players may use ([MR-025](docs/product/stories.md#mr-025-register-table-content)).
- **Generators.** Dungeons, encounters against the party's budget and treasure ([MR-010](docs/product/stories.md#mr-010-generate-dungeons), [MR-043](docs/product/stories.md#mr-043-generate-encounters), [MR-044](docs/product/stories.md#mr-044-generate-treasure)).
- **Images.** An image gallery, the campaign document, and AI-generated art made only from what the players have already seen ([MR-018](docs/product/stories.md#mr-018-campaign-document), [MR-019](docs/product/stories.md#mr-019-image-gallery), [MR-039](docs/product/stories.md#mr-039-ai-generated-images-for-dungeons-and-scenes)).

## The look: the paper sheet

The screens mimic the official D&D 5e sheet: ability medallions, the AC shield, proficiency dots.
Light and dark themes follow the operating system, and every screen works on a phone at the table.
Principles, tokens and how a screen is designed: [Design](docs/design/README.md).

<p align="center">
  <img alt="The Mirathel campaign on a phone, as the Game Master, light theme: the session in progress, Pensantus locked and the NPCs" src="docs/img/campanha-celular-claro.webp" width="300">
  &nbsp;&nbsp;
  <img alt="Pensantus's sheet on a phone, dark theme" src="docs/img/ficha-celular-escuro.webp" width="300">
</p>

## Running it locally

You need Docker, Go, Node and `make`.
The versions and the other tools are in [CONTRIBUTING](CONTRIBUTING.md#tools).

```bash
make up      # CockroachDB, migrations, devidp and the app at http://localhost:8080
make test    # backend tests
make e2e     # UI and accessibility tests
make down    # tear everything down
```

Open `http://localhost:8080` in Chrome and click "Entrar".
Sign-in goes to the devidp, a test provider that exists only on your machine and in CI, and one click on "Mestre Teste" signs you in.
Native options, other sign-in providers and every variable are in [Local development](docs/development.md).

## Stack

| Part | What it uses |
| --- | --- |
| Backend | Go, a modular monolith that also serves the app |
| API | Protobuf and [Connect](https://connectrpc.com); the TypeScript client is generated from the same `.proto` files |
| Database | CockroachDB, with sqlc queries and goose migrations |
| Web | Angular with Angular Material, in the [paper sheet](docs/design/README.md) look |
| Sign-in | OpenID Connect with PKCE and server-side sessions |
| Rules | A pure engine over the SRD 5.1, with effects as data |
| Infra | Cloud Run and CockroachDB on Google Cloud, in São Paulo |

How it is built: [Architecture](docs/architecture/README.md).
How it is tested and checked in CI: [Testing](docs/testing.md) and [CI](docs/ci.md).

## Documentation

The full index is in [docs/README.md](docs/README.md).
The product docs and privacy are also in Portuguese: [docs/pt-BR/](docs/pt-BR/README.md).
How to contribute: [CONTRIBUTING.md](CONTRIBUTING.md).
The data the app keeps: [Privacy](docs/privacy.md).

The first app, an Angular front end with a NestJS API, is discontinued and its code is gone; its audit is in the [archive](docs/archive/legacy-code-quality.md).

## License and credits

MeuRPG is distributed under the [Apache License 2.0](LICENSE).
The rules come from the System Reference Document 5.1 (SRD 5.1) under the Creative Commons Attribution 4.0 license, and three tables come from the SRD 5.2.1.
The app is 5E compatible and uses no trademark of the publisher.

> This work includes material taken from the System Reference Document 5.1 ("SRD 5.1") by Wizards of the Coast LLC and available at https://dnd.wizards.com/resources/systems-reference-document. The SRD 5.1 is licensed under the Creative Commons Attribution 4.0 International License available at https://creativecommons.org/licenses/by/4.0/legalcode.

The Portuguese names and the structured effects are ours, and no text from books outside the SRD enters the repository.
The 5e-database data (MIT), the fonts (Alegreya, OFL 1.1) and the icons (Material Symbols, Apache 2.0) keep their licenses.
Each attribution is in the [NOTICE](NOTICE), with the licenses in [`third_party/licenses/`](third_party/licenses/) and on the app's "Créditos" page.
How the rules content is kept: [Rules content](docs/rules-content.md).
