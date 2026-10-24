# Combat screens

The combat screens live in `pages/live-session/combat/` and `shared/combat-map`, `shared/combatant-token`.
The browser draws what the server sends (reach from `GetMoveOptions`, totals, verdicts); it computes no rule.

## Tokens

- **Two kinds, told apart by shape, never by colour:** a player character is an `ink` disc; an NPC is a rounded square with an `ink` border (28% corner); a character's creature is a round, hollow disc with a dashed `ink` ring.
- **The initial** (or two letters and the number of a copy, "G2") is in Alegreya 700. Sizes: 22 to 34 px on a map, 28 in lists, 48 in the bar.
- **The turn** is a 3 px `accent` ring outside. The player's own token has a 2 px `ink` halo and never the turn ring alone: a wolf's turn is the ring and the word "Vez". The dashed ring is never `accent`.
- **Defeated:** grey with ✕. **Hidden (master only):** dashed with the `visibility_off` seal, shown only when the map is at least 480 px wide.
- On a map the token uses the fixed `--mr-map-token-*` colours in both themes.

## The turn

- **Order and turn panel:** the player's turn is a 2 px `accent` frame ("Sua vez, Pensantus") with the four boxes Ação, Ação bônus, Reação and Movimento. "Disponível" is an open circle and the word, never a check; "Usada" is a dashed box with the name struck through.
  "Encerrar turno" is outlined until the action and bonus action are spent, and asks "Ainda tem ação disponível. Encerrar mesmo?" otherwise. A tap while the turn's options are being re-read waits for them (`aria-busy`).
- **Layout:** under 1024 px the big sheet and a fixed bottom bar; 1024 to 1279 px two columns; from 1280 px three columns (order, map and log, actions).
- **Action groups** (`action-groups`, `action-row`): one box per economy; each option is a row with name, pill, detail and a 48 px outlined button whose `aria-label` names the action. A disabled option has a dashed button with `aria-disabled` (still reachable), `block` and the reason linked by `aria-describedby`. A reaction that waits for its trigger has no button.
- **Turn alert** (`core/combat/turn-alert.ts`): when the turn passes to a combatant the player plays, the screen warns once: vibration where the browser has it, a two-note Web Audio chime (only after the first tap on the page), a 0.5 s shake of the turn panel (`.mr-nudge`) and, while the tab is hidden, the title "● Sua vez · MeuRPG". It never fires on page load, on a repeated stream update, or for the master's or an NPC's turn. Under reduced motion the shake is a still 2 px outline. The switch "Avisar na minha vez (som e vibração)" turns off sound and vibration only and is kept in memory.
- **Master's nudge:** "Chamar a atenção" (`app-nudge-button`) fires the alert on one player's screen and shows "O mestre está chamando você." (`app-calling-notice`), which leaves by itself after 10 s.
- **Joint turn:** combatants with the same initiative play one turn. The "Turno conjunto" box is a 1.5 px `control-line` frame (never a side stripe); each member shows "Ainda age" or "Encerrou" as icon and word; each player ends their own part ("Encerrar a minha parte", outlined, asks in place); there is no "end the group's turn".

## Distance and units

Every distance the screen writes comes from `core/units.ts`: 1 square = 1.5 m = 5 ft.
Displacement is "7,5 m · 5 quadrados"; a reach or the distance to a target is in meters (`metersText`).
The number and unit travel together (non-breaking space) and a "·" never starts a line.

## Rolling and results

- **Attack sheet:** a bottom sheet under 768 px, a 480 px dialog above (`openSheet`). Three steps (Alvo, Rolar, Dano); a target out of reach is dashed with `block` and the reason.
  `app-roll-picker` offers the two RN-18 ways; the typing field's border is its focus (2 px `accent`, 2 px `danger-ink` on error); "Confirmar 16" is dashed until valid.
- **Master's card** (`npc-card`): the screen's filled button is "Próximo turno"; "Rolar ataque" is outlined. The result shows the formula, the pill ("Acertou", "Crítico", "Errou") and "contra CA 18 do Toren"; "Aplicar N de dano" (2 px `accent` outline) with "Não aplicar", which asks on the row. Passing the turn with damage pending asks "Há dano sem aplicar. Passar o turno mesmo assim?". The roll buttons are off exactly when a roll would be refused.
- **Cast sheet:** levels are radio cards with `slot-dots`; a slot with none free is dashed with "Sem espaço livre"; the last slot warns in `warning-surface`. A spell that lets the caster pick a damage type adds the "Tipo de dano" row. A concentration spell cast over another says so above the button.
- **Area spells:** on a map with a grid the cast sheet has two steps, "Onde ela explode" / "Para onde?" then "Quem está na área". The outline moves in the browser (no call while moving); a release places it and the server's squares and effective origin replace it. The map is a `role="application"` with arrow keys, Enter, Escape and C.
- **Reactions and opportunity attacks** open an `alertdialog` by themselves, with no close button or Esc, and focus on the safe choice ("Deixar passar", "Não atacar"). The result shows only what the player may read: their own numbers, never an NPC's total or armor class.
- **Death saves:** the hero on their turn has a `danger-ink` frame, two rows of marks (success ✓, failure ✕, empty ring) and "Rolar teste contra a morte". Only the owner and the master see the marks (RN-24); other players read the state word. The master's confirmation of a death is a question in place with "Ainda não" focused.
- **Cover:** each target states its own cover with the swatch pictogram and its source ("Meia cobertura (do mapa)", "Três quartos (marcada pelo mestre)"), never an object's name or an AC; total cover from the map removes the target from the list, a total cover marked by the master shows dashed with the reason.
- **Conditions:** the 15 SRD conditions in 44 px boxes in a dialog; they become tags under the name and an `accent` dot on the token.

## The map

`app-combat-map`: the image with a 1 px SVG grid, a token per square, "Vez" above whoever plays; reach is a 30% `accent` tint on the squares the server says are reachable inside a dashed circle the size of the remaining movement.
The chosen square has a 3 px frame with a check (refused: dashed, `danger-ink`, `block`). The master drags any token; a player drags their own on desktop on their turn and the "Mover" page opens on that square (dragging never moves by itself). The map is one focus point: arrows move the choice, Enter confirms; a hidden list gives each position as words.

The "Mover", "Saltar" and "Ver mapa" page (`move-page`) draws the server's reach, the cost beside the chosen square (always one decimal) and the legend. A move that can provoke shows a `warning` notice and "Desengajar (gasta a ação)"; a move into a known trap asks in place.
Fog, layers and doors are in [maps.md](maps.md).

### Spell zones

The areas Spirit Guardians, Moonbeam, Cloudkill and Web keep on the map (RN-37) are a layer of `app-combat-map-card` (`app-zone-layer`), drawn from the squares the server sent, under the tokens and over the fog drawing; it takes no pointer and is `aria-hidden`, because the list is its accessible twin.
Each spell has a line pattern (vertical lines, dots, slanted lines, waves, triangles, horizontal lines) in one of the map's two inks, a 2.5 px outline with a white halo and a pill label "Raio Lunar · Bartolo" over the top edge (under the bottom edge at the top of the map), with a concentration icon. A zone of a hidden caster has a dashed outline and a crossed-eye icon on its label; only the master receives it.
The legend names each spell that has a zone, with the pattern's swatch ("Nuvem Mortal só você vê" when every zone of the spell is the master's alone).

"Zonas ativas (N)" (`app-zones-panel`) sits in the side column under the answers and the turn card, a `section` named by its heading, one `li` card per zone: the pattern swatch, "Raio Lunar · Bartolo", the area in meters, the save and damage ("Constituição CD 14 · 2d10 radiante, metade se passar", "4d4 cortante, sem teste"), "anda 3 m por turno", the notes, and chips "restam 9 rodadas" (from ten rounds, minutes; no chip with no count), "Concentração de Bartolo", "Aqui: Tor, Jabuti". A zone with hidden details (another caster's zone, for a player) shows the name, the area and the caster only. An empty list draws no card.
The master's buttons are 44 px: "Mover" (a zone that moves by an action) and "Encerrar zona", which asks in place ("Encerrar a zona?", "Não" holding the focus, "Sim, encerrar") and says a refusal under the question. A polite live region says "Zona criada: …" and "Zona encerrada: …" when the list changes, never on the first read.

## Gridless combat ("teatro da mente")

What depends on place does not exist without a map: no button is disabled "for lack of a map", and the mode's reason is written once ("Sem mapa, o app não sabe onde ninguém está.").
The master has a "Teatro da mente" tag, "Gastar movimento", the offer of an opportunity attack and a per-target cover editor; target lists drop distances and say "O mestre decide quem está ao alcance." No coordinate appears on any screen.

## Monsters, creatures and encounters

- **Bestiary and "Criar NPC"** (`pages/bestiary`) are master-only; others read "Só o mestre usa o bestiário da campanha."
- **Monsters in combat** are NPCs: the order row shows "ND 1/8" before the AC. XP at the end of combat lists each defeated creature ("Bandido 1 a 3 · ND 1/8 · 25 XP cada").
- **Encounter builder** (`pages/encounters`): the budget, total, band and maximum CR come from the server (`EvaluateEncounter`); the screen draws them. `app-count-stepper` and `app-hidden-switch` are its controls.
- **Creatures and Wild Shape** (`shared/wild-shape`, `shared/summon-sheet`): the server sends numbers and lists; a druid in a beast form has a "Na forma de Lobo" tag in the master's order; a concentration tag is one solid tag with the hourglass.
- **Familiar's eyes** (`shared/familiar-eyes`): the button appears for any familiar and the server decides whether it is in range; the question states the cost (blind and deaf; in combat it spends the action).

## End of combat

`combat-summary` is a `success-ink` frame with rounds, combatants and defeated; the master also sees "O grupo agora". In an enemies-XP campaign "Voltar à sessão" is outlined and the filled button is the XP one until XP is given.
