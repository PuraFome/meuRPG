# MeuRPG documentation

This directory is the source of truth about the product, the rules and the architecture of MeuRPG. English is the canonical language. A subject lives in one file; the others link to it instead of copying it. **Current stage: pre-MVP** (see the [roadmap](roadmap.md)).

## Index

| Document | For whom | Answers |
| --- | --- | --- |
| [product/vision.md](product/vision.md) | Everyone | Why the app exists and what each role does |
| [product/glossary.md](product/glossary.md) | Everyone | What each term means |
| [product/rules.md](product/rules.md) | Product owner, story developer | How the game behaves inside the app |
| [product/stories.md](product/stories.md) | Product owner, story developer | What to build and how to know it is done |
| [design.md](design.md) | Anyone touching a screen | The app's look: principles, tokens, components, and how a screen is designed and reviewed |
| [architecture.md](architecture.md) | PR authors | How the system is built and where it runs |
| [data.md](data.md) | PR authors | Tables, relations and migrations |
| [privacy.md](privacy.md) | PR authors, reviewers | Which personal data we keep, why and for how long, and the privacy checklist of every PR |
| [roadmap.md](roadmap.md) | Everyone | The stages up to the MVP and what comes after |
| [operations.md](operations.md) | Operators | Deploy, secrets, costs and alerts |
| [legacy-app.md](legacy-app.md) | Anyone consulting the old app | What the Angular app in `src/` and the NestJS server in `server/` did and how they ran (discontinued) |
| [archive/](archive/README.md) | Anyone looking for history | Decision history, per-stage delivery history and archived notes |
| `adr/` | Maintainers | Hard-to-reverse decisions. A separate private repository; the local folder is in `.gitignore` and not versioned here |
| [../CONTRIBUTING.md](../CONTRIBUTING.md) | Everyone | How to run, test and open a PR |
| [../README.md](../README.md) | Newcomers | Project overview: what works, the look, the stack, how to run it |

API contracts (`.proto`) and database migrations are not documented here: the `.proto` files in `proto/meurpg/**/v1/*.proto` are the reference themselves; migrations live in `backend/migrations/`.

## Portuguese versions

The readers' docs have a Portuguese (Brazil) version under [pt-BR/](pt-BR/README.md): the product docs (vision, stories, rules, glossary) and privacy, plus [README.pt-BR.md](../README.pt-BR.md). Engineering docs are English only. The UI text stays Portuguese until internationalization.

## Keeping the docs current

Documentation changes in the same PR as the code. A PR that changes behavior without changing its document goes back in review. The full "what to update when" table and the writing rules are in [CONTRIBUTING.md → How to keep the docs current](../CONTRIBUTING.md#how-to-keep-the-docs-current).

Quick conventions:

- English, with software terms as usual. Code, comments and names in code are in English; quoted UI strings stay in Portuguese.
- Diagrams in Mermaid, which GitHub renders directly in Markdown.
- The first sentence of each section is the answer; the context follows.
- Documents say what the system does and why. When something was decided and by whom goes to the [archive](archive/decisions.md).
