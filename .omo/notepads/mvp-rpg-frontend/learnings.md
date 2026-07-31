## Task 3: Core Infrastructure

### Created

**Models** � character.ts, campaign.ts, gallery.ts, map.ts, session.ts, rules.ts, shared.ts + barrel export (src/app/core/models/)

**Store** � src/app/core/store/store.service.ts: generic StoreService<T> with BehaviorSubject map, events$ Subject, methods get/getAll/set/update/patch/delete/subscribe/snapshot

**Repositories** � base-repository.ts (interface), local-storage-repository.ts (Generic JSON serializer), indexed-db-file-repository.ts (binary files via IndexedDB)

**Services** � search.service.ts (BehaviorSubject-based index), sidebar.service.ts (sidebar items state), persistence.service.ts (store ? localStorage sync + search index)

**Routes** � app.routes.ts: / (HomeComponent), /personagens, /campanha, /galeria, /regras, /sessao, /mapa (all lazy via loadChildren), ** redirect

**Updated** � app.config.ts (provideRouter with withComponentInputBinding + withRouterConfig), app.ts (RouterOutlet)

### Verification
- npx ng build: SUCCESS (6 lazy chunks)
- npx vitest run: PASS (2/2)

## Task 19: Rules — PDF Reader with Bookmarks and Pagination

### Enhanced
- **RulesReaderComponent** (`src/app/features/rules/rules-reader.component.ts`): Complete reader overhaul

### Features Added
- **Bookmarks sidebar**: Collapsible left sidebar (toggle via bookmark button in toolbar) showing PDF outline/bookmarks extracted via `pdf.getOutline()`. Uses a flattened tree with indentation per level. Clicking a bookmark navigates to the corresponding page (handles both named destinations and explicit dest arrays with page ref resolution).
- **Zoom controls**: Zoom in/out buttons with granular steps (0.1 below 1x, 0.25 above 1x), zoom percentage indicator, zoom-to-fit button (resets to 1.0).
- **Page navigation**: Added First/Last page buttons using `first_page`/`last_page` icons alongside existing prev/next. Improved "Página X de Y" display.
- **Keyboard shortcuts**: `@HostListener('window:keydown')` — PageDown (next), PageUp (previous), Home (first), End (last). Skips when focus is on input/textarea.

### Technical Notes
- ng2-pdf-viewer uses its own bundled pdfjs-dist types (v3.x), causing `PDFDocumentProxy` type mismatch with top-level pdfjs-dist (v6.x). Solution: use `any` type for the pdf document reference and call methods directly.
- Bookmarks are flattened from the outline tree into `FlatBookmark[]` with `level` prop for indentation — avoids recursive component.
- `zoom` signal controls `[zoom]` input on pdf-viewer. `fit-to-page` is kept at `true`, `original-size` at `false`.
- Zoom percentage computed via `Math.round(zoom * 100) + '%'`.
- All data still loaded from `IndexedDbFileRepository` via `StoreService`.
- Pre-existing build issues in `character-detail.component.ts` (missing AvatarCropDialogComponent) unrelated to these changes.

## Task 21: Session — Quick Search Dialog

### Created

**session-quick-search.component.ts** � MatDialog-based quick search for the session page:
- Standalone component with FormsModule, MatDialog, MatFormField, MatInput, MatButton, MatIcon, MatList, MatChips
- Search input with 300ms debounce via Subject + debounceTime
- Three-way parallel search using StoreService.snapshot():
  - Personagens: character name match
  - Regras: rule book name match
  - Falas: character quotes content match
- Results grouped by category with section headers, icons, and colored category chips
- Inline detail view (toggle within same dialog) — no navigation away from /sessao
  - Character detail: avatar, type badge, description, history, master notes, attributes grid, skill chips, inventory, quotes with highlighted match
  - Rule detail: avatar, description, file info
- Escape key closes dialog or goes back from detail view
- Backdrop click and X button close
- `@HostListener('keydown', ['$event'])` for Escape inside dialog

**session.component.ts** � Updated with Ctrl+F keyboard shortcut:
- HostListener on `window:keydown` capturing ctrl+f / meta+f
- `event.preventDefault()` to suppress browser native find
- Search button in top bar for mouse access
- Opens SessionQuickSearchComponent via MatDialog with custom panelClass and backdropClass

## Task 17: Characters — Image Upload/Crop + Master Notes + Quotes

### Created
- **avatar-crop-dialog.component.ts** (`src/app/features/characters/`): MatDialog wrapper for existing ImageCropComponent. Receives imageSrc via MAT_DIALOG_DATA, sets aspectRatio=1 for square crop, closes dialog with cropped data URL string.

### Updated
- **character-detail.component.ts** (`src/app/features/characters/`): Three features added:

**Avatar Upload/Crop:**
- Header avatar container with conditional `img` (when imageUrl exists) or gradient placeholder letter fallback
- Camera button (mat-mini-fab) overlaid on avatar that triggers hidden file input (png/jpg/gif/webp)
- File read via FileReader → data URL → AvatarCropDialogComponent with ImageCropComponent
- Cropped data URL saved to character.imageUrl via StoreService.patch

**Notas do Mestre Tab:**
- Full Quill editor with same config as História tab (bold, italic, lists, headers)
- Debounced auto-save (1s) via separate `masterNotesSaveSubject`
- Conditionally visible via `isMasterRole$` BehaviorSubject (hardcoded to true for MVP)
- Lock icon + "Apenas o Mestre pode ver esta aba" message when master role is false

**Falas Tab:**
- Reactive FormArray with FormGroup per item: `texto` (required) + `contexto` (optional)
- Add/Remove buttons per quote
- Copy button per quote using `navigator.clipboard.writeText()`
- Debounced auto-save (1s) via `valueChanges` subscription + `quotesInitialized` flag
- Quotes stored as JSON-serialized strings `{"texto":"...","contexto":"..."}` in existing `quotes: string[]` model (no model file modification needed)
- Empty state when no quotes exist
- Validation error on empty `texto` when touched

### Technical Notes
- AvatarCropDialogComponent is opened dynamically via `this.dialog.open()` — not used in template, so not in `imports` array. TypeScript import is sufficient for dynamic instantiation.
- ImageCropComponent emits both data URL (string) and Blob on cropComplete. Dialog wrapper closes on the first string emission.
- Quotes FormArray valueChanges subscription uses a `quotesInitialized` guard to prevent saving during initial form population.
- All persisted via StoreService.patch('characters', id, partial) with updatedAt timestamp.
- MatDialogModule, MatFormFieldModule, MatInputModule, MatTooltipModule, ReactiveFormsModule added to component imports.
- Spec updated with MatDialogModule import and extended mock types for new character fields.

### Verification
- npx ng build: SUCCESS (no new errors/warnings)
- npx vitest run: PASS (33/33, 8 test files)

## Task 18: Map — Submap Pins

### Created

**submap-pin-dialog.component.ts** � MatDialog-based pin creation:
- Form fields: name, target map select (from StoreService maps list, excluding current map), color swatches (10 colors), icon select (10 Material Icons)
- Data from dialog includes x/y coords passed in, returns SubmapPin with label, targetMapId, icon, color
- Validates name and target are set before enabling confirm

### Enhanced

**map.service.ts** � Added submap pin management:
- Injected StoreService<MapData> for 'maps' collection access
- `setCurrentMapId/getCurrentMapId` — tracks which map is being viewed
- `getMapById(id)`, `getAllMaps()`, `getParentMapId(mapId)` — map lookup queries via store snapshot
- `getMapHierarchy(mapId)` — builds breadcrumb chain by walking parent chain
- `addSubmapPin(pin)`, `removeSubmapPin(pinId)` — mutates MapData.submaps array via store.update()
- `renderSubmapPins(pins)` — re-creates OL features on submapPinsLayer source
- `getOrCreateSubmapPinsLayer()` — lazy OL vector layer with purple (#7c4dff) circle style, distinct from red markers
- `toggleSubmapPins()` / `showSubmapPinsLayer()` — visibility control
- `onMapClick(handler)` — registers singleclick listener with proj.toLonLat conversion, returns unregister fn
- Pins stored in StoreService 'maps' collection via existing update/patch pattern

**map-view.component.ts** � Updated with submap UI:
- Added Router, ActivatedRoute, MatDialog, StoreService, PageHeaderComponent
- Route param `:id` loads specific map from store; no id = default empty map
- Toolbar row with "Adicionar Pin" button (toggle placement mode) and "Voltar ao mapa pai" button
- Pin placement mode: click map → SubmapPinDialog → MapService.addSubmapPin()
- Breadcrumbs rendered via PageHeaderComponent using MapService.getMapHierarchy()
- "Voltar ao mapa pai" button calls router.navigate(['/mapa', parentMapId])

**map.routes.ts** � Added `path: ':id'` route loading same MapViewComponent (handles both root and submap views)

**map-config-panel.component.ts** � Added "Pins de Submapa" checkbox calling toggleSubmapPins()

**map.ts model** � Extended SubmapPin with optional `icon?: string` and `color?: string` fields

### Verification
- Build: PASS (no errors from map changes, only pre-existing warnings in other modules)

## Task 20 � Session Split-Screen Layout + Toolbar

### Files Created
- src/app/features/session/session-cockpit.component.ts � main cockpit with resizable split panels
- src/app/features/session/session-toolbar.component.ts � toolbar with dice, map, music, play/pause buttons
- src/app/features/session/dice-roller-dialog.component.ts � dice notation dialog with animation + history

### Files Modified
- src/app/features/session/session.routes.ts � lazy-loads SessionCockpitComponent instead of stub
- src/app/features/session/session-quick-search.component.ts � added missing FormsModule import (pre-existing file)

### Design Decisions
- Draggable divider uses raw mouse events (mousedown/mousemove/mouseup) instead of CDK drag-drop
- Left panel width constrained between 25%-75% for usable area on both sides
- Session state persisted via StoreService with collection 'sessions' and fixed ID 'current-session'
- Dice roller animation: interval cycles random values for ~800ms before settling on actual rolls
- Dice notation parsed via regex /^(\d+)[dD](\d+)(?:([+-])(\d+))?$/ supporting NdS+M format
- Quick references pre-populated with 3 MVP entries (basic rules, conditions, action economy)
- Toolbar buttons emit events to parent cockpit for centralized action handling

### Observations
- MatButton content projection conflicts with @if blocks � fixed by interpolating text directly
- session-quick-search.component.ts already existed in session folder and needed FormsModule
- Build produces separate lazy chunk session-cockpit-component at ~22.75 kB

## Task 22: Global Search (Ctrl+K) Integration

### Created
- **search-highlight.pipe.ts** (`src/app/shared/search-modal/`): Standalone pipe using DomSanitizer to wrap matched search terms in `<mark class="search-highlight">` with XSS protection via regex escaping. Returns SafeHtml for `[innerHTML]` binding.

### Enhanced
- **search.service.ts** (`src/app/core/services/`): Added `searchGrouped(query)` method that returns `Observable<SearchResultGroup[]>` with results grouped by EntityType in display order, including label and icon per group.
- **search-modal.component.ts** (`src/app/shared/search-modal/`): Complete overhaul:
  - Flat results replaced with `groupedResults` computed signal that preserves score order within each type group
  - Section headers per group with icon + label + count badge
  - Group display order: Personagens → Campanha → Galeria → Regras → Mapas → Sessões
  - Search term highlighting via `SearchHighlightPipe` on both name and description (using `[innerHTML]` binding)
  - Entity-specific navigation: character → `/personagens/{id}`, map → `/mapa/{id}`, rules → `/regras/{id}`, others → module root
  - Color-coded type badges (purple=character, teal=campaign, orange=gallery, amber=rules, blue=map, pink=session)
  - Colored result icons matching type badge colors
  - Increased padding, rounded corners, hover states on result items
  - `::ng-deep .search-highlight` style for gold highlight with bold weight

### Technical Notes
- `SearchResultGroup` interface exported from search.service.ts with fields: type, label, icon, items
- Grouping done via `computed()` signal from flat `results` signal — preserves subscription pipeline from SearchService
- Entity ID extracted via `(result.entity as { id: string }).id` — all entity models have `id`
- Highlight pipe uses `DomSanitizer.bypassSecurityTrustHtml()` to allow `<mark>` tags through Angular sanitization
- All existing functionality preserved: 300ms debounce, auto-focus, Escape close, clear button, empty states
- No new npm dependencies added

## Task 23: Polish — Error States, Empty States, Loading States

### Added
**character-detail.component.ts**:
- `loading`/`error` signals for async state tracking
- LoadingSpinnerComponent while character loads (instead of inline text)
- EmptyStateComponent with "Personagem não encontrado" error + "Voltar" button for invalid IDs
- Zero-config Load → Error → Content flow with `@if/@else if/@else`

**map-view.component.ts**:
- `loading`/`error` signals for async state while OpenLayers initializes
- LoadingSpinnerComponent while map loads
- EmptyStateComponent with "Mapa não encontrado" error + "Voltar" button for invalid submap IDs
- Try/catch around mapService.initialize() to surface init errors
- Null-safe parentElement via captured variable for ResizeObserver

**rules-reader.component.ts**:
- Extracted `loadRule()` method from ngOnInit to support retry
- Added `retry()` method with URL cleanup for "Tentar Novamente" button
- Replaced inline error state with EmptyStateComponent (icon, message, action)
- Error messaging: "Regra não encontrada" (instead of "Livro de regras não encontrado")
- RouterLink removed (no longer used in template)

**campaign-tree.component.ts**:
- Replaced inline empty state div with EmptyStateComponent (`folder_off` icon)
- Added aria-labels to all folder action buttons (expand, associate, subfolder, rename, delete)

**gallery-grid.component.ts**:
- Replaced inline mat-progress-spinner with LoadingSpinnerComponent (`Carregando galeria...`)

**session-cockpit.component.ts**:
- Replaced both inline empty states (left panel + references) with EmptyStateComponent
- Dynamic message based on search state via ternary binding

**Dialog animations** (fadeSlide `:enter` animation):
- search-modal.component.ts
- dice-roller-dialog.component.ts
- submap-pin-dialog.component.ts
- avatar-crop-dialog.component.ts
- confirm-dialog.component.ts

### Technical Notes
- `styles` property in @Component decorator can be a single backtick string (`styles: \`...\``) not an array. Adding `],` after it causes TS1136 error. Check before editing.
- EmptyStateComponent's `message` is typed as `string`, so `error()` returning `string | null` needs `error()!` non-null assertion in template (safe because it's inside `@else if (error())` block where it's guaranteed truthy).
- Observed: `signal()` doesn't narrow type in Angular templates — non-null assertions are needed for `string | null` signals used in `@else if` branches.
- `@angular/animations` was already installed (v21.2) and `provideAnimations()` already registered in `app.config.ts` from prior tasks.

### Verification
- npx ng build: SUCCESS (no new warnings/errors)

## Task 24: npm scripts + build optimization

### Added npm scripts (package.json)
- `"start"`: `"ng serve"` (already existed)
- `"build:prod"`: `"ng build --configuration production"` (explicit production build)
- `"test"`: `"vitest run"` (changed from `"vitest"` to run-once mode)
- `"test:watch"`: `"vitest --watch"` (already existed)
- `"lint"`: `"ng lint"` (placeholder for future lint setup)
- `"analyze"`: `"ng build --configuration production --stats-json"` (bundle analysis)

### angular.json production config verified
- `"optimization": true` — added explicitly
- `"sourceMap": false` — added explicitly
- `"outputHashing": "all"` — already set
- Budgets: initial `maximumWarning: "2MB"`, `maximumError: "3MB"` — already set
- `anyComponentStyle`: `maximumWarning: "16kB"`, `maximumError: "32kB"` — already set

### Production build result
- **Status**: ✅ PASS (zero errors)
- **Initial total**: 874.44 kB raw / 191.31 kB estimated transfer — well under 2MB budget
- **Warnings**: Only NG8107 extended diagnostics (optional chain on non-nullable types) — pre-existing, not errors
- **Build time**: 22.96 seconds
- **Lazy chunks**: All feature modules lazy-loaded (rules-reader 504 kB, campaign 91 kB, character-detail 81 kB, map-view 66 kB, gallery 37 kB, session-cockpit 22 kB, etc.)
