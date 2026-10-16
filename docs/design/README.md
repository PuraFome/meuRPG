# Design: the look of MeuRPG

MeuRPG looks like a paper sheet ("ficha de papel"): the official D&D 5e character sheet, with the same order and the same shapes (ability medallions, the AC shield, the proficiency dots), clean on a screen, with a real light theme and a real dark theme.
This folder is the reference for anyone who touches a screen in `web/`.

| Page | What it holds |
| --- | --- |
| This page | Principles, the rules every screen follows, text on screen, accessibility, how a screen is made and reviewed |
| [tokens.md](tokens.md) | Every `--mr-*` token, the fonts, the Material mapping |
| [components.md](components.md) | The shared classes and components, and when to use each |
| [sheet.md](sheet.md) | The character sheet and the editors |
| [session.md](session.md) | The live session: shown image, scenes, rolls, resources, puzzles |
| [combat.md](combat.md) | The combat screens |
| [maps.md](maps.md) | Maps, layers, fog of war, the map editor and printing |

The same system is drawn in Claude Design, to design a new screen before the code: "MeuRPG — sistema visual" (the components) and "MeuRPG — direções visuais" (the explored directions).
Both are private links the project owner shares.
The UI text is Portuguese until i18n lands, so these English pages quote UI strings in Portuguese ("Sair dos outros dispositivos").

## Principles

- **The game number comes first.** Modifiers, AC, HP and DC are the most visible things on the sheet. Everything else is quieter.
- **The sheet stays where the player is used to it.** Abilities on the left, then saving throws and skills, combat and spells in the middle, features and story on the right (four columns from 1200 px).
  On a phone and a tablet the numbers used in play come first: abilities, the combat block, then saves and skills, features, and the player's notes last as one "Anotações (N)" row.
- **One bold move.** Burgundy (`--mr-accent`) is a fill only on the screen's main button.
  Otherwise it is an outline (AC, HP), text (links) or the "session" frame.
  No gradients, no decorative shadows, no card with a coloured stripe on the side.
- **Structure is information.** A border, a panel or a divider separates things that really differ.
  Numbering only where there is a sequence (the editor's steps).
- **Both themes count the same.** Every colour has a pair in both themes with AA contrast. The theme follows the operating system's.
- **The browser draws; the server decides.** A screen never computes a game rule, a reach, a total or a verdict: it draws what the API sends (RN-10, RN-20, RN-21).
  What a player must not read never reaches the browser; the screen does not hide it, it does not have it.

## Rules every screen follows

- **One filled button per screen.** `mat-flat-button` is the main action.
  Supporting actions are `mat-stroked-button`, text actions are `mat-button`.
  When two actions matter (the master looking at a pending character), the main one ("Aprovar personagem") is filled and the others ("Pedir ajustes", "Recusar personagem") are outlined.
  A screen whose actions repeat per row has none: the filled button sits where the decision is made ("Salvar ajuste").
  A destructive confirmation is outlined in `--mr-danger-ink`, so it never becomes a second filled button.
- **State is never colour alone.** Every tag has a word, every notice has an icon and text, a selected card has a border and a mark (check or filled disc), a hidden thing is dashed with a seal and a word.
- **A state tag is one word:** Rascunho, Pendente, Travada, Aprovado, Morto; invites Ativo, Usado, Expirado, Revogado.
  The notice beside it explains the rest.
- **A pending item is not a reminder.** What the rules engine flags as a problem (armor without proficiency, a spell outside the spellbook) becomes one notice under the sheet header, each item starting with the problem in bold.
  A reminder (advantage against a spell) stays quiet in "Características e traços".
- **An action that cannot be undone asks on the screen itself,** never in a browser `confirm`.
  The button turns into "Confirmar …" with "Cancelar" beside it.
  Focus goes to the safe choice, or to the new button where the question is only a confirmation ("Marcar como morto" becomes "Confirmar morte", focus on it).
  "Recusar personagem" opens "Recusar <name>?" with "Confirmar recusa" outlined in `danger-ink` and focus on "Cancelar".
  An undoable action (stopping the shown image, "Reviver") does not ask.
- **A double click never confirms.** The safe choice ("Voltar", "Cancelar", "Ainda não") sits where the opening button was, so the second click of a double click lands on it.
  A confirmation whose destructive choice appears where the opening button was marks its container `data-confirm`: for the first 500 ms its buttons ignore clicks and carry `aria-disabled="true"` (`core/confirm-guard`, `CONFIRM_GUARD_MS`).
  It is opt-in: a guard on every form-like container swallowed ordinary clicks.
- **A button that cannot act yet is dashed, quiet and still focusable** (`.mr-button--off` with `disabledInteractive`), and a sentence near it says why, linked with `aria-describedby`.
  What depends on something that does not exist (a map, a grid) is left out instead of disabled where the design says so.
- **A form in steps** (the sheet editor): each step is a tab with its name; the content starts with "Passo N de M" and the title.
  On a phone the tabs become 44 px numbers. The save button sits below the open step, in any step.
  A step with an invalid field gets "(com erro)" on its tab, and submitting leads to the first of them.
- **Counts say "usados":** "1 de 3 usados", for spell slots, hit dice and the adjustment.
- **Numbers and units never separate:** "7,5 m", "2.716 XP", "120 PO" travel with a non-breaking space (`tight()`, `joinDots()`, `formatInt`, `formatXp` in `core/format/text.ts`; distances from `core/units.ts`).
- **Borders, not shadows.** The only shadows are an open dialog's, the image window's, and the edge of a `sheet-frame` that has more to scroll.
- **Free text carries the fiction notice.** Every group where a person could type about a real person shows `<app-fiction-notice>`, and every "Enviar imagem" shows `<app-image-privacy-note>` ([Privacy](../privacy.md)).
- **Layout:** maximum width 1280 px; the sheet has four columns on desktop, two on a tablet, one on a phone; the phone layout is under 768 px.
  Touch targets are at least 44 px (48 px for the filled button on a phone).

## Text on screen

The UI text is Brazilian Portuguese until i18n lands; these rules are about that text.

- Terms of the official D&D translation ("Classe de Armadura", "Testes de resistência", "Pontos de vida").
- "habilidade" for ability (never "atributo"; never for a skill, "perícia", or a feature, "característica"); "nível da magia" for a spell level ("magia de 1º nível", never "círculo"; near the character's level write "nível da magia" and "nível do personagem"); "teste de resistência" for a saving throw (never "salvaguarda").
  The increase is the feature "Incremento no Valor de Habilidade".
- Sentence case ("Minhas campanhas"); labels never in capitals.
- The button says what happens ("Iniciar sessão", "Gerar convite"); the confirmation repeats the verb ("Convite gerado").
- An error says what happened and how to fix it, without apologising. An empty screen invites the next action.
- Numbers with a unit and a decimal comma: "7,5 m (25 pés)".
- Nothing internal on screen: no IDs, content versions, enum names or "Sem nome".
- No emoji.

## Accessibility

- **One focus ring:** `:focus-visible` draws 2 px `--mr-focus` at a 2 px offset (`web/src/styles.scss`); Material buttons get the same ring; `[data-ring]` shows it on an element the app moved focus to.
  An element with `tabindex="-1"` that the app focuses (a heading) draws no ring.
- **Page title:** `<page> · MeuRPG`, in Portuguese, first letter capitalised.
  Each route has a `title` (`app.routes.ts` and the child `*.routes.ts`); `page-title.spec.ts` fails for a route without one.
  A page that shows a campaign's or character's name calls `setPageSubject(() => name)` (`core/title/page-title.ts`): `Pensantus · Ficha · MeuRPG`.
- **"Pular para o conteúdo"** is the first link in `app.html`, hidden off-screen until focused, and takes focus to `<main id="main" tabindex="-1">`.
- **Focus on navigation:** after a navigation to another page, focus goes to the `h1` (or `<main>`).
  It does not move on first load, on a query or fragment change, when the page already focused something, or while a dialog or sheet is open.
- **Touch and keyboard:** every icon-only button has `aria-label`; a composite widget (radio cards, toolbars, segmented groups, maps) is one tab stop with arrow keys.
  The same actions exist without a pointer (a map has a list, a drag has arrows).
- **Motion:** only in response to an action, and nothing animates under `prefers-reduced-motion: reduce` (`styles.scss` shortens every animation; Playwright runs with `reducedMotion: 'reduce'`).
- **Live regions:** a result that appears is announced once in a polite `role="status"`; a refusal that needs attention is `role="alert"`.

## How a screen is made

Every new screen, and every visible change to a screen, goes through five steps (they are also in [CONTRIBUTING](../../CONTRIBUTING.md#screens-design-and-review)).

1. **Brief:** the screen's single task, who uses it (master or player), where (phone at the table or laptop), the real data and every state (empty, loading, error, locked, pending, dead, no permission).
2. **Design in Claude Design** with the system above, at 390 px and 1280 px, with real data, approved before the code. A small fix inside the system needs no design.
3. **Code with the tokens** ([tokens.md](tokens.md)) and the shared pieces ([components.md](components.md)), never a hand-written colour.
4. **Review on screen:** screenshots at 390, 768 and 1280 px, light and dark, every state, against the design and the PR checklist (hierarchy, alignment, overlap, touch targets, focus, contrast, text).
   Look at each piece up close in a 2x screenshot: an icon 1 px off its words or text glued to a border does not show in a shrunken screenshot.
   The PR carries before and after screenshots.
5. **Automatic check:** `e2e/tests/a11y.spec.ts` runs axe (WCAG 2.1 A and AA, serious and critical fail) and `expectAligned` from `e2e/tests/layout.ts` on every screen and state it visits. Every new screen goes into it.

`layout.ts` does not demand that everything be centred (the design decides).
It catches what is never right:

- an icon whose middle is more than 1 px off the middle of the capitals of its own words;
- a button's or chip's content off its middle (2 px);
- text running over an icon by more than 2 px;
- words closer than 6 px to the top or bottom border of a radio or checkbox tile;
- a button or link outside the box of its `.mr-panel`.

An icon next to words needs no hand adjustment: the fonts carry metric overrides ([tokens.md](tokens.md#fonts)).
On one line, centre both (`align-items: center`); when the text can wrap, align to the top and give the icon `margin-top` = (line height − icon size) / 2.

## Logo, icons and footer

- **Logo:** an original d20 seen from the front, front face in burgundy; no publisher's trademark or trade dress.
  The app bar shows the name alone in Alegreya 800; the d20 shows in the footer, the tab and the phone's home screen.
  The files are in `web/public/` (`favicon.ico`, `favicon.svg`, `apple-touch-icon-180.png`, `icon-192.png`, `icon-512.png`, `icon-maskable-512.png`, `site.webmanifest`) and linked from `web/src/index.html`, which sets `theme-color` twice (light and dark).
- **Icons:** Material Symbols Outlined, served by the app (`@material-symbols/font-400`), 16 to 24 px, in the colour of the text beside them.
- **Footer** (`shell/app-footer`): a quiet strip after every page, in the flow (never fixed): a 1 px `line` rule, the one-colour d20 and "© <year> MeuRPG", and the links "Termos de uso" (`/terms`), "Privacidade" (`/privacy`), "Créditos" (`/credits`) with 44 px targets; the open page has `aria-current="page"`.
- **Legal pages** (`pages/legal`): `/terms` and `/privacy` are public (Google's consent screen links to them).
  A reading layout: title, last-update line, a summary box ("Em resumo"), a table of contents ("Nesta página", `app-legal-toc`, open beside the text from 768 px and closed on a phone) and an article card of at most 760 px.
