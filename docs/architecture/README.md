# Architecture

MeuRPG is a Go modular monolith on Cloud Run that serves a Connect API and the Angular app from one origin, on top of CockroachDB.
Start with the [overview](overview.md); each page below covers one subject.

## The whole system

| Page | What it covers |
| --- | --- |
| [Overview](overview.md) | The pieces, the modules and their boundaries (a test enforces them), the path of a request. |
| [API](api.md) | The `.proto` contracts, error codes, idempotency keys, slow clients, abuse limits. |
| [Transactions](transactions.md) | `db.InTx` and `db.ReadTx`, the retry, and why nothing reads through the pool inside a transaction. |
| [Logs](logs.md) | The JSON log line, the `request_id`, what never goes in the log. |
| [Detached work](tasks.md) | `platform/tasks`: work that outlives its call runs as a Cloud Tasks task that calls the service back; in-process elsewhere. |
| [Web app](web.md) | `web/`: structure, routing, the Connect transport, security headers, what the browser keeps. |

## Modules

| Page | What it covers |
| --- | --- |
| [Identity](identity.md) | Sign-in with OIDC, the server-side session, CSRF, who is calling. |
| [Campaigns](campaigns.md) | Campaigns, members, invites, authorization, dice, table rules, the campaign document. |
| [Campaign package](campaign-package.md) | Exporting and importing a whole campaign as one file. |
| [Rules: the engine](rules.md) | Rules as data: `Derive`, effects, formulas, rests, choices, Portuguese texts. |
| [Rules: level-up](rules-level-up.md) | What a level gives, multiclassing. |
| [Rules: table content](rules-table-content.md) | The master's content as a layer over the SRD (`Content.With`). |
| [Rules: creatures and items](rules-creatures-items.md) | SRD creatures, summons, Wild Shape, magic items, trap and light presets. |
| [Rules: combat engine](rules-combat.md) | The pure arithmetic of a fight and the reaction rules. |
| [Rules: grid, vision and dungeons](rules-maps.md) | Map geometry, fog of war, the dungeon generator. |
| [Characters](characters.md) | Sheets, creation, approval, claims, level-up, revival, vitals and rests. |
| [Characters: table content](characters-table-content.md) | How the master's content is stored, served and switched on for players. |
| [Characters: the browser side](characters-web.md) | The character editor, level-up and the content and table-rules screens. |
| [Progression](progression.md) | XP, milestones and their history. |
| [Notes](notes.md) | Players' private notes and the clues revealed to them. |
| [Play](play/README.md) | The game table: sessions, the live stream, combat, casting, reactions, scenes, traps, puzzles (one page per subject inside `play/`). |
| [Maps](maps/README.md) | Maps, points and tokens, images and their generation, fog tiles, doors, dungeons, treasure (one page per subject inside `maps/`). |

The data model is in [data](../data/README.md), how to run and release in [operations](../operations/README.md), the visual system in [design](../design/README.md), and how the code is tested in [testing](../testing.md).
