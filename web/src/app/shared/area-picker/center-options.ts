import type { Combatant, TargetInReach } from '../../../gen/meurpg/play/v1/combat_pb';
import type { Square } from '../../core/combat/combat-grid';
import { combatantInitial, isPlayer } from '../../core/combat/combat-view';
import { isCreature } from '../../core/combat/creature-names';
import { metersFixed } from '../../core/units';

/** An entry of "Centrar em…": a creature the caster sees, the caster itself, or the map's own arrows. */
export interface CenterOption {
  readonly id: string;
  readonly label: string;
  readonly initial: string;
  readonly npc: boolean;
  readonly creature: boolean;
  readonly detail: string;
  readonly square: Square | null;
}

/** An entry for a combatant on the map: its token's look (an NPC is the rounded square, a creature the dashed ring) and its square. */
function optionOf(c: Combatant, label: string, detail: string): CenterOption {
  return {
    id: c.id,
    label,
    initial: combatantInitial(c.label),
    npc: !isPlayer(c) && !isCreature(c),
    creature: isCreature(c),
    detail,
    square: { col: c.col, row: c.row },
  };
}

/**
 * "Centrar em…": the creatures the caster sees (in the order the server gave them), then the caster itself (for a point spell) and
 * the map's own arrows.
 */
export function centerOptions(
  combatants: readonly Combatant[],
  seen: readonly TargetInReach[],
  casterId: string,
  isPoint: boolean,
): CenterOption[] {
  const byId = new Map(combatants.map((c) => [c.id, c]));
  const out: CenterOption[] = [];
  for (const t of seen) {
    const c = byId.get(t.combatantId);
    if (!c?.placed || t.combatantId === casterId) {
      continue;
    }
    // The initial comes from the name the server gave the target, as before.
    out.push({
      ...optionOf(c, t.label, t.distanceFt !== undefined ? `a ${metersFixed(t.distanceFt)}` : ''),
      id: t.combatantId,
      initial: combatantInitial(t.label),
    });
  }
  const caster = byId.get(casterId);
  if (caster?.placed && isPoint) {
    out.push(optionOf(caster, `${caster.label} (você)`, 'aqui'));
  }
  out.push({
    id: 'grid',
    label: 'Um quadrado do mapa (use as setas)',
    initial: '',
    npc: false,
    creature: false,
    detail: '',
    square: null,
  });
  return out;
}
