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
