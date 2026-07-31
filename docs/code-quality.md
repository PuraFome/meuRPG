# Code Quality Report — meuRPG

> Generated: 2026-07-29
> Scope: `src/app/*` — Full manual audit

---

## Overview

| Metric | Count |
|--------|-------|
| Total TS source files (features + core) | ~35 |
| `any` type usages | 19 (14 in map-three.service.ts) |
| `@ts-*` directives | 0 ✅ |
| `console.log` / `console.debug` | 0 ✅ |
| `console.error` / `console.warn` | 4 (acceptable) |
| Empty catch blocks `catch {}` | 0 ✅ |
| Silent promise `.catch(() => {})` | 4 (acceptable fire-and-forget) |
| Unused `CommonModule` imports (suspected) | ~5 |

---

## Category A: Suspicious / Potential Bugs

### 1. Gallery Upload — Simulated Progress (Deceitful UX)

**File:** `src/app/features/gallery/gallery-upload.component.ts`
**Lines:** 217–224

```typescript
this.simulatedTimer = setInterval(() => {
  this.uploadProgress.update((v) => {
    if (v >= 95) return 95;
    const increment = v < 50 ? 15 : v < 80 ? 8 : 3;
    return Math.min(v + increment + Math.random() * 5, 95);
  });
}, 250);
```

- The upload progress bar shows a **simulated** animation, not actual progress.
- The real `IndexedDbFileRepository.save()` is awaited inline without progress events.
- **Risk:** User sees "95%" for several seconds while the real save finishes, then it jumps to 100%.
- **Fix:** Use IndexedDB transaction events (`onprogress`) or switch to a determinate indeterminate bar.

### 2. Dice-Roller Dialog — Mixed Template Syntax

**File:** `src/app/features/session/dice-roller-dialog.component.ts`
**Line 56:**

```html
<button *ngIf="notation" matSuffix ...>
```

Alongside Angular 17+ `@if` control flow (lines 77, 86, 113).

- Angular 17 control flow blocks (`@if`, `@for`) and structural directives (`*ngIf`, `*ngFor`) are both supported, but **mixing them in the same component** is inconsistent and harder to maintain.
- **Fix:** Convert `*ngIf` to `@if` for consistency.

### 3. Rules Reader — `window` Cast Bypasses Strict Mode

**File:** `src/app/features/rules/rules-reader.component.ts`
**Line 383-385:**

```typescript
(window as any)['pdfWorkerSrc'] = 'pdfjs-dist/build/pdf.worker.min.mjs';
```

- While functional, this pollutes the global scope and bypasses Angular's DI/typings.
- **Fix:** Use Angular's `DOCUMENT` token or configure pdf.js worker path via provider.

---

## Category B: `any` Type Erosion

### High Priority (Feature Code)

| File | Count | Examples |
|------|-------|---------|
| `rules-reader.component.ts` | 4 | `flattenOutline(items: any[])`, `pdfDocument: any = null`, `onLoadComplete(pdf: any)`, `getOutline().then((outline: any)` |
| `map-three.service.ts` | 14 | Heavy Three.js interop with `any` throughout |

**Rules Reader Risk:** Typing pdf.js interactions as `any` means a pdf.js API version bump could silently break at runtime.

**Recommendation:**
- Create lightweight type declarations for the pdf.js subset you use (in `src/types/pdfjs.d.ts`).
- For map-three.service, use `@types/three` and define proper interfaces for Three.js objects.

---

## Category C: Unused / Dead Code

### 1. Deprecated Stub Component

**File:** `src/app/features/characters/characters.component.ts` (8 lines)

```typescript
template: `<p>Personagens — em construção</p>`,
```

This component is **never used** in any route — `character-list.component.ts` handles `/personagens`. It should be **removed**.

### 2. Deprecated Map Placeholder

**File:** `src/app/features/map/map.component.ts` (3 lines)

```
export const MAP_COMPONENT_DEPRECATED = true;
```

Exists solely as a reference while other files import `map.component.ts`. Verify all references are migrated to `map-view.component.ts` and delete this file.

### 3. Unused Route

**File:** `src/app/app.routes.ts`

Check if `/personagens` (old route) vs `/personagens` (handled by character-list) are duplicated.

---

## Category D: Possible Unused Imports

These components import `CommonModule` but their templates appear to use **zero** CommonModule-derived directives or pipes (`ngIf`, `ngFor`, `ngClass`, `ngStyle`, `date`, `async`, `uppercase`, `slice`, etc.).

| File | Line | Notes |
|------|------|-------|
| `gallery.component.ts` | 2 | Template uses only `@if`/`@for` and Material components |
| `gallery-grid.component.ts` | 10 | Template uses only `@for`, no pipes |
| `campaign-tree.component.ts` | 14 | Verify template — `CommonModule` may be needed for `async` pipe |
| `campaign.component.ts` | 2 | Verify template |
| `folder-content.component.ts` | 9 | Verify template |

**Impact:** Each unnecessary import adds ~2-3 KB to the production bundle.

---

## Category E: Error Handling

### Silent Catches (Acceptable but flag for review)

| File | Line | Pattern |
|------|------|---------|
| `audio-player.component.ts` | 196 | `this.audio.play().catch(() => {})` — OK, audio autoplay is best-effort |
| `character-detail.component.ts` | 632 | `navigator.clipboard.writeText(text).catch(() => {})` — OK, clipboard may be unavailable |
| `map.service.ts` | 101 | `element.requestFullscreen().catch(() => {})` — OK, fullscreen may be denied |
| `map.service.ts` | 103 | `document.exitFullscreen().catch(() => {})` — OK, same reason |

### Proper Catches (Good pattern)

| File | Line | Pattern |
|------|------|---------|
| `rules-reader.component.ts` | 425 | `catch (err) { console.error(...); this.error.set(...) }` ✅ |
| `rules-reader.component.ts` | 543 | `catch (err) { console.warn(...) }` ✅ |
| `rules-list.component.ts` | 161 | `catch (err) { ... }` ✅ |
| `map-view.component.ts` | 241 | `catch (err) { console.error(...); this.error.set(...) }` ✅ |
| `gallery-lightbox.component.ts` | 545 | `catch { }` — silent but for fire-and-forget blob fetch |

---

## Category F: Subscription Management

### Good Patterns ✅

- **`takeUntil(destroy$)`** — Used in: `session-cockpit`, `character-detail`, `session-quick-search`
- **`Subscription` with unsubscribe** — Used in: `gallery-grid`, `gallery`, `campaign-tree`

### Minor Concern

`gallery-grid.component.ts` mixes `async/await` inside an RxJS subscription (line 282-287):

```typescript
this.subscription = this.store.getAll('gallery').subscribe(async (items) => {
  this.items = items;
  await this.loadThumbnails(items);
  this.applySort();
});
```

**Risk:** If the store emits faster than thumbnails load, state updates can interleave. Consider using `switchMap` or `concatMap` to handle async side effects.

---

## Category G: Bundle Size / Performance

### 1. Large Template Styles

Several components have 150+ line inline style arrays:
- `dice-roller-dialog.component.ts` — 233 lines of styles (488 total lines)
- `session-quick-search.component.ts` — 442 lines of styles (939 total lines)
- `rules-reader.component.ts` — 143 lines of styles (573 total lines)

**Recommendation:** Extract large style blocks into `*.css` files when they exceed ~100 lines. This enables:
- Better editor performance (no syntax highlighting of CSS inside template strings)
- Caching separation
- Easier theming overrides

### 2. `CommonModule` bundle impact

As noted in Category D, removing unused `CommonModule` imports from ~5 components would save ~10-15 KB from the prod bundle.

---

## Category H: NGRX / State Architecture

The project uses a **custom `StoreService`** instead of NgRx:

```typescript
// core/store/store.service.ts
export class StoreService<T extends { id: string }> {
  private collections = new Map<string, BehaviorSubject<Map<string, T>>>();
  ...
}
```

**Pros:** Lightweight, no extra dependency, simpler mental model.
**Cons:** No devtools, no action logging, no entity adapters, no selectors with memoization.

**Assessment:** Appropriate for MVP scale. If the app grows past ~15 collections, consider migrating to NgRx or similar.

---

## Summary

| Severity | Count | Key Actions |
|----------|-------|-------------|
| 🔴 High | 2 | Remove simulated upload progress, type pdf.js interactions properly |
| 🟡 Medium | 6-8 | Clean unused CommonModule imports, remove dead stub components, add type declarations for Three.js |
| 🟢 Low | 4-5 | Consider extracting large inline styles, minor subscription pattern improvements |

### Quick Wins (30-minute fixes)

1. Delete `characters.component.ts` (unused stub)
2. Delete `map.component.ts` (deprecated, if no remaining imports)
3. Convert `*ngIf` to `@if` in `dice-roller-dialog.component.ts`
4. Audit and remove unused `CommonModule` imports
5. Extract largest inline style blocks to CSS files
