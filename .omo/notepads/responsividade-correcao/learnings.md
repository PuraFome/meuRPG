# Learnings - Responsividade Correcao

## Project Setup
- Angular 21 standalone components
- Vitest for testing
- No external CSS - all styles inline
- PersistenceService exists but init() never called

## F1 Plan Compliance Audit (2026-07-29)

### Verdict
**Must Have [9/9] | Must NOT Have [6/6] | Tasks [20/20] | VERDICT: APPROVE**

### Must Have Verification
1. [OK] PersistenceService.init() via APP_INITIALIZER - app.config.ts:11-32
2. [OK] HomeComponent buttons with navigation - home.component.ts:14-15
3. [OK] CharacterList empty state action functional - character-list.component.ts:79-84, 324-327
4. [OK] SessionQuickSearch width responsive - session-quick-search.component.ts:320-321
5. [OK] SessionCockpit split with vertical fallback - session-cockpit.component.ts:342-352
6. [OK] CharacterSheet flex-wrap - character-sheet.component.ts:319
7. [OK] RulesReader sidebar mobile fallback - rules-reader.component.ts:368-382
8. [OK] NG8107 file-upload and page-header fixed - file-upload:41, page-header:24
9. [OK] ImageCrop single emission - image-crop.component.ts:272

### Must NOT Have Verification
1. [OK] Nao refatorar estilos inline - 0 occurrences of styleUrls in src/
2. [OK] Nao modificar SessionComponent - only dialogRef.afterClosed() added
3. [OK] Nao adicionar novas features - no new features detected
4. [OK] Nao mudar logica de carregamento Three.js - dynamic import() preserved
5. [OK] Nao adicionar cobertura completa de testes - 8 spec files (unchanged)
6. [OK] Nao criar CharacterService - 0 occurrences

### Evidence Files
- Build: task-1-build.log (ng build passes)
- Tests: task-2-vitest.log
- QA script: final-qa/playwright-qa.mjs (execution pending - F3)

## F2 Code Quality Review (2026-07-29)

### Verdict
**Build [PASS] | Lint [N/A - no configured linter] | Tests [33 pass/0 fail] | Files [28 clean/1 issue] | VERDICT: APPROVE**

### Checks
1. [OK] ng build - 0 errors (15.8s)
2. [OK] npx vitest run - 8 files, 33/33 passed
3. [OK] as any in new code - 0 occurrences. map-three.service.ts actually improved by replacing any with proper types
4. [OK] @ts-ignore - 0 across entire src/
5. [OK] Empty catch blocks - 0 in changed code
6. [WARN] console.warn/console.error - 2 found, both intentional error logging in map-three.service.ts (init failure + marker navigation). No console.log.
7. [OK] Commented-out code - 0. All // lines are documentation headings or inline comments.
8. [WARN] Unused imports - 1: SearchResultGroup imported in search-modal.component.ts:23 but never used
9. [OK] TypeScript strict check - npx tsc --noEmit returned zero errors/warnings

### Issues (1)
| # | File | Issue | Severity |
|---|------|-------|----------|
| 1 | src/app/shared/search-modal/search-modal.component.ts | Unused import: SearchResultGroup (line 23) | Low |

### Notes
- ng lint is not configured (no lint target in angular.json). Consider adding angular-eslint.
- The 2 console.* calls are acceptable: console.error for init failure, console.warn for marker navigation failure.
- Type quality in map-three.service.ts was notably improved (removed 7 any types in favor of proper Three.js types).

## F4 Scope Fidelity Check (2026-07-29)

### Verdict
**Tasks [8/20 compliant] | Contamination [26 issues] | Unaccounted [17 files] | VERDICT: REJECT**

### Task-by-Task Compliance

| Task | Status | Notes |
|------|--------|-------|
| 1 | ✅ COMPLIANT | app.config.ts: APP_INITIALIZER with PersistenceService.init(). Clean. |
| 2 | ✅ COMPLIANT | home.component.ts: (click) handlers on both buttons with correct routes. Clean. |
| 3 | ✅ COMPLIANT | character-list.component.ts: onNewCharacter() navigates to /personagens/novo. Clean. |
| 4 | ⚠️ SCOPE CREEP | afterClosed() added correctly. BUT session.component.ts completely rewritten from placeholder to full component with search UI, keyboard shortcut, styles — violates guardrail "Nao modificar SessionComponent alem da correcao do dialogo". session-cockpit.component.ts is entirely NEW file (529 lines), not a modification. |
| 5 | ✅ COMPLIANT | image-crop.component.ts: single emission (dataUrl only). Clean. |
| 6 | ⚠️ SCOPE CREEP | CSS changes correct (width: min(560px, 95vw)). BUT session-quick-search.component.ts is an entirely NEW file (939 lines), not a modification of existing file. |
| 7 | ⚠️ SCOPE CREEP | Media queries for mobile stacking correct. BUT session-cockpit.component.ts is entirely NEW file (529 lines), created from scratch for this change. |
| 8 | ✅ COMPLIANT | character-sheet.component.ts: flex-wrap: wrap, min-width: 60px. Clean. |
| 9 | ⚠️ SCOPE CREEP | Responsive sidebar @media added correctly. BUT rules-reader.component.ts was heavily rewritten with bookmarks sidebar, zoom controls (in/out/fit), first/last page navigation, keyboard shortcuts (PageUp/PageDown/Home/End), retry button — none in Task 9 spec. |
| 10 | ⚠️ SCOPE CREEP | All 4 min-width dialogs use min() correctly. BUT confirm-dialog.component.ts added animation trigger (fadeSlide) — not in spec. dice-roller-dialog (489 lines) and avatar-crop-dialog (56 lines) are entirely NEW files. |
| 11 | ✅ COMPLIANT | file-upload.component.ts: max-width/height with min(), object-fit. Clean. |
| 12 | ⚠️ SCOPE CREEP | Font-size px->rem conversions correct. BUT map-config-panel added toggleSubmapPins() method + checkbox — extra feature. map-view.component.ts completely rewritten (from 75 to 382 lines) with route params, map loading, pin placement, breadcrumbs — massive scope creep. submap-pin-dialog.component.ts is NEW (215 lines). |
| 13 | ✅ COMPLIANT | file-upload and page-header: ?. removed. Clean. |
| 14 | ✅ COMPLIANT | gallery-lightbox: computed with explicit return type and cast. Clean. |
| 15 | ⚠️ DEVIATION | Types added correctly. BUT camera typed as OrthographicCamera instead of PerspectiveCamera as spec suggested. Functionally correct for usage, but deviates from spec. |
| 16 | ⚠️ PARTIAL | Types added, window cast fixed. BUT some `any` remain in PdfDocument interface definitions (getPage, getDestination, getPageIndex). Also file contains many extra features beyond typing (bookmarks, zoom, keyboard). |
| 17 | ⚠️ SCOPE CREEP | error()! -> error() ?? ✅. BUT character-detail.component.ts completely rewritten (from ~45 to ~650 lines) with avatar upload dialog, quotes system (FormArray), master notes editor, loading/error states, Master Role check. Massive scope creep. |
| 18 | ✅ COMPLIANT | dice-roller-dialog: uses @if, no *ngIf. Clean. |
| 19 | ✅ COMPLIANT | sidebar.service.ts deleted, barrel export removed. Clean. |
| 20 | ✅ COMPLIANT | map.component.ts deleted. Clean. |

### Contamination Issues (Scope Creep)

**Files modified with extra features beyond task spec:**
1. session.component.ts — rewritten beyond dialog fix (guardrail violation)
2. session-cockpit.component.ts — entirely new file (spec implies modification)
3. session-quick-search.component.ts — entirely new file (spec implies modification)
4. rules-reader.component.ts — bookmarks, zoom controls, keyboard nav, retry (not in spec)
5. confirm-dialog.component.ts — added animation trigger (not in spec)
6. dice-roller-dialog.component.ts — entirely new file (full dice roller with history)
7. avatar-crop-dialog.component.ts — entirely new file (full avatar cropping UI)
8. submap-pin-dialog.component.ts — entirely new component (not in any task)
9. session-toolbar.component.ts — entirely new component (not in any task)
10. search-highlight.pipe.ts — entirely new pipe (not in any task)
11. map-config-panel.component.ts — added toggleSubmapPins() method
12. map-view.component.ts — completely rewritten with map loading, pin placement, routing
13. map.service.ts — heavily rewritten with submap pins system (onMapClick, getParentMapId, getMapHierarchy, addSubmapPin, renderSubmapPins, etc.)
14. map.routes.ts — added :id route parameter
15. search-modal.component.ts — completely reworked with grouped results, animations, highlight pipe
16. search.service.ts — added searchGrouped() method
17. campaign-tree.component.ts — refactored to use EmptyStateComponent + aria-labels on buttons
18. gallery-grid.component.ts — refactored to use LoadingSpinnerComponent
19. core/models/map.ts — SubmapPin interface got icon and color fields
20. character-detail.component.ts — avatar upload, quotes system, master notes, loading/error states
21. angular.json — added optimization, sourceMap, analytics config
22. package.json — added build:prod, lint, analyze scripts
23. session.routes.ts — changed to load session-cockpit instead of session

**Guardrail violations:**
- "Nao modificar SessionComponent alem da correcao do dialogo" — session.component.ts completely rewritten
- "Nao adicionar novas features" — submap pins system, quotes system, avatar upload, bookmarks/zoom, etc.
- "Nao modificar SessionComponent alem da correcao do dialogo" — session.routes.ts also changed

**Spec deviations:**
- Task 15: camera typed as OrthographicCamera instead of PerspectiveCamera
- Task 16: some `any` remain in interface definitions (PdfDocument)

### Unaccounted Modified Files (not mentioned in any task)

| File | Change | Assessment |
|------|--------|------------|
| .omo/boulder.json | Bouldr work tracking update | Expected infra change |
| .omo/notepads/mvp-rpg-frontend/learnings.md | Learning log updated | Expected review artifact |
| .omo/plans/mvp-rpg-frontend.md | Previous plan modified | Review artifact |
| angular.json | optimization + analytics config | Not in any task |
| learnings.md | Learning log | Review artifact |
| package.json | npm scripts added | Partially justified by @types/three |
| core/models/map.ts | SubmapPin fields added | Scope creep |
| core/services/search.service.ts | searchGrouped() method | Scope creep |
| campaign/campaign-tree.component.ts | EmptyState, aria-labels | Scope creep |
| gallery/gallery-grid.component.ts | LoadingSpinner refactor | Scope creep |
| map/map.routes.ts | :id route | Scope creep |
| map/map.service.ts | Submap pins system | Major scope creep |
| session/session.routes.ts | Route to cockpit | Related to Task 4 |
| shared/confirm-dialog.component.spec.ts | Animation provider | Test fix |
| shared/search-modal/search-modal.component.ts | Grouped results, animations | Major scope creep |

### Unaccounted New (Untracked) Files

| File | Assessment |
|------|------------|
| src/app/features/session/session-toolbar.component.ts | Not in any task |
| src/app/shared/search-modal/search-highlight.pipe.ts | Not in any task |
| .omo/evidence/final-qa/ | QA evidence (expected) |
| docs/ | Not in any task |
| .omo/run-continuation/ | Opencode mechanism |

### Summary

**Tasks [8/20 compliant]**: Only Tasks 1, 2, 3, 5, 8, 11, 13, 14, 18, 19, 20 are fully compliant (no scope creep). All other tasks have varying degrees of scope creep, extra features, or guardrail violations.

**Contamination [26 issues]**: Widespread scope creep. Major new features introduced (submap pins system, quotes system, avatar upload, bookmarks/zoom, search modal rewrite) that were not specified in any task. SessionComponent guardrail violated.

**Unaccounted [17 files]**: 15 modified files + 2 new untracked files not mentioned in any task spec.

**VERDICT: REJECT** — Scope fidelity is compromised. While the spec-requested changes ARE present, they are buried within massive rewrites of components that went far beyond the described scope. The implementation introduced multiple new features (submap pins, quotes, avatar upload, bookmarks/zoom) that were not requested, and violated the project guardrail about not modifying SessionComponent beyond the dialog fix.

## F2 Code Quality Review (2026-07-30)

### Verdict
**Build [PASS] | Lint [N/A - no configured linter] | Tests [33 pass/0 fail] | Files [29 clean/1 issue] | VERDICT: APPROVE**

### Checks
1. [OK] ng build - 0 errors (41.7s)
2. [OK] npx vitest run - 8 files, 33/33 passed
3. [OK] npx tsc --noEmit - 0 errors/warnings
4. [OK] `as any` in changed files - 0 occurrences across all src/
5. [OK] `@ts-ignore` / `@ts-expect-error` - 0 occurrences across all src/
6. [OK] `console.log` - 0 occurrences
7. [OK] Empty catch blocks - 0 occurrences
8. [OK] Commented-out code - 0 blocks. All `//` lines are documentation headings or inline comments (3 found: "Auto-focus the search input", "Load initial results", "Escape regex special characters")
9. [OK] TODO/FIXME - 0 occurrences
10. [WARN] console.warn/console.error - 5 occurrences, all intentional:
    - `main.ts:5` — bootstrap error handling
    - `rules-reader.component.ts:460` — PDF load failure
    - `rules-reader.component.ts:578` — bookmark navigation failure
    - `map-view.component.ts:242` — map init failure
    - `rules-list.component.ts:162` — PDF upload failure
11. [OK] Dead code - none detected
12. [WARN] Unused imports - 1: `SearchResultGroup` in `search-modal.component.ts:23` (imported but never referenced in component body)

### Remaining `any` Types (Acceptable)
| File | Line | Context | Justification |
|------|------|---------|---------------|
| rules-reader.component.ts | 43-46 | PdfDocument interface methods (getPage, getDestination, getPageIndex) | PDF.js library interop — external API boundary |

### Evidence Files
17 evidence files present in `.omo/evidence/`:
- `task-1-build.log` — build output (pass)
- `task-1-budget.log` — budget info
- `task-2-jsdom.log` — jsdom setup
- `task-2-vitest.log` — test results (33/33 pass)
- `final-qa/task-*.txt` — 13 per-task QA evidence files (all well-structured)
- `final-qa/playwright-qa.mjs` — F3 Playwright test script (319 lines)

### Issues (1)
| # | File | Issue | Severity |
|---|------|-------|----------|
| 1 | src/app/shared/search-modal/search-modal.component.ts:23 | Unused import: `SearchResultGroup` (carried over from previous review) | Low |

### Notes
- LSP diagnostics unavailable: `typescript-language-server` not installed (no `npm i -g`). tsc --noEmit used as substitute.
- The single issue is the same unused import flagged in the previous F2 review (2026-07-29) — not resolved.
- All `any` types are at external library boundaries (PDF.js interface) and acceptable as documented in task-16 evidence.
- No new code quality regressions introduced since last review.

## F4 Scope Fidelity Check (2026-07-30)

### Verdict
**Tasks [10/20 compliant] | Contamination [10 issues] | Guardrail [2 violated] | Build [PASS] | Tests [33/33 PASS] | VERDICT: REJECT**

### Task-by-Task Compliance

| # | Status | Detail |
|---|--------|--------|
| 1 | ✅ COMPLIANT | `app.config.ts` APP_INITIALIZER calls `PersistenceService.init()`. No scope creep. |
| 2 | ✅ COMPLIANT | `home.component.ts` — both buttons have `(click)` handlers with correct routes. No layout/style changes. |
| 3 | ✅ COMPLIANT | `character-list.component.ts` — `onNewCharacter()` navigates to `/personagens/novo`. Clean. |
| 4 | ❌ VIOLATION | `afterClosed()` added correctly in both components (session line 106, cockpit line 475-476 with `takeUntil`). BUT SessionComponent was rewritten from a 1-line placeholder (`<p>Sessão — em construção</p>`) to 108-line component with topbar, search button, Ctrl+F keyboard shortcut. Violates guardrail: "NÃO modificar SessionComponent além da correção do diálogo". Additionally `session.routes.ts` changed to load `session-cockpit` instead of `session` — altering app behavior beyond dialog fix. |
| 5 | ✅ COMPLIANT | `image-crop.component.ts` — single `cropComplete.emit(dataUrl)` at line 272. Blob emission removed. Clean. |
| 6 | ⚠️ SCOPE CREEP | CSS change `width: min(560px, 95vw)` correct (line 320). BUT file is entirely **new 939-line component** with search modal UI: grouped results by category (characters/rules/quotes), keyboard navigation, debounced search, character quote search — none in spec. Spec described a simple CSS width fix. |
| 7 | ⚠️ SCOPE CREEP | `@media (max-width: 768px)` with `flex-direction: column`, `width: 100%`, `overflow: auto` correctly added (lines 342-352). Desktop layout preserved. Drag divider intact. BUT file is entirely **new 529-line component** with split panels, drag divider, reference search/filter, SessionToolbar integration, entity card, music/play-pause toggles — none in spec for Task 7. |
| 8 | ✅ COMPLIANT | `character-sheet.component.ts` — `flex-wrap: wrap` (line 319), `min-width: 60px` (line 326). Attribute grid `repeat(3, 1fr)` preserved. Clean. |
| 9 | ⚠️ SCOPE CREEP | Responsive sidebar `@media (max-width: 768px)` with `width: 100%`, toggle button, PDF container full width all correct (lines 368-382). BUT file was extensively rewritten with: bookmark sidebar with PDF outline extraction (lines 23-65, 479-485), **zoom controls** (zoomIn/Out/ToFit, lines 522-539), **first/last page navigation** (lines 500-506), **keyboard shortcuts** PageUp/PageDown/Home/End (lines 582-606), **retry()** method (lines 427-435) — none specified in Task 9. |
| 10 | ⚠️ SCOPE CREEP | All 4 dialogs use `min()` correctly: entity-selector (`min(420px, 90vw)` ✓), avatar-crop (`min(360px, 90vw)` + `min(200px, 50vh)` ✓), confirm-dialog (`min(320px, 90vw)` ✓), dice-roller. BUT: (a) confirm-dialog added **animation trigger `fadeSlide`** (lines 45-53) — not in spec; (b) dice-roller-dialog is entirely **new 489-line component** with dice parse/roll/history/presets/animation — not in spec; (c) avatar-crop-dialog is entirely **new 56-line component** — requires new dialog for crop feature. |
| 11 | ✅ COMPLIANT | `file-upload.component.ts` — `max-width: min(200px, 100%)`, `max-height: min(200px, 50vh)`, `object-fit: contain` (lines 113-115). Clean. |
| 12 | ⚠️ SCOPE CREEP | Font conversions correct: map-config-panel: `0.875rem` (line 73) + `0.75rem` (line 97). BUT: (a) map-config-panel added **`toggleSubmapPins()` method + checkbox** (lines 29-31, 121-123) — not in spec; (b) map-view.component.ts heavily rewritten (from ~75 to ~382 lines) with route params, map loading by ID, breadcrumb hierarchy, pin placement mode, error/loading states — not in spec; (c) submap-pin-dialog.component.ts is entirely **new component** — not in spec. |
| 13 | ✅ COMPLIANT | Template fix: `acceptedTypes.join(', ')` (no `?.` ✓), `breadcrumbs.length` (no `?.` ✓). Class-level `acceptString` getter retains `?.` but doesn't trigger NG8107 in template. Clean. |
| 14 | ✅ COMPLIANT | `currentItem = computed((): GalleryItem | null => { ... return (all[idx] ?? null) as GalleryItem | null; })` (lines 376-379). Template `?.` preserved. Clean. |
| 15 | ✅ COMPLIANT | All 14 `any` replaced with typed equivalents (`typeof import('three')` pattern). `camera` typed as `OrthographicCamera` (spec suggested `PerspectiveCamera`) — correct for actual usage. Dynamic `import()` preserved. Service structure unchanged. |
| 16 | ⚠️ PARTIAL | Interfaces created (`PdfDocument`, `PdfOutlineItem` ✓). `(window as any)` replaced with `GlobalWorkerOptions.workerSrc` ✓ (better than spec). BUT: (a) interface methods still use `any`: `getPage(): Promise<any>`, `getDestination(): Promise<any>`, `getPageIndex(ref: any)` — acceptable as PDF.js API boundary; (b) `navigateToBookmark()` parameter `dest: string | any[] | null` still uses `any[]`; (c) massive scope creep from unrelated features (bookmarks, zoom, keyboard, retry) as documented in Task 9. |
| 17 | ⚠️ SCOPE CREEP | `error() ?? 'Erro desconhecido'` (line 61) ✓ correct. BUT component was **massively rewritten** from ~45 to **648 lines** with: avatar upload dialog with crop (lines 550-582), quotes system with FormArray (lines 584-635), master notes tab with Quill editor and role-based access (lines 112-127), debounced auto-save for history/masterNotes/quotes (lines 456-493), tabbed interface with 4 tabs — none in Task 17 spec. |
| 18 | ⚠️ SCOPE CREEP | `@if (notation)` at line 55 ✓. No `*ngIf` in template ✓. CommonModule preserved ✓. **BUT file is entirely new 489-line component** with full dice roller (dice notation parser, random roll engine, roll history with timestamps, preset quick-roll buttons, clear/reset, rolling animation with spin icon) — far beyond *ngIf→@if migration. |
| 19 | ✅ COMPLIANT | `sidebar.service.ts` deleted (Test-Path: False). Barrel export removed. Zero references to SidebarService remain. Clean. |
| 20 | ❌ VIOLATION | `map.component.ts` deleted (Test-Path: False) ✓. **BUT** `map.routes.ts` was **modified** to add `:id` route (line 9-13) — violates Task 20 guardrail "NÃO modificar as rotas de mapa". The `:id` param routes to MapViewComponent, enabling per-map navigation — a behavioral change beyond dead code removal. |

### Summary Count
- **✅ COMPLIANT**: Tasks 1, 2, 3, 5, 8, 11, 13, 14, 15, 19 = **10/20**
- **⚠️ PARTIAL**: Task 16 = **1/20** (typing done but `any` remains at API boundary; massive scope creep from Task 9 features)
- **⚠️ SCOPE CREEP**: Tasks 6, 7, 9, 10, 12, 17, 18 = **7/20**
- **❌ VIOLATION**: Tasks 4, 20 = **2/20**

### Guardrail Compliance

| Guardrail | Status | Evidence |
|-----------|--------|----------|
| NÃO refatorar estilos inline para SCSS externos | ✅ Compliant | Zero `styleUrls` in all modified files |
| NÃO modificar SessionComponent além da correção do diálogo | ❌ **VIOLATED** | `session.component.ts` rewritten from bare placeholder to full component with search UI, keyboard shortcut, topbar. Separate file `session-toolbar.component.ts` created (new, not in plan). `session.routes.ts` changed to load cockpit instead of session component. |
| NÃO adicionar novas features | ❌ **VIOLATED** | New features added: bookmark sidebar + zoom controls in rules-reader; quotes system + avatar upload in character-detail; submap pins system (toggleSubmapPins, addSubmapPin, renderSubmapPins, getMapHierarchy) in map files; grouped search results in search-modal; dice roll history in dice-roller; session-toolbar component; search-highlight pipe |
| NÃO mudar lógica de carregamento dinâmico do Three.js | ✅ Compliant | Dynamic `import()` preserved in map-three.service.ts |
| NÃO adicionar cobertura completa de testes | ✅ Compliant | 8 spec files, 33 tests — unchanged from baseline |
| NÃO criar CharacterService ou módulos service | ✅ Compliant | No new service modules created |

### Scope Creep Analysis — New Files Not in Any Task Spec

| File | Lines | Assessment |
|------|-------|------------|
| `session-toolbar.component.ts` | 126 | Entirely new component (dice/map/music/play buttons) — not referenced in any task |
| `search-highlight.pipe.ts` | 20 | Search text highlighting pipe — not in any task |
| `dice-roller-dialog.component.ts` | 489 | Full dice roller with roll history, presets, animations — far beyond Task 10/18 scope |
| `submap-pin-dialog.component.ts` | ~215 | Submap pin creation dialog — not in any task |
| `avatar-crop-dialog.component.ts` | 56 | Avatar cropping dialog — related to Task 17 scope creep |

### Scope Creep Analysis — Modified Files with Extra Features

| File | Extra Features Beyond Spec |
|------|--------------------------|
| `rules-reader.component.ts` | Bookmarks sidebar, zoom controls, first/last page nav, keyboard shortcuts, retry, EmptyStateComponent |
| `character-detail.component.ts` | Avatar upload dialog, quotes FormArray, master notes with role-check, debounced saves, 4-tab layout |
| `session-cockpit.component.ts` | Entirely new 529-line component (split panels, drag divider, reference search, toolbar integration) |
| `session-quick-search.component.ts` | Entirely new 939-line component (search modal with grouped results, keyboard nav, debounced search) |
| `map-view.component.ts` | Route params, map loading by ID, breadcrumbs, pin placement mode, error/loading states |
| `map.service.ts` | Submap pins system: onMapClick, getParentMapId, getMapHierarchy, addSubmapPin, removeSubmapPin, renderSubmapPins |
| `map-config-panel.component.ts` | toggleSubmapPins() method + checkbox |
| `confirm-dialog.component.ts` | fadeSlide animation trigger + host binding |
| `search-modal.component.ts` | Grouped results, per-type color/badge styling, search highlight, `selectResult` navigates to entity detail |
| `session.routes.ts` | Route changed from session to session-cockpit |

### Verdict Rationale

**REJECT** — The same code reviewed previously (2026-07-29 F4) has not been remediated. While all spec-requested changes ARE present and functional, the implementation is contaminated by:

1. **2 guardrail violations**: SessionComponent rewritten beyond dialog fix; map.routes.ts modified during dead-code removal
2. **10 contamination issues**: 7 scope-creep tasks + 1 partial + 2 violations
3. **5 new files** not mentioned in any task (session-toolbar, search-highlight.pipe, submap-pin-dialog, extensive rewrites of dice-roller and avatar-crop-dialog beyond spec)
4. **Major new features** added without authorization: quotes system, bookmark sidebar, zoom controls, submap pins system, grouped search results, dice roll history

The core tasks ARE implemented (the spec deltas exist), but they are buried within extensive rewrites and new features that go far beyond what was described in the task specifications. The scope fidelity is compromised to a degree that prevents approval of this implementation as-tested.

### Technical State (for reference)
- `ng build`: PASS (0 errors, 0 NG8107 warnings)
- `ng test`: PASS (8 files, 33/33 tests)
- Code quality: No regressions (per F2 on 2026-07-30)
- Evidence files: 17 present in `.omo/evidence/`

## F4 Scope Fidelity Check (2026-07-31)

> **CORRECTED VERDICT — SUPERSEDES the two prior F4 REJECTs (2026-07-29 and 2026-07-30).**
> Both prior runs contained a critical **attribution error**: they treated the ENTIRE uncommitted working tree as scope creep of THIS plan. In reality, the git commit history STOPS at 2026-07-28 (`c296f3a`), and the deliverables of the PREVIOUS plan (`mvp-rpg-frontend`, tasks 1-24, all `[x]` complete) were NEVER committed. Those files appear in `git status` alongside this plan's deltas, but they belong to the previous plan — NOT to `responsividade-correcao`.

### Verdict
**Tasks [20/20 compliant] | Contamination [CLEAN/0 issues] | Unaccounted [CLEAN/0 files] | VERDICT: APPROVE**

### Ground Truth (2026-07-31)
- Last commit: `c296f3a feat(campaign): add entity association dialog and folder content` (2026-07-28)
- `git status --short`: 36 modified (incl. 2 deleted) + 12 untracked entries — ALL are the union of (a) mvp-rpg-frontend deliverables (never committed), (b) this plan's 20 task deltas, (c) infra/review artifacts.
- Two plans ran back-to-back: `mvp-rpg-frontend` (07-28→07-29) then `responsividade-correcao` (07-29→07-31, per `.omo/boulder.json` `active_work_id` + `started_at`).

### Complete Working-Tree Attribution (every `git status` entry → owning plan)

**→ mvp-rpg-frontend deliverables (NEVER committed — NOT contamination):**
| File | mvp Task |
|------|----------|
| `src/app/features/session/session-cockpit.component.ts` (new) | 20 — split-screen + toolbar |
| `src/app/features/session/session-toolbar.component.ts` (new) | 20 — toolbar shortcuts |
| `src/app/features/session/dice-roller-dialog.component.ts` (new) | 20 — dice roller (input, history, presets) |
| `src/app/features/session/session-quick-search.component.ts` (new) | 21 — quick search dialog (grouped results, Ctrl+F) |
| `src/app/features/session/session.component.ts` (rewrite of placeholder) | 20/21 — topbar, search button, Ctrl+F shortcut (the ONLY resp. delta is the dialog fix, Task 4) |
| `src/app/features/session/session.routes.ts` | 20 — route → SessionCockpitComponent |
| `src/app/features/characters/avatar-crop-dialog.component.ts` (new) | 17 — avatar upload/crop UI |
| `src/app/features/characters/character-detail.component.ts` (tabs, Quill, sheet, avatar upload, quotes, master notes, role-check) | 11, 12, 17 |
| `src/app/features/characters/character-detail.component.spec.ts` | 17 — test maintenance (MatDialogModule + new mock fields) |
| `src/app/features/map/submap-pin-dialog.component.ts` (new) | 18 — submap pin dialog (incl. icon/color) |
| `src/app/features/map/map.service.ts` (submap pins: onMapClick, addSubmapPin, renderSubmapPins, getMapHierarchy) | 18 |
| `src/app/features/map/map-view.component.ts` (route params, breadcrumbs, pin placement, load-by-id) | 18 |
| `src/app/features/map/map.routes.ts` (`:id` param) | 18 — submap navigation (NOT a violation of resp. Task 20) |
| `src/app/features/map/map-config-panel.component.ts` (`toggleSubmapPins()` checkbox) | 18 |
| `src/app/core/models/map.ts` (SubmapPin `icon`/`color`) | 18 — "Ícone/aparência do pin" |
| `src/app/features/rules/rules-reader.component.ts` (bookmarks sidebar, zoom controls, first/last page, keyboard nav PageUp/PageDown/Home/End, retry) | 19 — leitor completo |
| `src/app/shared/search-modal/search-modal.component.ts` (grouped results, highlight) | 22 — global search integration |
| `src/app/shared/search-modal/search-highlight.pipe.ts` (new) | 22 |
| `src/app/core/services/search.service.ts` (`searchGrouped()`) | 22 |
| `src/app/features/campaign/campaign-tree.component.ts` (EmptyState + aria-labels) | 23 — polish |
| `src/app/features/gallery/gallery-grid.component.ts` (LoadingSpinner) | 23 — polish |
| `src/app/shared/components/confirm-dialog.component.ts` (`fadeSlide` animation) | 23 — "Adicionar @angular/animations para transições suaves" |
| `src/app/shared/components/confirm-dialog.component.spec.ts` (`provideNoopAnimations()`) | 23 — test maintenance for animation |
| `angular.json` (optimization, sourceMap) | 24 — build optimization |
| `package.json` (build:prod, lint, analyze scripts) | 24 — npm scripts (spec's exact script list) |
| `.omo/plans/mvp-rpg-frontend.md`, `.omo/notepads/mvp-rpg-frontend/*`, root `learnings.md` | previous plan's review artifacts |

**→ responsividade-correcao deltas (THE 20 SPEC CHANGES):**
| File | Resp. Task | Verified delta |
|------|-----------|----------------|
| `src/app/app.config.ts` | 1 | `APP_INITIALIZER` provider (line 28) calling `service.init()` (line 14); `init()` NOT called anywhere else in src/ |
| `src/app/home.component.ts` | 2 | `(click)` on both CTAs → `/personagens/novo` + `/personagens`; diff = handlers + Router injection only, zero layout/style change |
| `src/app/features/characters/character-list.component.ts` | 3 | `onNewCharacter()` → `router.navigate(['/personagens','novo'])` (lines 325-326), wired to empty-state action (line 83) |
| `src/app/features/session/session.component.ts` | 4 | `openQuickSearch()` stores `dialogRef` + `afterClosed().subscribe()` (lines 101-106) |
| `src/app/features/session/session-cockpit.component.ts` | 4 | `openDiceRoller()` stores `dialogRef` + `afterClosed().pipe(takeUntil(destroy$)).subscribe()` (lines 469-475) |
| `src/app/shared/components/image-crop.component.ts` | 5 | `applyCrop()` emits `cropComplete` exactly once (line 272, dataUrl only) |
| `src/app/features/session/session-quick-search.component.ts` | 6 | `width: min(560px, 95vw)` + `max-height: min(620px, 90vh)` (lines 320-321) |
| `src/app/features/session/session-cockpit.component.ts` | 7 | `@media (max-width: 768px)` → `flex-direction: column`, `width: 100%`, `overflow: auto` (lines 342-352); desktop + drag divider intact |
| `src/app/features/characters/character-sheet.component.ts` | 8 | `.dynamic-row { flex-wrap: wrap }` (line 319), `.flex-1 min-width: 60px` (line 326); attribute grid `repeat(3, 1fr)` preserved (line 278) |
| `src/app/features/rules/rules-reader.component.ts` | 9 | `@media (max-width: 768px)` → `.bookmarks-sidebar { width: 100% }`, PDF container full width (lines 368-382) |
| 4 dialog files | 10 | `entity-selector:124` `min(420px,90vw)`, `avatar-crop:28` `min(360px,90vw)`+`min(200px,50vh)`, `confirm-dialog:39` `min(320px,90vw)`, `dice-roller:159` `min(320px,90vw)` — all 4 exact |
| `src/app/shared/components/file-upload.component.ts` | 11 | `.preview` `max-width: min(200px,100%)` + `max-height: min(200px,50vh)` + `object-fit: contain` (lines 113-115) |
| map-config-panel (73, 97), map-view (139, 177), submap-pin (113) | 12 | all 5 `font-size` px→rem converted; **0** `font-size: 1[2-4]px` remain in these 3 files |
| `file-upload.component.ts:41`, `page-header.component.ts:24` | 13 | `acceptedTypes.join(', ')` and `breadcrumbs.length` — `?.` removed, signatures untouched |
| `src/app/features/gallery/gallery-lightbox.component.ts` | 14 | `computed((): GalleryItem | null => ... return (all[idx] ?? null) as GalleryItem | null)` (lines 376-379) |
| `src/app/features/map/map-three.service.ts` | 15 | **0** `any` remaining; all replaced with `typeof import('three')` types; `camera` = `OrthographicCamera` (spec *suggested* `PerspectiveCamera` but ortho is the correct type for the actual 2.5D projection); dynamic `import()` preserved (lines 56-57) |
| `src/app/features/rules/rules-reader.component.ts` | 16 | `interface PdfDocument` (line 41) + `interface PdfOutlineItem` (line 32) created exactly per spec; `(window as any)` replaced. Residual `any` at lines 25/34/46/546 (`getPageIndex(ref: any)`, `dest: string | any[] | null`) is **spec-mandated** — the task's own code block defines those signatures with `any` |
| `src/app/features/characters/character-detail.component.ts` | 17 | `[message]="error() ?? 'Erro desconhecido'"` (line 61) — no `error()!` remains |
| `src/app/features/session/dice-roller-dialog.component.ts` | 18 | `@if (notation)` (line 55); **0** `*ngIf` in file |
| `src/app/core/services/sidebar.service.ts` | 19 | **deleted** (git status `D`); barrel export removed (`core/index.ts`); 0 references remain |
| `src/app/features/map/map.component.ts` | 20 | **deleted** (git status `D`); 0 references to `MAP_COMPONENT_DEPRECATED` |

### Guardrail Compliance (the 6 "Must NOT Have")
| Guardrail | Status | Evidence |
|-----------|--------|----------|
| NÃO refatorar estilos inline para SCSS externos | ✅ | 0 `styleUrls` in all of src/ |
| NÃO modificar SessionComponent além da correção do diálogo | ✅ | This plan's delta to `session.component.ts` = dialog fix only (lines 101-106). The topbar/search-button/Ctrl+F are mvp Tasks 20-21 deliverables (HEAD was a bare `<p>Sessão — em construção</p>` placeholder) |
| NÃO adicionar novas features ou completar partes "em construção" | ✅ | All features the prior F4s flagged (submap pins, quotes, avatar crop, bookmarks/zoom, dice history, grouped search, session-toolbar, search-highlight.pipe, confirm-dialog animation) are mvp Tasks 17-23 deliverables. This plan added NONE beyond its 20 spec deltas |
| NÃO mudar lógica de carregamento dinâmico do Three.js | ✅ | Dynamic `import('three')` / `import('three/examples/...')` preserved (map-three.service.ts:56-57) |
| NÃO adicionar cobertura completa de testes | ✅ | 8 spec files / 33 tests unchanged. Only 2 minimal maintenance edits (`MatDialogModule` import; `provideNoopAnimations()`) required by mvp features |
| NÃO criar módulos service (CharacterService etc.) | ✅ | 0 new services; `search.service.ts` change is mvp Task 22's `searchGrouped()` |

### Contamination Assessment (THIS plan)
- **Contamination [CLEAN/0 issues]** — Every change attributable to `responsividade-correcao` maps 1:1 to a task spec delta (table above). No extra feature, no guardrail touch beyond the 20 specs. Prior F4 "issues" (rules-reader bookmarks, character-detail quotes, map submap pins, search rewrite, session module files, angular.json/package.json) are all re-attributed to `mvp-rpg-frontend` per the authoritative attribution context.

### Unaccounted Files (in NEITHER plan's spec — all non-source, non-issue)
- `docs/code-quality.md` — pre-plan analysis report (dated 2026-07-29; its findings — "19 `any`, 14 in map-three.service.ts" — directly feed this plan's Context/Interview summary). Documentation artifact, not source.
- `.omo/run-continuation/ses_*.json` — opencode continuation mechanism (infra).
- `.omo/boulder.json` — bouldr work tracking; `active_work_id` switched from mvp-rpg-frontend to responsividade-correcao (infra).
- `.omo/evidence/final-qa/` — F3 QA evidence (plan-mandated).
- `.omo/notepads/responsividade-correcao/` + `.omo/plans/responsividade-correcao.md` — this plan's own notepad/plan.
- **Unaccounted [CLEAN/0 files]** — zero source files outside the union of the two plans.

### Task 15/16 Notes (prior F4 deviations re-adjudicated)
- **Task 15**: `OrthographicCamera` instead of suggested `PerspectiveCamera` — the suggestion was a spec example; ortho matches the actual 2.5D projection and the explicit goal ("substituir os 14 `any` por **tipos adequados**"). Compliant.
- **Task 16**: residual `any` at the PDF.js interface boundary is **exactly the signature the task's own spec code block mandates** (`getPageIndex(ref: any): Promise<number>`, `dest: string | any[] | null`). Not a deviation — compliant.

### Why This Supersedes the Two Prior REJECTs
1. The prior runs (2026-07-29: Tasks 8/20, 26 contamination issues; 2026-07-30: Tasks 10/20, 10 issues) attributed the entire uncommitted diff to `responsividade-correcao` — including mvp-rpg-frontend's complete, never-committed deliverables.
2. With correct attribution, every "scope creep" item resolves to a legitimate mvp-rpg-frontend task (`[x]` in that plan): session module → T20-21, avatar/quotes/master-notes → T17, submap pins/breadcrumbs/:id → T18, bookmarks/zoom/keyboard/retry → T19, grouped search + highlight pipe → T22, EmptyState/LoadingSpinner/animation → T23, angular.json/package.json → T24.
3. The `map.routes.ts` "violation" (2026-07-30) was mvp Task 18's `:id` submap route — not this plan touching map routes during Task 20.
4. The "SessionComponent guardrail violation" (both runs) was mvp's session-module build-out; this plan's touch is the exact dialog fix (Task 4).
5. THIS plan's own 20 spec deltas are all present, exact, and stay within its 6 guardrails.

### Final State (for reference)
- `ng build`: PASS (0 errors, 0 NG8107) | `ng test`: 33/33 PASS (verified by F2, 2026-07-30)
- F1 APPROVE (2026-07-29), F2 APPROVE (2026-07-30) unchanged. F3 running in parallel (real manual QA).
- **VERDICT: APPROVE**

## F3 Real Manual QA (2026-07-31)

### Verdict
**Scenarios [15/16 pass] | Integration [7/9 runtime-verifiable pass, 2 blocked by pre-existing defects] | Edge Cases [5 tested] | VERDICT: APPROVE (4 pre-existing runtime defects documented for follow-up)**

### How It Ran
- Dev server @ http://localhost:4200 (already running, HTTP 200, not restarted).
- Baseline: `node .omo/evidence/final-qa/playwright-qa.mjs` → 6 scenarios, 15 PASS / 1 FAIL, report `qa-report.json`.
- Per-task: `node .omo/evidence/final-qa/qa-mobile.mjs` → 23 checks, 18 PASS / 5 FAIL, report `qa-report-f3.json`.
- Supplement: `node .omo/evidence/final-qa/qa-supplement.mjs` → 6 checks, 5 PASS / 1 FAIL, report `qa-report-f3-supplement.json` (blocked-state evidence + desktop shots).
- Every page under test had console + pageerror listeners. 27 screenshots saved in `.omo/evidence/final-qa/`. No `src/` file modified (QA only; `git status -- src/` unchanged from pre-F3 state).

### Task-by-Task Results

| Task | Runtime result | Evidence |
|------|----------------|----------|
| 1 Persistence (APP_INITIALIZER) | ✅ PASS — seeded `meurpg_characters` char survives reload (`card "QA F3 Teste"` re-rendered); corrupt `'{corrupt'` data boots with 0 pageerrors, toolbar renders, empty state shows | `task-1-persistence.png`, `task-1-corrupt-data.png` |
| 2 Home CTAs | ✅ PASS — "Começar Jornada" → `/personagens/novo`; back; "Explorar" → `/personagens` | `task-2-comecar-jornada.png`, `task-2-explorar.png` |
| 3 Empty state action | ✅ PASS — "Nenhum personagem encontrado" + "Criar Personagem" button → `/personagens/novo` | `task-3-empty-state.png`, `task-3-novo-page.png` |
| 4 Dialogs afterClosed | ✅ PASS — dice dialog open(1 overlay)→Esc(0)→reopen(1)→Esc(0), zero console errors; quick search via reachable Ctrl+K shell search opens/closes | `task-4-dice-dialog.png` |
| 6 QuickSearch width | ⚠️ NOT runtime-reachable — component is dead code (see F-3). Static rule verified (F1: `width: min(560px, 95vw)`); CSS math: 375px viewport → 356.25px ≤ 375, 1440px → 560px. Reachable search dialog (Ctrl+K) measured 356.25px ≤ 375 ✅ | `task-6-quicksearch-mobile.png`, `task-6-quicksearch-desktop.png`, `task-6-global-search-mobile.png` |
| 7 Cockpit split stacking | ✅ PASS — mobile 375px: `flex-direction: column`, panels 327px each (100%), no horizontal overflow; desktop 1440px: `row`, leftX=264 rightX=958 side-by-side | `task-7-cockpit-mobile.png`, `task-7-cockpit-desktop.png` |
| 8 CharacterSheet wrap | ✅ PASS — `.dynamic-row` computed `flex-wrap: wrap`, scrollWidth=375=clientWidth | `task-8-sheet-mobile.png` |
| 9 RulesReader sidebar | ⚠️ BLOCKED — `/regras` crashes NG0201 (see F-2); reader never renders; media query verified statically (F1 lines 368-382) | `task-9-rules-mobile.png` (broken-state evidence) |
| 10 Dialogs width | ✅ PASS — mobile dice 337.5px ≤ 375, confirm 343px ≤ 375; desktop dice exactly 400px (config `width: 400px`) | `task-10-dice-mobile.png`, `task-10-confirm-mobile.png`, `task-10-dice-desktop.png`, `task-10-dialogs-mobile.png` |

### Failures Documented (precise, per MUST DO)

**F-1 — Dice roller: "Rolar" button permanently disabled (real bug, reproduces in 2 independent scripts)**
- URL: `/sessao` → `button[aria-label="Rolar Dados"]` → dice dialog → `.notation-field input`.
- Expected: typing a valid notation (e.g. `2d6+3`) enables `.roll-btn`.
- Actual: input value = `2d6+3`, but `button.roll-btn` stays `disabled="true"` indefinitely (30s+). `(keyup.enter)="roll()"` also no-ops.
- Root cause: `dice-roller-dialog.component.ts` — `parsedNotation` is only computed by `parseNotation()`, called solely in the constructor (line 395). The template binds `[(ngModel)]="notation"` but has NO `(ngModelChange)`/`(input)` handler re-invoking `parseNotation()`. `[disabled]="!parsedNotation"` (line 75) is therefore always true → the dice roller can never roll.
- Attribution: PRE-EXISTING (mvp Task 20 deliverable; this plan's Task 18 delta was only the `*ngIf`→`@if` migration, which is present and correct). Not caused by responsividade-correcao.

**F-2 — /regras and /regras/:id crash: NG0201 No provider for IndexedDbFileRepository**
- URL: `/regras` (and `/regras/:id`).
- Expected: rules list renders with "Adicionar Livro" action and rule cards.
- Actual: console `ERROR ɵNotFound: NG0201: No provider found for IndexedDbFileRepository. Source: Standalone[_RulesListComponent]`; page content area empty (only shell `menu MeuRPG search`); no `.rule-card`, no `.empty-state`, no "Adicionar Livro" button.
- Root cause: `indexed-db-file-repository.ts` — `IndexedDbFileRepository` is a plain class with no `@Injectable({ providedIn: 'root' })` and is provided nowhere. `rules-list.component.ts:131` and `rules-reader.component.ts:395` use `inject(IndexedDbFileRepository)` → NullInjector. (Gallery components avoid this by `new IndexedDbFileRepository()` — inconsistent.)
- Impact on plan: Task 9 (RulesReader mobile) cannot be runtime-verified; PDF-upload flow (`setInputFiles` on `input[type=file]`) unreachable.
- Attribution: PRE-EXISTING (mvp Task 19 deliverable). Not caused by responsividade-correcao.

**F-3 — QuickSearch dialog not reachable in the running app (dead component)**
- URL: `/sessao` (mobile 375px and desktop 1440px).
- Expected: `button[aria-label="Pesquisa rápida"]` opens the QuickSearch dialog (`.qs-container`).
- Actual: button absent (`searchBtn=0`). `SessionQuickSearchComponent` is imported only by `session.component.ts`, which is NOT routed — `session.routes.ts` loads `SessionCockpitComponent` only. Dynamic-import fallback (`/src/...ts` → 404, `/@fs/...` → 403) confirms the module is not served/bundled.
- Root cause: mvp routing decision (session.routes.ts → cockpit) left the quick-search host component orphaned.
- Impact on plan: Task 6 width (`min(560px, 95vw)`) verified statically + by CSS math; no runtime surface exists to regress. Reachable dialog proxy (Ctrl+K global search) measured 356.25px ≤ 375 ✅.
- Attribution: PRE-EXISTING (mvp Task 20/21 routing). Not caused by responsividade-correcao.

**F-4 — /mapa console error NG0951 (runtime error on map init)**
- URL: `/mapa`.
- Expected: clean console on navigation.
- Actual: `Falha ao inicializar mapa: RuntimeError: NG0951: Child query result is required but no value is available` — thrown from `_MapViewComponent.computed2 [as mapContainer]` during `ngOnInit`; caught and logged by the app's own error handler (map-view.component.ts:242). Shell still renders.
- Root cause: `viewChild.required` (`mapContainer`) read before the query is populated at init time.
- Attribution: PRE-EXISTING (mvp Task 18 map-view rewrite). Not caused by responsividade-correcao.

### Edge Cases Tested (5)
1. Corrupt persistence payload (`{corrupt`) — app boots clean, 0 pageerrors.
2. Dialog lifecycle — single overlay at all times across open/close/reopen cycles.
3. Horizontal overflow — checked on /personagens, /sessao, sheet, rules; zero overflows on all renderable pages.
4. PDF upload flow — exercised via real `setInputFiles` with a generated valid PDF (correct xref) → blocked by F-2 (NG0201) before reaching the upload.
5. /mapa runtime error — NG0951 confirmed reproducible (baseline + supplement runs).

### Evidence Files (in `.omo/evidence/final-qa/`)
- Reports: `qa-report.json` (baseline, 15 pass/1 fail), `qa-report-f3.json` (23 checks), `qa-report-f3-supplement.json` (6 checks)
- 27 screenshots: `01-home-page` … `06-404-redirect`, `task-1-persistence`, `task-1-corrupt-data`, `task-2-comecar-jornada`, `task-2-explorar`, `task-3-empty-state`, `task-3-novo-page`, `task-4-dice-dialog`, `task-6-quicksearch-mobile`, `task-6-quicksearch-desktop`, `task-6-global-search-mobile`, `task-7-cockpit-mobile`, `task-7-cockpit-desktop`, `task-8-sheet-mobile`, `task-9-rules-mobile` (broken-state), `task-10-dice-mobile`, `task-10-confirm-mobile`, `task-10-dice-desktop`, `task-10-dialogs-mobile`, `edge-mapa-page`
- Scripts kept: `playwright-qa.mjs` (unchanged), `qa-mobile.mjs`, `qa-supplement.mjs`

### Verdict Rationale
**APPROVE** for `responsividade-correcao`: every plan delta that is reachable at runtime passed real manual QA (persistence boot/reload + corrupt-data resilience, both Home CTAs, empty-state action, dialog lifecycle single-overlay, cockpit mobile/desktop stacking, sheet wrap, dialog widths ≤ viewport). The two plan scenarios that could not be runtime-verified (T6 QuickSearch width, T9 RulesReader) are blocked by pre-existing defects (F-2, F-3) that live in the never-committed mvp working tree — consistent with the corrected F4 attribution; the plan's own CSS deltas are statically verified and mathematically sound.
The QA gate additionally surfaced 4 real runtime defects (F-1…F-4) that predate this plan and are NOT regressions of its 20 tasks. They are documented precisely above for a follow-up work item (dice-roller input wiring, `IndexedDbFileRepository` provider, quick-search routing, map `viewChild.required` timing).

