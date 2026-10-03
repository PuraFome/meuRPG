/** What the master's lists say about a combatant besides the encounter: its
 * character's class line, the player's name and the kind of NPC. Read from
 * the campaign's roster (`RosterClient`), never from the combatant's label. */
export interface CombatantInfo {
  /** "Mago 3", or empty. */
  readonly classSummary: string;
  /** The player's display name, or `null`. */
  readonly playerName: string | null;
  /** "Inimigo", "Minion"… for an NPC; empty for a player. */
  readonly kindLabel: string;
  /** "Gnomo das Rochas", or empty. */
  readonly raceName: string;
}
