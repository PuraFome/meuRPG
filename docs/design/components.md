# Components

A piece shared by several screens is a global class in `web/src/styles/_ui.scss` or a component in `web/src/app/shared/` (or `shell/`); what belongs to one screen stays in that screen's component.
Check here before drawing a panel, tag, notice, switch or sheet: the same thing must look the same everywhere.

## Global classes

Defined in `web/src/styles/_ui.scss`.

| Class | Use |
| --- | --- |
| `.mr-page-title`, `.mr-page-lead` | The page's `h1` and the line under it |
| `.mr-panel`, `.mr-panel__title` | One section of a page: surface, 1 px `line` border, 12 px corners, `h2` title |
| `.mr-list`, `.mr-list__row`, `__text`, `__name`, `__sub` | Rows in a panel (characters, NPCs, invites): at least 56 px high, a rule between rows; `<a class="mr-list__row">` highlights the name on hover |
| `.mr-tag` (`--pending`, `--success`, `--danger`) | A state in one word, a pill with a border in its colour; a `mat-icon` inside is 14 px (Travada takes `lock`) |
| `.mr-notice` (`--warning`, `--danger`, `--success`, `--neutral`) | A message with a `mat-icon` and a `<p>` that starts with `<strong>` naming the problem |
| `.mr-muted`, `.mr-label` | Supporting text; a 13 px label |
| `.mr-visually-hidden` | Text for screen readers only |
| `.mr-button--off` | A button that cannot act yet: dashed, quiet, still focusable (use with `disabledInteractive`), with the reason in a line linked by `aria-describedby` |
| `.mr-check-input` | A visible native checkbox or radio, drawn with `--mr-control-line`; never on a visually hidden input |
| `.mr-legend`, `.mr-swatch` (+ modifiers) | The legend under a map and its 22 px swatches; see [maps.md](maps.md#legend) |
| `.mr-nudge`, `.mr-nudge--still` | The half-second shake of the turn panel (`SHAKE_MS` in `core/combat/turn-alert.ts`); `--still` is the static 2 px outline used under reduced motion |
| `[data-ring]` | Gives the focus ring to an element the app focused (`core/creatures/focus-ring.ts`) |

## Shell components

| Component | Use |
| --- | --- |
| `<app-footer>` (`shell/app-footer`) | The strip with the d20, the year and the legal links, after every page |
| `<app-user-menu>` (`shell/user-menu`) | The account menu in the app bar ("Minha conta" until a name is set) |
| `<app-live-notice>` (`shell/live-notice`) | "A sessão 4 de Mirathel começou." for people who play in the campaign, under the app bar, outside that campaign's pages. A notice on `surface` with a 1.5 px `accent` border (a burgundy frame means "session"), `role="status"`, an outlined "Entrar na sessão" and a 44 px "Fechar aviso"; closing holds for the tab. One notice per open session, oldest on top; a new one enters below, so nothing moves under a finger |
| `<app-live-pill>` (`shared/live-pill`) | "Ao vivo": an 8 px `accent` dot and the word in a bordered pill; it does not blink. In the app bar it links to the newest open session (hidden on session pages); in "Minhas campanhas" it becomes the tag "Sessão ao vivo" |

## Sheets, dialogs and in-place questions

- **`openSheet`** (`shared/sheet/sheet-host.ts`) opens one component as a bottom sheet under 768 px and as a dialog from a tablet up. The component asks `injectSheet` for its data and for closing.
  Options: `ariaLabel`, `width`, `tall`, `panelClass`, `alert` (an `alertdialog` that Esc and the backdrop do not close; focus starts on `[data-initial-focus]`, the safe button), `focus` (a selector for a form's first field) and `restoreFocus`.
  Focus starts on the title so a stray Enter cannot roll or confirm.
- **`<app-sheet-frame>`** (`shared/sheet/sheet-frame`) is the frame of every sheet that scrolls inside itself: grab bar on a phone, title (the first focus, `tabindex="-1"`), a close button, a scrolling body and a footer with the actions that never scrolls away, so "Conjurar" stays in reach on a 320 × 568 phone.
- **`<app-pair-foot>`** (`shared/pair-foot`) is the pair of buttons at the foot of a sheet, an in-place form or a dialog: the outlined "Cancelar" or "Voltar" and the one filled button that says what happens.
  176 px wide and right-aligned from a tablet up; on a phone they share the width at 48 px and stack when a label needs more than half of it. With `ready` false the filled one is dashed and `aria-disabled`.
- **`<app-map-ask>`** (`shared/map-ask`) is the in-place question of the map editor ("Mudar a grade?"): it opens inside the panel of whoever asked, focus on its title, an outlined "Voltar" first and the filled button, which says what it erases, under it; Esc closes it.
- **Number adjustment** (`pages/live-session/adjust-vitals`, `combat/adjust-npc`): a bottom sheet on a phone and a 440 px dialog on desktop. Step buttons around a field where typing sets the value ("−5 −1 [field] +1 +5" for HP, "− [field] +" for the rest). At a limit the button is disabled and the words beside it say why ("máximo 24"). A value outside the limit is not silently corrected: the field says "Use um número de 0 a 24." and "Salvar ajuste", the only filled button, does not submit. The numbers scroll while the title and the action row stay in view.
- **Full-screen maps:** every page that can go full screen injects `FullscreenOverlay` (`core/ui`), which moves the CDK overlay container into the full-screen element so dialogs and sheets keep working.

## Form pieces

| Component | Use |
| --- | --- |
| `<app-text-field>`, `<app-select-field>`, `<app-switch-field>`, `<app-check-row>`, `<app-number-stepper>`, `<app-pick-list>` (`shared/form-fields`) | The content editors' fields: outlined, the refusal under the field, `data-field` for where a server refusal lands. A native `<select>` so the phone's picker opens. Switches carry "Ligado" / "Desligado". Rows are 44 px targets |
| `<app-check-box>` (`shared/check-box`) | A 24 px box in a 44 px target for a list of choices (who joins the combat, who gets the XP) |
| `<app-reveal-switch>`, `<app-hidden-switch>` (`shared/reveal-switch`, `shared/hidden-switch`) | A real `role="switch"`: an `ink` track with a check when on, a `control-line` outline when off (burgundy is for the main button only), always with the word |
| `<app-count-stepper>` (`shared/count-stepper`) | − and + at 44 px, never shrinking; `minusLabel`, `plusLabel`, `valueLabel` change the spoken phrases |
| `<app-segmented>` (`shared/segmented`) | "Andar \| Saltar": radios drawn as segments, 48 px on a phone and 44 px from a tablet, a check on the chosen one; a segment can be `disabled` (dashed, muted) |
| `<app-dice-choice>` (`shared/dice-choice`) | Radio cards (64 px minimum) for how dice are rolled: the master's "Dados" panel and the player's "Como você rola os dados". The chosen card has a 2 px `accent` border, `accent-soft` and the filled disc; a locked group dashes the card that does not apply ("Indisponível nesta campanha.") |
| `<app-method-cards>` (`shared/method-cards`) | Radio cards for "Como definir os valores" |
| `<app-roll-picker>` (`shared/roll-picker`) | The two ways to roll one die (RN-18): "Rolar no app" and "Digitar o resultado". The saved preference is the filled button, the other an underlined text; a forced mode hides the other. Typing shows a 64 px field, a live total in a `role="status"` and "Confirmar 16", dashed until valid |
| `<app-gallery-picker>` (`shared/gallery-picker`) | A `radiogroup` of gallery thumbnails; the chosen one has a 2 px `accent` border and a check; the last, dashed block is "Enviar imagem". The surrounding form supplies the visible label and the error |
| `<app-effect-picker>`, `<app-feature-editor>`, `<app-feat-picker>` | The table-content editors' pieces |

## Display pieces

| Component | Use |
| --- | --- |
| `<app-fiction-notice>` | The fixed "É ficção" line next to free-text groups |
| `<app-image-privacy-note>` | The fixed privacy line next to every "Enviar imagem" |
| `<app-effect-pill>` (`shared/effect-pill`) | A lasting effect under a vitals card: icon and sentence in a pill; tones `shield`, `accent`, `aid`, `down`, `ok`, `neutral` |
| `<app-table-mark>` (`shared/table-mark`) | "Da mesa", "Arquivada", "Desligada para os jogadores" beside an entry of the table's own content |
| `<app-slot-dots>` (`shared/slot-dots`) | Spell slots: a hollow ring for a free slot, a filled crossed-out disc for a used one; decorative, the row carries the words ("2 de 4 usados", "1 livre de 4") |
| `<app-count-pips>` (`shared/count-pips`) | A filled `accent` dot per use left and a ring per use spent; decorative, up to 6 |
| `<app-portrait>` (`shared/portrait`) | An NPC's portrait in a 5:6 frame, falling back to initials |
| `<app-live-toast>` (`shared/live-toast`) | Session toasts: a green notice with a 44 px ✕ in an always-present polite live region |
| `<app-markdown>` (`shared/markdown`) | Our own Markdown parser and renderer: no `innerHTML`, no dependency |
| `<app-rule-text>`, `<app-srd-text>`, `app-description-lang-button` | The SRD text in our Portuguese translation, with a "Ver em inglês" text button (44 px) that flips every SRD description until reload (`DescriptionLanguage`); the English is in a `lang="en"` block marked "(em inglês)". An entry with no translation shows the English, flagged |
| `<app-spell-help>` and `openSpellDetails` (`shared/spell-details`) | The 44 px "?" and the spell description: a bottom sheet on a phone, a 560 px dialog from a tablet; over another sheet on a phone it is a dialog drawn as a sheet, so the one below stays open. Title and "Fechar" stay in view; only the text scrolls |
| `img[appRetryImage]` (`shared/retry-image`) | Asks again after 2, 4, 8, 16 and 30 s for an image the server throttled (`429`/`503`) |

## Map and combat pieces

Documented with their screens: `map-view`, `map-layers`, `map-pins`, `fog-map`, `point-sheet`, `map-lists`, `dungeon-preview`, `door-crop` in [maps.md](maps.md); `combat-map`, `combatant-token`, `area-picker`, `start-combat`, `creature-pick`, `revive`, `familiar-eyes`, `wild-shape`, `summon-sheet` in [combat.md](combat.md); `roll-overlay`, `highlights`, `session-summary`, `xp`, `puzzle-boards`, `puzzles-panel`, `notes`, `trap-noticers`, `treasure-facts` in [session.md](session.md); `sheet` pieces in [sheet.md](sheet.md).

## Lists, cards and rows

- **Radio cards** (target choice, slot level, method, dice mode): 64 px minimum (52 px stacked on a phone), bold title and an `ink-muted` line, text away from the card edges (`layout.ts` enforces 6 px). The chosen card has a 2 px `accent` border, `accent-soft` and a mark.
  A card that cannot be chosen is dashed with the `block` icon and the reason in words.
- **Search field** in a list (`spell-picker`): outlined, with the magnifier and its hint inside the field (`placeholder`), no floating label (a floating label next to an icon is positioned by measuring the icon and can end up over it); the field's name goes in `aria-label`.
- **Profile sessions** (`pages/profile`): "Sessões" panel with "Sair dos outros dispositivos"; the confirmation is on the screen ("Confirmar saída" outlined in `danger-ink`, focus on it; "Cancelar" returns focus to the opener); the result is an `mr-notice--success` with `role="status"`.
- **Attack card of an NPC** (`character-editor/npc-short-form`): up to three attacks; "Adicionar ataque" is disabled on the third and says "Máximo de 3 ataques".
