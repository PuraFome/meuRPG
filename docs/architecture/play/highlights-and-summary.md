# Highlights and session summary

When a combat ends the table sees who did what, and when the session ends it sees how it went (MR-032).
Both are computed on every read from `session_events`, never stored, so the history and the numbers cannot disagree.

## Combat highlights

`CombatService.GetCombatHighlights(encounter_id)` (`highlights.go`) is for any member, only for an ended combat of the open session.
Otherwise it is `failed_precondition` with `EncounterBlocked` `NOT_ENDED`; a combat of another campaign is `not_found`.

- **Where it comes from.** The combat's `session_events`.
  An event that `action_undone` undid is skipped (the first pass finds all the undone), and a player character's damage only counts when the master applies it (`damage_applied`): rolled and waiting, or discarded, does not count.
- **Who reads what.** The master reads every row and every category.
  A player reads only their own character's row (zeros included, RN-20), and the categories are worked out without the characters the master gave back to the reserve, which are invisible to players in the combats they fought before too (RN-10): not in a category, not in a row, not even for their former player.
  `dropReserved` is the one filter, shared with the session summary.
- **What counts**, per player character who fought:
  - damage dealt to NPCs, only the HP that really left (temporary too; damage beyond 0 does not count);
  - healing done, what really came back, to whoever;
  - damage taken, from whoever (a blow at 0 HP is a death-save failure, not HP; a druid in Wild Shape takes it on the beast's pool, and what carries over when the form drops counts too, without counting twice: `hitPool`);
  - killing blows, NPCs their blow took to 0 HP;
  - critical hits, with a weapon and with a spell attack.
  An NPC's HP adjustment by the master's hand is nobody's.
  Whoever left the combat (`RemoveCombatant`) does not appear.
  A creature's damage does not count for the owner, and a blow on a group target (a creature or a character) is friendly fire and counts for nobody: only what hits an NPC counts.
- **The categories**, in the app's order: Mais dano causado, Mais cura, Tanque (most damage taken), Golpe final and Acertos críticos (`HighlightKind`, `HighlightCategory`).
  Each carries the number and all the tied, in combat order; a category in which everyone has 0 is left out.
- **What each receives (RN-20).** No NPC is named, hidden or not: damage to a hidden NPC enters as a number, and no response carries an NPC's name or id.
  Every member receives the categories with the winners' numbers.
  `characters`, the table with the numbers of each character (zeros included), comes whole only to the master ("Números de cada jogador"); a player receives exactly one row, their own ("Seu resultado"), and never another player's.
- **Tests.** `TestMR032_HighlightsOfTheAmbush`, `TestMR032_HighlightsSkipTheUndoneAndCountWhatHappened`, `TestMR032_OverkillDoesNotCount`, `TestMR032_UndoneActionsAreSkipped`, `TestMR032_DamageTakenIsWhatTheMasterApplied`, `TestMR032_HealingIsWhatWasGivenBack`, `TestMR032_TiesNameEveryoneAndZerosAreLeftOut`, `TestMR032_HighlightsAuthorizationMatrix`.

## Session summary

`PlayService.GetSessionSummary(campaign_id, game_session_id)` (`summary.go`) is for any member, only for a session that ended.
The open session is `failed_precondition` (`GameSessionBlocked` `SESSION_NOT_ENDED`), because its numbers still change and each combat already has its own highlights; a session of another campaign is `not_found`.
The app knows the summary exists from the `session_ended` event, which carries the session id.

- **Where it comes from.** Like the highlights, on every read.
  The checks are counted by the database (`CountSessionScenesOpened` and `TallySessionSceneChecks`: one row per character), so the number of rolls does not change what the summary costs.
  For each combat that began in the session (`started_at` filled) the server runs the same `tallyHighlights` as `GetCombatHighlights` and sums, per character, in the order they appeared.
  The categories come from the same `categoriesOf`, with two more: `HIGHLIGHT_KIND_CHECKS_PASSED` ("Mais testes passados fora do combate": the count of checks passed, a tie names everyone) and `HIGHLIGHT_KIND_TREASURE_FOUND` ("Mais tesouro encontrado").
- **What counts as a check.** Only a `scene_check_rolled` roll with `dc_shown` (the scene showed the DC when it was made) and `passed` filled (the action had a DC).
  A roll in a scene that hid the DC enters no count, for anyone (RN-20).
  Switching the scene's key later changes nothing already rolled.
- **"Mais tesouro encontrado" (MR-041).** `HIGHLIGHT_KIND_TREASURE_FOUND` exists only in the session summary, and each character's `treasure_found_po` (`SessionCharacterSummary`, in the master's table and the player's `mine`) is the gold each character found in that session: treasures with `treasure_session_id` equal to its, each value split between those who found it and rounded down (125 and 125 for 251 gp from two).
  `play` asks `maps` through `MapKeeper.TreasureFoundIn(session_id)` (the query `ListTreasureFindsOfSession`).
  It reads live data: the treasures found in that session with the value and the finders as they are now, not the `treasure_found` events, because marking again to change who found it writes no event.
  So editing a treasure later changes an old session's summary (a converted treasure is locked, and an unmarked one stops counting).
  A treasure found outside a session has no session and does not count.
  Someone who found one and neither fought nor rolled a check enters the summary for that.
  Every member receives the category (treasure found is public), in any XP mode; it is a different number from the "Voltar à cidade" XP.
  Test: `TestMR032_SummaryCountsTreasureFoundInTheSession`.
- **What each receives (RN-20).** Every member: the start, the end and the duration, and the categories with the winners and their number, without the "of N".
  The master also: how many combats and open scenes (opening again counts again), the checks passed and attempted in total ("9 de 12") and the `players` table with each character who fought or rolled a check, zeros included ("Pensantus: 3 de 4").
  The player instead: `mine`, their own result (their damage and checks passed of attempted), never another player's.
  No NPC is named.

**The screen.**
`SessionEnded` (`pages/live-session/session-ended`) reads the summary when the session page turns "ended" (the master confirmed, or the stream sent `session_ended`) and draws the master's panel or the player's card; if the read fails it falls back to "A sessão acabou".
The screen computes nothing.
The highlight boxes, the per-player table and the card are in `web/src/app/shared/highlights` and are the same as at the end of a combat.
The summary stays readable afterwards: the campaign page's "Sessões anteriores" panel and the route `/campaigns/:id/sessions/:number` (`pages/session-summary`, lazy; the number is `GameSession.session_number`) read it.
`CampaignDetail` provides one `CampaignSessions` (`ListGameSessions`, newest first) that the "Sessão" and "Sessões anteriores" panels share; the summary page makes its own call to find the id and neighbouring sessions, then calls `GetSessionSummary` and, for the master, `ListCharacters` for the players' names.
Both the page and the ending screen draw `SessionSummaryBody` (`shared/session-summary`).
The server answers `not_found` both for a number that does not exist and for a non-member, so the app separates them by what it knows: a list that answered without the number is "Não há Sessão N", and a `not_found` from the list or the campaign is one generic page with no campaign name.

**Tests.** `TestMR032_SessionSummary` (a combat and three scenes, one with the DC hidden), `TestRN20_SessionSummaryHidesWhatTheSceneHid`, and the `GetSessionSummary` and `GrantSceneAttempt` rows in the authorization matrices.

The summary also carries the conversation of the session (`conversation`, the last 500 lines, `conversation_truncated`) under the same reading rule as the live feed: a player reads only the lines meant for them (RN-36; [live session](live-session.md#the-conversation-of-the-scene)).
