# MeuRPG documentation

This directory says what MeuRPG does, why, and how it is built.
English is the canonical language.
A subject lives in one file, and the others link to it.

## Players and masters

The user guides live on a separate documentation site, not in this repository.
These pages are what a table needs to know about the app.

| Page | What it is for |
| --- | --- |
| [../README.md](../README.md) | What MeuRPG is, what a table can do with it and how to run it. |
| [product/vision.md](product/vision.md) | Why the app exists and what each role does. |
| [product/glossary.md](product/glossary.md) | What each term means, in English and Portuguese. |
| [privacy.md](privacy.md) | Which personal data the app keeps, why and for how long. |

## Product

| Page | What it is for |
| --- | --- |
| [product/stories.md](product/stories.md) | What the app does, story by story, and the acceptance criteria that prove it. |
| [product/rules.md](product/rules.md) | How the game behaves inside the app (RN-xx). |
| [roadmap.md](roadmap.md) | What is built and what comes next. |

## Engineering

| Page | What it is for |
| --- | --- |
| [../CONTRIBUTING.md](../CONTRIBUTING.md) | The PR flow, the commit style and the rule for keeping docs current. |
| [development.md](development.md) | The local environment in full: Docker, native, sign-in, images and variables. |
| [testing.md](testing.md) | The test layers, how to run them and what each proves. |
| [ci.md](ci.md) | What each CI job checks, the ratchets and gates, pinning and Dependabot. |
| [performance.md](performance.md) | The memory and time budgets in the code and how to measure them. |
| [rules-content.md](rules-content.md) | The SRD snapshot, the effects, names and texts, and how to change them. |

### Architecture

| Page | What it is for |
| --- | --- |
| [architecture/README.md](architecture/README.md) | The map of the system and the index of this folder. |
| [architecture/overview.md](architecture/overview.md) | The modular monolith, the modules and how a request flows. |
| [architecture/api.md](architecture/api.md) | How the `.proto` contracts become Go and TypeScript. |
| [architecture/identity.md](architecture/identity.md) | Sign-in and sessions. |
| [architecture/campaigns.md](architecture/campaigns.md) | Campaigns, members and invites. |
| [architecture/campaign-package.md](architecture/campaign-package.md) | Exporting and importing a whole campaign. |
| [architecture/characters.md](architecture/characters.md) | Characters and NPCs on the server. |
| [architecture/characters-table-content.md](architecture/characters-table-content.md) | Storing and serving the content a master registers. |
| [architecture/characters-web.md](architecture/characters-web.md) | The character screens in the browser. |
| [architecture/progression.md](architecture/progression.md) | XP and milestones. |
| [architecture/notes.md](architecture/notes.md) | Players' notes and revealed clues. |
| [architecture/rules.md](architecture/rules.md) | The rules engine: choices in, sheet numbers out. |
| [architecture/rules-combat.md](architecture/rules-combat.md) | The pure combat arithmetic. |
| [architecture/rules-creatures-items.md](architecture/rules-creatures-items.md) | Creatures, magic items, traps and lights as data. |
| [architecture/rules-level-up.md](architecture/rules-level-up.md) | What a level gives and what a locked sheet may change. |
| [architecture/rules-maps.md](architecture/rules-maps.md) | Grid geometry, vision and the dungeon generator. |
| [architecture/rules-table-content.md](architecture/rules-table-content.md) | The table's content as a layer over the SRD. |
| [architecture/transactions.md](architecture/transactions.md) | Transactions and the connection pool. |
| [architecture/logs.md](architecture/logs.md) | The log format and what never goes in a line. |
| [architecture/web.md](architecture/web.md) | The Angular app. |
| [architecture/maps/README.md](architecture/maps/README.md) | Maps, points of interest and tokens. |
| [architecture/maps/images.md](architecture/maps/images.md) | The image gallery and uploads. |
| [architecture/maps/image-generation.md](architecture/maps/image-generation.md) | AI-generated images. |
| [architecture/maps/editor.md](architecture/maps/editor.md) | Painted layers and grid calibration. |
| [architecture/maps/points.md](architecture/maps/points.md) | The kinds of map point. |
| [architecture/maps/doors.md](architecture/maps/doors.md) | Doors on the map. |
| [architecture/maps/dungeons.md](architecture/maps/dungeons.md) | Generated dungeon maps. |
| [architecture/maps/fog.md](architecture/maps/fog.md) | Fog of war and the per-player image tiles. |
| [architecture/maps/treasure.md](architecture/maps/treasure.md) | The treasure generator. |
| [architecture/maps/live-session.md](architecture/maps/live-session.md) | Map changes reaching the open session. |
| [architecture/play/README.md](architecture/play/README.md) | The `play` module and the index of its pages. |
| [architecture/play/live-session.md](architecture/play/live-session.md) | Sessions, the sheet lock and the live stream. |
| [architecture/play/combat.md](architecture/play/combat.md) | Starting a combat, initiative and turns. |
| [architecture/play/attacks-and-damage.md](architecture/play/attacks-and-damage.md) | Attack and damage rolls. |
| [architecture/play/casting.md](architecture/play/casting.md) | Casting in and out of combat. |
| [architecture/play/movement.md](architecture/play/movement.md) | Movement, jumping, cover and area spells. |
| [architecture/play/reactions.md](architecture/play/reactions.md) | The reaction window. |
| [architecture/play/held-rolls.md](architecture/play/held-rolls.md) | Rolls kept for a question. |
| [architecture/play/contests.md](architecture/play/contests.md) | Grapple, shove, Hide, Help and group checks. |
| [architecture/play/lasting-effects.md](architecture/play/lasting-effects.md) | Effects that last on a creature. |
| [architecture/play/death-and-revival.md](architecture/play/death-and-revival.md) | Death saves and bringing a character back. |
| [architecture/play/rests-and-resources.md](architecture/play/rests-and-resources.md) | Rests and the numbers that change in play. |
| [architecture/play/creatures.md](architecture/play/creatures.md) | A character's creatures in combat. |
| [architecture/play/encounters.md](architecture/play/encounters.md) | Monsters in combat and the encounter builder. |
| [architecture/play/theatre.md](architecture/play/theatre.md) | Combat without a grid. |
| [architecture/play/fog.md](architecture/play/fog.md) | What a player sees of the NPCs in a fog map. |
| [architecture/play/scenes-and-stage.md](architecture/play/scenes-and-stage.md) | RP scenes and NPCs on stage. |
| [architecture/play/traps.md](architecture/play/traps.md) | Noticing, triggering and disarming traps. |
| [architecture/play/puzzles.md](architecture/play/puzzles.md) | Puzzles played in the session. |
| [architecture/play/table-rules.md](architecture/play/table-rules.md) | The critical hit and death save rules of the table. |
| [architecture/play/combat-log.md](architecture/play/combat-log.md) | The combat log built from events. |
| [architecture/play/highlights-and-summary.md](architecture/play/highlights-and-summary.md) | Combat highlights and the session summary. |
| [architecture/play/undo.md](architecture/play/undo.md) | Undoing the last action. |

### Data

| Page | What it is for |
| --- | --- |
| [data/README.md](data/README.md) | The database, the migrations and the index of this folder. |
| [data/identity.md](data/identity.md) | Accounts and sessions. |
| [data/campaigns.md](data/campaigns.md) | Campaigns, members, invites and the document. |
| [data/package.md](data/package.md) | Campaign export and import. |
| [data/characters.md](data/characters.md) | Characters, creatures and the table's content. |
| [data/progression.md](data/progression.md) | The XP history and milestones. |
| [data/notes.md](data/notes.md) | Players' notes. |
| [data/maps.md](data/maps.md) | Maps, layers, tokens, fog and RP scenes. |
| [data/gallery.md](data/gallery.md) | The gallery and AI image requests. |
| [data/play.md](data/play.md) | Sessions, history and effects. |
| [data/combat.md](data/combat.md) | Encounters, pending damage and reactions. |
| [data/puzzles.md](data/puzzles.md) | Puzzles. |

### Operations

| Page | What it is for |
| --- | --- |
| [operations/README.md](operations/README.md) | Where the app runs, what it costs and the index of this folder. |
| [operations/runbook.md](operations/runbook.md) | The procedures: deploy, rollback, secrets and the edge policy. |
| [operations/reference.md](operations/reference.md) | The values: variables, limits, alerts and costs. |
| [operations/logs.md](operations/logs.md) | Reading the logs in Cloud Logging and ELK. |

### Design

| Page | What it is for |
| --- | --- |
| [design/README.md](design/README.md) | The paper sheet look, the rules every screen follows and how a screen is designed and reviewed. |
| [design/tokens.md](design/tokens.md) | Colours, type, spacing and the other `--mr-*` tokens. |
| [design/components.md](design/components.md) | The shared pieces. |
| [design/sheet.md](design/sheet.md) | The character sheet and its editors. |
| [design/session.md](design/session.md) | The live session page. |
| [design/combat.md](design/combat.md) | The combat screens. |
| [design/maps.md](design/maps.md) | The map components. |

### Archive and decisions

| Page | What it is for |
| --- | --- |
| [archive/README.md](archive/README.md) | What the archive holds and when to read it. |
| [archive/decisions.md](archive/decisions.md) | Who decided what, and why. |
| [archive/etapas.md](archive/etapas.md) | The delivery history. |
| [archive/legacy-code-quality.md](archive/legacy-code-quality.md) | A code-quality audit of the discontinued old app. |
| `adr/` | Hard-to-reverse decisions. A separate private repository, not versioned here. |

## Portuguese

The readers' docs have a Portuguese (Brazil) version: [pt-BR/README.md](pt-BR/README.md) indexes it, and [../README.pt-BR.md](../README.pt-BR.md) is the overview.
The product docs and privacy are translated: [vision](pt-BR/produto/visao.md), [stories](pt-BR/produto/historias.md), [rules](pt-BR/produto/regras.md), [glossary](pt-BR/produto/glossario.md) and [privacy](pt-BR/privacidade.md).
Engineering docs are English only.
The app's screens are in Portuguese.

## How the docs are kept

- A change updates the docs that say what the code cannot, in the same PR: the product docs (with their Portuguese twins), privacy, the runbooks and the references. [CONTRIBUTING.md](../CONTRIBUTING.md#keeping-the-docs-current) has the table.
- Implementation detail lives in the code, its comments and its tests. The `.proto` files in `proto/meurpg/**/v1/` are the API reference, and `backend/migrations/` is the schema.
- Every doc states the current state only, with no history or dates. History goes to the [archive](archive/README.md).
- Diagrams are Mermaid, which GitHub renders.
- The first sentence of each section is the answer; the context follows.
