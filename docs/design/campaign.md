# Design: the campaign page

`/campaigns/:id` is where a campaign is run from.
The master's page has a session strip and three tabs; a player's page is one page.
The code is `web/src/app/pages/campaign-detail`; the routes and the shared services are in [Architecture: web](../architecture/web.md#the-campaign-pages-tabs).

## The master's page

The first sentence of each part says what it is for.

| Part | What it holds |
| --- | --- |
| Header | The back link "Minhas campanhas", the `h1` with the campaign's name and the lead ("Você é mestre nesta campanha. XP por inimigos derrotados.") |
| Session strip | The session, on every tab |
| Tabs | A nav named "Seções da campanha" with "Mesa", "Preparação" and "Regras" |
| The open tab | Panels in two columns (3fr and 2fr) from 1024 px, one column below |

### The session strip

The strip is a `surface` panel with a 1.5 px `accent` border, above the tabs, so the session is in reach on every tab.
It says the session in one row: the "Ao vivo" pill and "Sessão 4 em andamento, desde 20:12." (the clock for a session of today, the day and the clock for an older one).
At the end of the row, in this order: "Encerrar sessão", a hairline, "Copiar link da sessão" with its icon, and "Entrar na sessão", which is the page's one filled button.
With no open session the row says "Nenhuma sessão em andamento. Ao iniciar, a ficha dos jogadores trava." and "Iniciar sessão" is the filled button.
"Encerrar sessão" asks in place ("Encerrar a sessão 4? Ela não reabre depois.", "Confirmar encerramento" outlined in `danger-ink`, "Cancelar"), and "Iniciar sessão" lists the sheets with choices open under the strip and asks "Iniciar mesmo assim?".
On a phone (under 768 px) the text is one line ("Sessão 4 desde 20:12"), and the strip keeps "Entrar na sessão" and a "⋮" button named "Mais ações da sessão", both 48 px high (the button 48 px wide).
The button opens a menu with "Copiar link da sessão" and "Encerrar sessão" (Escape closes it and the focus returns to the button).
The page's level-up notice, when there is one, sits above the strip.

### The tabs

The tabs are links, not ARIA tabs: a route each, `aria-current="page"` on the open one, an icon before each name.
A tab change keeps the focus on the link that was used.
The open tab is bold and ink with a 2 px `accent` rule under it; the others are `ink-muted`.
A count after a name says what waits there, as a small amber badge: "Mesa 1" is one character waiting for the master's approval.
The number also goes into the link's name in words ("Mesa, 1 esperando aprovação").
On a phone the three tabs share the row; when the names do not fit, the row scrolls sideways, the open tab is scrolled into view, and a soft fade at the edge says there is more.
The tabs have no count of other kinds, no tab that disappears and no ARIA-tab arrow keys, so those states do not apply; the loading, error and empty states belong to each panel.

| Tab | Route | Panels |
| --- | --- | --- |
| Mesa | `/campaigns/:id` | "Personagens dos jogadores" (with the amber banner of each character that waits), "Personagens reservados", "Experiência", "Sessões anteriores", "Membros", "Convites" |
| Preparação | `/campaigns/:id/prep` | "Mapas", "Encontros", "Quebra-cabeças", "NPCs", "Galeria", "Bestiário", "Documento da campanha" |
| Regras | `/campaigns/:id/setup` | "Dados", "Regras da mesa", "Conteúdo da mesa", "Magias", "Pacote da campanha" |

### A panel's head

Every panel starts with the same row: a 22 px Material Symbol in `ink-muted`, the title as an `h2` in the display font, a quiet count at the end and, after it, the panel's actions.
The icon is decoration; the title says the same in words.
A panel's create action is a text action in the title row, in `accent-text`, bold, with its icon before the words ("Novo mapa", "Gerar masmorra", "Gerar tesouro", "Novo NPC", "Montar encontro", "Novo quebra-cabeça", "Enviar imagem").
A link that reads as a sentence is underlined ("Ver todos os 9 NPCs", "Mudar em Regras da mesa", "Abrir as regras").
Counts read in words: "4 mapas", "9 NPCs", "12 de 200 imagens".
A name never ends in an ellipsis: it wraps, and the icons beside it keep their size.
Outlined buttons on this page have an ink label and the `control-line` border; the accent is for the one filled button and for text actions.

### Mesa

- **Personagens dos jogadores** has one amber banner for each character that waits: "Brisa espera a sua aprovação. Barda 1, de Bia." and the outlined "Revisar Brisa" (a link to the sheet, where the master approves or refuses).
  Each approved player is a card in a two-column grid (one column on a phone): the first letter in a circle, the name, "Mago 3, de Ana", a lock when the sheet is locked and a chevron.
  Tags that need a word (choices open, "Pode subir de nível") sit under the class line.
  The note under the cards says the sheets are locked and that the NPCs are on "Preparação".
  A card has a creatures line only when the character has creatures; "Dar uma criatura a Toren" is an item of the card's "⋮" menu ("Mais ações de Toren"), which a phone does not show.
- **Experiência** has "Dar XP" (or "Registrar marco") in the title row: an outlined button with an ink label and a star, at every width.
  Each character is one line: the name, "nível 3", a bar and "2.150 XP"; on a phone the level and the XP share the name's line and the bar goes under it.
  "Histórico de XP (5)" is a row that opens the history in place; a player's panel shows the history under its own title.
- **Sessões anteriores** lists the ended sessions, each with the date and the length ("18/10 · 3 h 10 min") and "Resumo" (named "Ver resumo da Sessão N").
- **Membros** shows each person with the first letter in a circle and the role, and "Pendente" on a player whose character waits.
- **Convites** lists the invites and has the form of a new one.

On a phone "Membros", "Convites" and "Sessões anteriores" fold to one row each, in that order, after "Experiência": the title, its count ("Membros · 4 na mesa", "Convites · 1 usado", "Sessões anteriores · 3") and a chevron.
The row is a button with `aria-expanded` and the whole row is its target; "Membros" adds the pill "1 pendente", and "Convites" the action "Gerar convite" in place of the chevron.
A row opens its panel in place and the state lives in memory only.

### Preparação

- **Mapas**: one row for each map, with the icon, the name and "Mapa atual", "24 × 16 quadrados · 3 pontos", "Revelado" or "Escondido" with an eye, and a chevron.
- **NPCs**: a square with the initials, the name, what it is, a chevron.
- Both lists show five rows and then "Ver todos os N …", which shows the rest and moves the focus to the first new row.
- **Galeria** shows the six newest images in two rows of three and "Ver todas as 12 imagens".
- **Quebra-cabeças**: each row is a chevron row that opens the editor, with the name and a second line ("Resumo · Não mostrado", "Já mostrado" or "Arquivado"); "Arquivar" and "Desarquivar" are in the row's "⋮" menu.
- **Bestiário** and **Documento da campanha** are one row each, and the whole row is the link.

### Regras

Each panel says one thing in words and has one way to change it.
**Dados** says how the table rolls and, while each player chooses, what each one chose.
**Regras da mesa** says six rules (hit points, ability scores, critical hits, XP, fog, reminders); "Abrir as regras" opens the page where all of them change.
**Conteúdo da mesa** counts what the table made ("2 classes, 1 raça, 1 antecedente e 5 magias da mesa").

## A player's page

A player's page is one page, with no tabs.
The session strip shows only while a session is open, with "Entrar na sessão".
Then come the player's characters, "Experiência" with its history, "Dados" as the player's own choice, "Mapas", the three links to the rules and "Membros".

## States

- Loading: the `h1` "Campanha" and "Carregando a campanha...".
- Not found: "Campanha não encontrada", the same for a campaign that does not exist and one the person is not in.
- Error: the message of the failure, in a danger notice.
- Each panel has its own loading, empty and error state inside its box; a panel that fails does not take the page with it.

## Accessibility

- The tab nav is a `nav` named "Seções da campanha"; the open link has `aria-current="page"`.
- Every button and link is 44 px high or more; the filled one on a phone is 48 px.
- A banner's link carries the character's name ("Revisar Brisa"), so no two controls on the page share a name.
- The strip, the tabs and the panels pass axe and the layout checks in both themes at 390, 768 and 1280 px.
