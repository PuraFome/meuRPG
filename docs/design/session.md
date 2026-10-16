# Live session

The session page shows what the master chose to show and what a player may read.
Every piece draws what the server sends (RN-10, RN-20); nothing hidden is drawn grey, it is absent.

## Frames and the meaning of burgundy

A 1 to 2 px `accent` frame means "this is on the players' screens now" or "this is the session": the shown image, the open scene, the session notice, the puzzle notice.
It always comes with a sentence and an icon beside it.
Other panels use a `line` border. A "burgundy border" is never used for anything else, including in-place questions.

## Images shown to players

- **"Imagem para os jogadores"** (`pages/live-session/shown-image-panel`): empty it says "Nenhuma imagem à mostra." with an outlined "Mostrar imagem"; showing, it has a 1.5 px `accent` frame, the thumbnail, the tag "Mostrando agora" and "Parar de mostrar" / "Trocar imagem".
  Stopping acts at once (showing again undoes it) and moves focus to "Mostrar imagem"; showing moves focus to "Parar de mostrar".
  Every change is announced in a `role="status"`.
- **"Deixar com os jogadores"** is a `role="switch"` that only asks: the server decides and the state comes from the response. The master's list "Deixadas com os jogadores" removes an image with "Tirar".
- **Players' blocks** (`shown-image-block`, `left-images-block`): "O mestre está mostrando" (an `accent` frame) and "Imagens que o mestre deixou" (a `line` border). The image frame has a fixed size so swapping a portrait for a landscape moves nothing; it never takes focus; a `role="status"` announces changes; "Ver em tela cheia" follows the state (if the master removes the image, it closes).
- **Picker** (`shared/gallery-picker/image-picker-dialog`): a 720 px dialog (full screen on a phone). An image that shows a whole map carries "Mapa inteiro"; where the picker shows it to players, choosing it asks first ("Mostrar o mapa inteiro?", "Voltar" with focus, "Mostrar mesmo assim"). An image that is the background of a hidden map carries "Fundo de mapa escondido".
- **A player's image only while it is shown, left with the players, or from a map they see** (RN-10): the screen keeps nothing for later.

## RP scenes, stage, clues and notes

- **Scene:** the master sees the DC and the result; the player only receives them when the master turns on "Mostrar a CD aos jogadores": off, the player's screen has neither the word "CD" nor "passou".
  Each action rolls up to its attempt limit (1 by default); after the last roll the row shows the result and "Sem mais tentativas".
- **While a combat is on screen the scene blocks are not drawn;** the scene stays open on the server and returns as it was when the combat ends.
- **Stage:** NPC portraits are transparent cut-outs resting on a baseline; whoever speaks is larger, with a bold name and "Fala agora"; empty, the stage disappears. Initials keep their frame.
- **Clues, hooks and notes:** every form that takes free text carries the "É ficção" notice; no player screen shows the hooks, an unrevealed clue or the name of an undiscovered scene.
  "Anotações" is a button in the shell (`shell/session-notes`) that opens a sheet; a clue that arrives shows a notice.
- **Session summary** (`shared/highlights`, `shared/session-summary`): one box per category with the number as the largest text; a tie names everyone. The player sees only "O combate acabou" with "Você" in words.

## Rolls

- **Dice animation** (`shared/roll-overlay`, mounted once in the shell) plays when this screen rolls in the app: attacks, damage, scene checks, saving throws, initiative, grapples, reactions, hit dice and the like.
  It never plays for a physical die the player typed, for another person's roll arriving by stream, or outside play (editors, campaign pages).
  A word ("Passou", "Falhou") appears only where the same screen tells it to this viewer; the DC and the other side's result are never shown.
  It is a native popover (`popover="manual"`) so it lies above open sheets; it is `aria-hidden` with one polite sentence when the dice land; focus does not move.
  The dice tumble together, land one after another (at most 8 drawn, the rest is a "+N" chip), stay until a tap or Esc or 10 s, and a new roll replaces them.
  Under `prefers-reduced-motion: reduce` (or without `matchMedia`) it does nothing.
- **Natural 20 and natural 1:** a check, saving throw, contest, group check, scene roll or initiative whose counting d20 came up 20 or 1 carries a small `mr-tag` "20 natural" or "1 natural" next to its formula (`naturalMark`, `core/combat/natural-mark.ts`).
  It is information only: those rolls have no automatic success or failure on a natural die, so the tag never says "crítico" or changes a verdict. Attacks keep their "crítico" wording; an attack that misses on a natural 1 reads "Errou · 1 natural".
  A player sees it only on a roll whose face they already read.
- **What a use costs:** a choice that spends something says what, in words, beside the button, with what is left ("Gasta 1 ki · restam N"), never an armor class or an NPC's number (RN-20).

## Experience and level up

- **Who receives** (`xp-recipients`, `app-check-box`): one row per character with the name, a lower line and "+116 XP" or "Não recebe" in words (the empty box is never the only signal).
- **Split line** (`xp-split`): the big number and the written calculation; the polite live region only says "116 XP para cada.".
- **The two buttons** (`xp-actions`): two real buttons of the same size, 44 px side by side on desktop, 48 px stacked on a phone, the filled one first; while invalid the filled one is dashed with the reason above it, linked by `aria-describedby`.
- Dialogs ("Dar XP", "Registrar marco") use `sheet-frame` so the title and footer stay in reach at 320 × 568.

## Traps, treasure and rests

- **Trap card, treasure card, "Quem notaria"** (`app-trap-noticers`): the table shows the server's numbers (passive Perception, the dim-light deduction, where the character is, whether they notice); the browser neither sums nor compares.
- **Waiting damage:** an amber card in an `aria-live` region with a number field ("Mude o número se houver resistência: o app não calcula."), the HP before and after, "Aplicar N de dano" (the only filled button) and "Não aplicar", which asks in place.
- **Rests** (`rest-card`): the master's "Descanso curto" / "Descanso longo" read the server's preview and open an `alertdialog`-style question in place that lists what comes back.
- **Treasure** (`pages/treasure`): generated by the server; "Pôr no mapa" asks for a hidden point; gold becomes XP only in a gold campaign.

## Puzzles

The player's and master's screens draw only what the server sends: no solution, minimum or victory maths in the browser.
`PuzzleHost` is the one place that knows which screen belongs to which kind and returns the move the server accepts.
The master's live view labels what only he knows with "Só você vê". The player's notice uses the session's burgundy frame and a 48 px "Abrir o quebra-cabeça"; the puzzle page opens inside the session page.

## Generated images

The "Gerar imagem" dialog (`shared/image-generate`) asks the server for an image, shows the result in the gallery (hidden, RN-10) and lets the master show or adjust it.
Each field states in one sentence what goes to Google and says not to write names of people; the SynthID watermark is stated once in the footer.
A generated image carries "Gerada por IA"; an adjusted one "Editada de Imagem 1".

## Campaign document and gallery

- **Gallery** (`pages/gallery`): cards with the thumbnail, name, size and where the image is used; deleting an image that is a map's background is refused by the server and the card names the maps. The filled button is "Enviar imagem", so the delete confirmation is outlined in `danger-ink`. The quota becomes a `--warning` notice from 90%.
  The image window is a native `<dialog>` with the app's only large shadow; an upload error notice uses the server's `reason`, never its message.
- **Campaign document** (`pages/campaign-document`): Markdown read and drawn by `app-markdown` (our parser, no `innerHTML`); `[text](map:<id>)` and `[text](character:<id>)` open windows; a deleted target becomes text; only `https://` links open in another tab. The editor keeps the draft in memory only and asks before leaving ("Sair sem salvar?", never `beforeunload`).
