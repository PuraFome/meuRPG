import type { Encounter } from '../../../../../gen/meurpg/play/v1/combat_pb';

/** Remembers each combatant's shield between two states of the combat, to name the ones that ended. */
export class ShieldWatch {
  private seen = new Map<string, number>();
  private encounterId = '';

  /** The labels whose shield went from above 0 to 0 on a combatant that is still there (not one that left the combat, or another combat). */
  update(e: Encounter): string[] {
    const ended: string[] = [];
    const seen = new Map<string, number>();
    for (const c of e.combatants) {
      const before = this.seen.get(c.id) ?? 0;
      if (e.id === this.encounterId && before > 0 && c.armorClassBonus === 0) {
        ended.push(c.label);
      }
      seen.set(c.id, c.armorClassBonus);
    }
    this.seen = seen;
    this.encounterId = e.id;
    return ended;
  }
}
