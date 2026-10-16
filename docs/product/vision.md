# Product vision

[Português (Brasil)](../pt-BR/produto/visao.md)

MeuRPG is where a D&D 5e table prepares and plays its campaigns.
The Game Master (*mestre*) prepares the world and runs the live session.
Players join by invite, follow their sheet and act in roleplay (RP) and in combat within what the rules allow.

The goal of the MVP is one thing: **the table plays a full session through the app**, from the invite to the XP at the end of the night.

| Role | What it does in the app |
| --- | --- |
| Game Master (*mestre*) | Creates campaigns, NPCs and maps. Invites the players, starts the session, controls the map and the enemies, and awards XP. |
| Player (*jogador*) | Joins through the invite, creates a character and follows the sheet and the map. On their turn they see the possible actions; in RP they see what they can roll. |
| System | Applies the rules during the session: modifiers, HP, spell slots, turns and XP. The Game Master has the final word. |

## Principles

1. **The server is the authority.** Every rule (sheet lock, HP, XP, who sees what) is checked on the server, never only on the screen.
2. **No spoilers.** Players see only what the group has already discovered. What the Game Master hides never reaches a player ([RN-10](rules.md)).
3. **Players play on the phone.** Player screens are designed for the small screen first.
4. **Cost near zero.** With no active session the server idles and nothing stays connected.
5. **Requirements first.** Every story has acceptance criteria that become automated tests. A feature is done only when its tests pass.
6. **The SRD 5.1 is the rulebook.** The rules come from the System Reference Document; what a table needs from other books, it registers in its own words ([MR-025](stories.md#mr-025-register-table-content)).

## See also

- [Glossary](glossary.md): what each term means.
- [Business rules](rules.md): how the principles become rules.
- [Stories and acceptance criteria](stories.md): what the app does and how to know it is done.
- [Roadmap](../roadmap.md): what is built and what comes next.
