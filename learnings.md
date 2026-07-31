# Learnings

## Task 4 — App Shell (Layout, Sidebar, Global Search)

### Architecture
- **ShellComponent** (`src/app/layout/shell.component.ts`) wraps the entire app.
  - Uses Angular Material `MatSidenavContainer` with `MatSidenav` (240px, mode="side"/"over" based on Breakpoints.Handset) and `MatSidenavContent`.
  - Toolbar with hamburger toggle, title "MeuRPG", search button.
  - Sidebar nav links for all 6 modules: Personagens, Mapa, Campanha, Galeria, Regras, Sessão.
  - Cmd+K / Ctrl+K keyboard shortcut opens the search modal.
  - Responsive: sidebar collapses (mode="over") on handset screens; hamburger closes drawer on nav click.
- **SearchModalComponent** (`src/app/shared/search-modal/search-modal.component.ts`)
  - `MatDialog`-based global search.
  - 300ms debounce via `Subject.pipe(debounceTime(300), distinctUntilChanged(), switchMap(...))`.
  - Uses `SearchService.search()` to query indexed entities.
  - Navigates to entity module route on selection.
  - Auto-focuses input on open. Closes on Escape.
- Routes restructured: `ShellComponent` at `''` with children containing `HomeComponent` + all 6 lazy feature routes.
- `provideAnimations()` added to `app.config.ts`.
- `@angular/animations@21.2` installed.

### Angular Material M3 Theming
- Angular 21 uses the new M3 `mat.define-theme()` API (not legacy `mat.define-light-theme`).
- Available palettes in M3: `mat.$violet-palette`, `mat.$blue-palette`, `mat.$red-palette`, etc. (no `$indigo-palette`).
- Dark theme with violet primary palette.
- Theme imported via `@use '@angular/material' as mat` in `styles.scss`.
- Roboto font + Material Icons loaded via Google Fonts in `index.html`.

### Key Commits (5 atomic)
```
d65ad57 feat(config): add provideAnimations and install @angular/animations
108a2df feat(routes): wrap all routes under shell layout parent route
0f82224 feat(search): add global search modal with debounce and result navigation
ffd6dfb feat(layout): create app shell with responsive sidenav, toolbar, and nav links
4bc0a28 feat(theme): add Angular Material dark theme and font imports
```

### Blockers & Notes
- `@angular/animations` must be installed separately — not bundled with `@angular/material`.
- `BreakpointObserver` from `@angular/cdk/layout` is used for responsive behavior (handset detection).
- `MatDialogRef` reference tracked to prevent duplicate search modal opens.
- The search modal navigates to module root routes (e.g., `/personagens`) since detail routes aren't built yet.
- Test `app.spec.ts` still passes because it provides its own simplified routes.

## Task 5 — Shared UI Components

### Components Created (7 standalone, tree-shakeable)

| Component | File | Key Dependencies |
|-----------|------|-----------------|
| **ConfirmDialog** | `components/confirm-dialog.component.ts` | `MatDialogModule`, `MatButtonModule` |
| **EmptyState** | `components/empty-state.component.ts` | `MatIconModule`, `MatButtonModule` |
| **LoadingSpinner** | `components/loading-spinner.component.ts` | `MatProgressSpinnerModule` |
| **FileUpload** | `components/file-upload.component.ts` | native HTML5 drag-drop, `MatIconModule`, `MatButtonModule` |
| **AudioPlayer** | `components/audio-player.component.ts` | native `<audio>`, range sliders, `MatIconModule`, `MatButtonModule` |
| **ImageCrop** | `components/image-crop.component.ts` | Canvas API, mouse drag selection, `MatButtonModule`, `MatIconModule` |
| **PageHeader** | `components/page-header.component.ts` | `MatToolbarModule`, `RouterLink`, `<ng-content select=\"[actions]\">` |

### Architecture Notes
- **Barrel**: `shared/index.ts` exports all 7 components + the existing `SearchModalComponent`
- **FileUpload** uses native HTML5 DragEvent API (not `@angular/cdk/drag-drop`) since CdkDrag/CdkDropList don't provide file `DataTransfer` access; CSS classes for drag-over visual feedback
- **ImageCrop** uses a click-and-drag selection rectangle; aspect ratio constraint via `@Input() aspectRatio`; emits both `Blob` and `data URL` via `cropComplete` EventEmitter
- **AudioPlayer** uses `get` accessors for `currentTime` and `duration` (not methods) — Angular templates must call getters without `()`
- All components use `signal()` for reactive internal state (isDragOver, previewUrl, playing, etc.)
- Tests use `@testing-library/angular` for rendering and `@testing-library/user-event` for realistic event simulation
- `@testing-library/user-event` was not in the original dependency list — had to be installed separately

### Component Interfaces
- **ConfirmDialogData**: `{ title, message, confirmText?, cancelText? }` — returns `boolean` via dialogRef.close
- **EmptyState**: `@Input() icon = 'inbox'`, `@Input() message`, `@Input() actionLabel?`, `@Output() action`
- **LoadingSpinner**: `@Input() isLoading`, `@Input() message?`
- **FileUpload**: `@Input() acceptedTypes: string[]`, `@Input() maxSize = 10MB`, `@Output() fileChange`
- **AudioPlayer**: `@Input() src`, `@Input() title?`
- **ImageCrop**: `@Input() imageSrc`, `@Input() aspectRatio?`, `@Output() cropComplete`
- **PageHeader**: `@Input() title`, `@Input() breadcrumbs: BreadcrumbItem[]`, `<ng-content select=\"[actions]\">`

### Test Results
- **EmptyState**: 6 tests (message renders, default icon, custom icon, hidden button, visible button, click emits event)
- **ConfirmDialog**: 5 tests (title/message, default labels, custom labels, confirm returns true, cancel returns false)
- Total: 13 tests across 4 suites, all passing

## Task 6 — Characters: Routing + List View (TDD)

### Files Created/Modified
- `src/app/features/characters/character-list.component.ts` — Card grid with type filters, search, empty/loading states
- `src/app/features/characters/character-list.component.spec.ts` — 4 TDD tests (empty, render, type filter, search filter)
- `src/app/features/characters/characters.routes.ts` — Routes for `/` (list), `/novo` (placeholder), `/:id` (placeholder)
- `src/app/features/gallery/gallery-grid.component.ts` — Fixed pre-existing TS2678 type error (WritableSignal<T> in switch)

### CharacterListComponent Architecture
- **Data flow**: `StoreService<Character>` via `getAll('characters')` → `shareReplay()` (shared across loading/filtered)
- **Loading state**: `loading$` derived from `allCharacters$` with `startWith(true)` / `map(() => false)` — avoids circular dependency where filtered content (inside `@else` branch) never gets subscribed
- **Type filter**: `BehaviorSubject<string>('all')` bound to `<mat-chip-listbox>` with (Todos, NPC, Jogador, Boss)
- **Search**: `Subject<string>` → `debounceTime(300)` → `distinctUntilChanged()` → `startWith('')` piped into `combineLatest`
- **Filtering**: `combineLatest([allCharacters$, typeFilter$, search$])` with `map` applying both filters

### Design System
- **Color tokens** (CSS in component):
  - Avatar gradients: player (emerald), NPC (cyan), boss (red)
  - Type badges: player `#059669`, NPC `#0891b2`, boss `#dc2626`
  - Page title: gradient `#e0e0e0 → #b388ff` (gradient text)
- **Grid**: `repeat(auto-fill, minmax(280px, 1fr))` responsive card grid
- **Cards**: 12px border-radius, subtle border, lift-on-hover with `translateY(-4px)` + shadow

### Critical Lessons
1. **Catch-22 with loading state and async pipe**: If `loading$` is derived from `allCharacters$`, but `allCharacters$` is only subscribed inside `filteredCharacters$` (via `combineLatest`), and `filteredCharacters$` is in the `@else` branch (which only activates when `loading$` is `false`), you get a deadlock. **Solution**: Use `shareReplay()` on `allCharacters$` and derive `loading$` independently so it subscribes at the top level.
2. **TypeScript 5.9 + Angular WritableSignal**: Calling `someSignal()` in a `switch` expression may produce `TS2678` because TypeScript doesn't properly narrow the callable type. **Workaround**: Extract to a local variable first (`const val = someSignal(); switch(val) { ... }`).
3. **Testing chips with duplicate text**: `mat-chip-option` text and character type badge text may overlap ("NPC" appears both as a chip label and a badge). Use `getAllByText('NPC')[0]` to target the chip specifically.

### Test Results
- **CharacterListComponent**: 4 tests (empty state renders, characters render, type chip filters, search input filters)
- **Build**: `ng build` passes (fixed gallery-grid TS error)
- All 4 tests passing
