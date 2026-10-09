# Archive

This folder keeps the history of MeuRPG: who decided what and when, how each Etapa was delivered, and notes about the discontinued app. History is not current behaviour. The docs outside this folder say what the system does and why; these files say when it was decided and by whom, so a doc never has to carry "decided on dd/mm" or "slice 10.4b" in its text.

Read these files when you need the origin of a rule, the question behind it, the PR that delivered it or the migration that created a table. Do not use them to learn how the system behaves today: they are not kept in sync with the code.

| File | What it holds |
| --- | --- |
| [decisions.md](decisions.md) | The decision history: Samuel's answers (questions 1 to 75), Vinicius's decisions (questions 66 to 86, scope changes, audit decisions), and the decisions that used to sit in the product, privacy, architecture, data, operations and design docs. Ordered by date, then by document. |
| [etapas.md](etapas.md) | The delivery history: each Etapa (1 to 10, then the Etapa 11 plan) with its PRs and migrations, the slice labels ("fatia 10.x") and what they delivered, per-story and per-rule delivery notes, and every migration with the table it added. |
| [legacy-code-quality.md](legacy-code-quality.md) | A code-quality audit of the old Angular app in `src/`, discontinued. |

Where to look for what:

- "Why does the rule say this?" Start at [decisions.md](decisions.md), by question number or by RN-xx / MR-xx id.
- "Which PR or migration delivered it?" Start at [etapas.md](etapas.md), by Etapa or by story id.
- "What does the system do now?" Go to [architecture](../architecture.md), [data model](../data.md), [operations](../operations.md), [design](../design.md) and [product](../product/rules.md), not here.

New history goes here when a doc stops needing it, not into the docs. The private ADRs (`docs/adr/`) are a separate record of architecture decisions and stay private.
