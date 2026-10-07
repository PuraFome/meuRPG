# Product vision

[Português (Brasil)](../pt-BR/produto/visao.md)

MeuRPG is where a D&D 5e table prepares and plays its campaigns. The Game Master (mestre) prepares the world and runs the live session. Players join by invite, follow their sheet and act in roleplay (RP) and combat within what the rules allow.

The MVP has a single goal: **the table plays its first full session through the app**, from the invite to the XP at the end of the night.

| Role | What it does in the app |
| --- | --- |
| Game Master (mestre) | Creates campaigns, NPCs and maps. Invites the players, starts the session, controls the map and the enemies, and awards XP. |
| Player (jogador) | Joins through the invite, creates a character and follows the sheet and the map. On their turn they see the possible actions; in RP they see what they can roll. |
| System | Applies the rules during the session: HP, spell slots, turns and XP. The Game Master has the final word. |

## Principles

These principles guide product and architecture decisions.

1. **The server is the authority.** Every rule (sheet lock, HP, XP, who sees what) is checked on the server, never only on the screen.
2. **No spoilers.** Players see only what the group has already discovered. The Game Master's notes never leave the server for a player.
3. **Players play on the phone.** Player screens are designed for the small screen first.
4. **Cost near zero.** With no active session, the server idles and nothing stays connected.
5. **Requirements first.** Every story has acceptance criteria that become automated tests; a feature is done only when its tests pass. The new backend is built from scratch, story by story, with no gradual migration from the legacy app.

## See also

- [Glossary](glossary.md): what each term used here means.
- [Business rules](rules.md): how these principles become rules in the system.
- [Stories and acceptance criteria](stories.md): what to build to reach the MVP.
- [Roadmap](../roadmap.md): the order of the stages up to the MVP.
