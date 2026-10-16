# Design tokens

Tokens are CSS custom properties on `html` in `web/src/styles.scss`, prefixed `--mr-`.
Each colour is one `light-dark()` declaration, so a token works in both themes (`color-scheme: light dark` follows the operating system).
Components use tokens and never a raw colour.

## Colour

| Token | Light | Dark | Use |
| --- | --- | --- | --- |
| `--mr-ground` | `#eef0f3` | `#11151c` | Page background |
| `--mr-surface` | `#ffffff` | `#1a1f29` | Panels, app bar, fields |
| `--mr-line` | `#d5dae1` | `#2e3542` | Panel border (decorative) |
| `--mr-rule` | `#e3e7ec` | `#262c38` | Divider between a panel's rows |
| `--mr-ink` | `#1b2230` | `#e6e9ef` | Text, medallion border, filled proficiency dot |
| `--mr-ink-muted` | `#525b6b` | `#a3abb9` | Labels and supporting text |
| `--mr-control-line` | `#78818f` | `#7d879a` | Field border and the outline of every unchecked box, ring, switch and spell slot; at least 3:1 (WCAG 1.4.11) against card, page and `accent-soft` |
| `--mr-accent` | `#9e2b3b` | `#e88593` | Burgundy: main button, AC and HP outline, active tab, focus |
| `--mr-accent-strong` | `#7a1f2d` | `#f2a9b3` | Hover and pressed of the main button, link hover |
| `--mr-on-accent` | `#ffffff` | `#2a0a10` | Text on burgundy |
| `--mr-accent-text` | `#9e2b3b` | `#f0a0ab` | Links and text buttons |
| `--mr-accent-soft` | `#f6e3e6` | `#3a1e24` | Selected item |
| `--mr-focus` | `--mr-accent` | `--mr-accent` | The 2 px focus ring |
| `--mr-warning-surface` / `-line` / `-ink` | `#fbf3e2` / `#e6cf9f` / `#6e4700` | `#2e2616` / `#6b5320` / `#f0d08a` | Rule pendings, the Pendente state, warnings |
| `--mr-danger-surface` / `-ink` | `#fce8e6` / `#8c1d18` | `#3b1a1a` / `#f4b4ae` | Errors, the Morto state, destructive confirmations |
| `--mr-success-surface` / `-ink` | `#e3f2ea` / `#1f6b45` | `#16301f` / `#9ed8b4` | Confirmations, the Aprovado state |

### Map tokens

A map's picture is not themed, so what is drawn on it does not follow the theme either.
These are single values, the same in both themes.

| Token | Value | Use |
| --- | --- | --- |
| `--mr-map-token-ink` | `#1b2230` | Token letters and borders, grid, marks |
| `--mr-map-token-surface` | `#ffffff` | Token fill, label chips |
| `--mr-map-token-down`, `-down-line`, `-down-ink` | `#d5dae1`, `#8a93a3`, `#525b6b` | A defeated token |
| `--mr-map-accent` | `#9e2b3b` | Reach, chosen square, area templates |
| `--mr-map-fog` | `#000000` | Fog of war: what was never seen, and the darkening of what was |
| `--mr-map-sample` | `#a8977a` | A sample of a map's floor under the legend swatches |

## Fonts

Two OFL families, served by the app (`@fontsource/alegreya` and `@fontsource/alegreya-sans`, Latin subset, which covers Portuguese), woff2 only.
Never Google Fonts: the CSP (`font-src 'self'`) and [privacy](../privacy.md) forbid asking a third party for a font.

| Token | Family | Faces | Use |
| --- | --- | --- | --- |
| `--mr-font-display` | Alegreya, Georgia, serif | 500, 700, 800 | Headings (`h1` to `h3`), the game numbers, names |
| `--mr-font-sans` | Alegreya Sans, Segoe UI, system-ui, sans-serif | 400, 500, 700 | Body, labels, buttons |

Each `@font-face` carries `ascent-override` and `descent-override` (Alegreya Sans 92% and 28%, Alegreya 100.5% and 35.6%).
They put the middle of the capitals at the middle of the line without changing its height, so an icon centred next to words lines up with its capitals; `e2e/tests/layout.ts` measures it.

The base is `body`: Alegreya Sans, 16 px, line height 1.4, `font-variant-numeric: lining-nums tabular-nums` (bonus columns line up).
Headings use `text-wrap: balance`.

Sizes the shared classes fix (the rest belong to their component):

| Where | Size / line height / weight |
| --- | --- |
| `.mr-page-title` | 34 / 38 / 800 (38 / 40 from 768 px), Alegreya |
| `.mr-page-lead` | 17, `ink-muted` |
| `.mr-panel__title` | 21 / 26 / 700, Alegreya |
| `.mr-label` | 13 / 16 / 500, `ink-muted`, never in capitals |
| `.mr-list__name` / `.mr-list__sub` | 18 / 700 and 14 `ink-muted` |
| `.mr-tag` | 14, 26 px high |
| `.mr-notice` | 16 / 22 |
| Buttons | 16 / 700 (Material override) |

## Space and shape

| Token | Value |
| --- | --- |
| `--mr-space-1` to `-6` | 4, 8, 12, 16, 20, 24 px (`-1`, `-2`, `-3`, `-4`, `-5`, `-6`) |
| `--mr-space-8` | 48 px |
| `--mr-gutter` | the page's side margin: `--mr-space-4` (16 px), `--mr-space-8` (48 px) from 768 px |
| `--mr-radius-sm` | 8 px: buttons, fields, Material extra-small and small corners |
| `--mr-radius-md` | 10 px: notices |
| `--mr-radius-lg` | 12 px: panels, Material medium corner |
| `--mr-radius-xl` | 14 px: medallions, header, dialogs, bottom sheets, Material large corner |
| `--mr-radius-pill` | 9999 px: tags |

## Material mapping

`styles.scss` calls `mat.theme` with the red palette and points Material's system tokens at ours, so every Material component speaks the same palette:

- `primary` is `--mr-accent`; `on-primary` is `--mr-on-accent`; `primary-container` and `secondary-container` are `--mr-accent-soft`; `secondary` is `--mr-ink-muted`.
- `background`, `surface` and the `surface-container-*` family are `--mr-surface`, except `surface-variant` and `surface-container-highest`, which are `--mr-ground`.
- `outline` is `--mr-control-line`, `outline-variant` is `--mr-line`.
- `error` is `--mr-danger-ink` with `--mr-danger-surface` as container.
- Typography: `plain-family` is `--mr-font-sans`, `brand-family` is `--mr-font-display`; density 0.

Component overrides in the same file:

- **Buttons:** 8 px corners on every kind, weight 700, size 16, 44 px high (the filled one 48 px under 768 px).
- **Checkbox and radio:** the unchecked outline is `--mr-control-line` (`mat.checkbox-overrides`, `mat.radio-overrides`).
  A native checkbox or radio that stays visible takes the class `mr-check-input` ([components.md](components.md#global-classes)), because the browser's own unchecked box is nearly invisible in the dark theme.
- **Dialogs and bottom sheets:** 14 px corners; a bottom sheet keeps its top corners round at every width.
- **Sheet panel classes** (`panelClass` of `openSheet`): `.mr-sheet` lets the container use `100dvh − 24px`; `.mr-sheet-tall` leaves 84 px at the top (the combat log); `.mr-sheet-image` leaves 40 px (24 px under 360 px wide; the image-generation sheet); `.mr-sheet-over` is a dialog drawn like a sheet, to open over another sheet without closing it.
- **`.mr-nd-panel`:** the challenge-rating select of the NPC editor, six 44 px rows and then it scrolls.
