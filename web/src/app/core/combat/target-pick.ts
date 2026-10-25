import type { TargetInReach } from '../../../gen/meurpg/play/v1/combat_pb';
import type { Attack } from '../../../gen/meurpg/rules/v1/rules_pb';
import { tight } from '../format/text';
import { listing } from './cover';
import { damageText } from './combat-options';
import type { TargetRule } from './cast-flow';

/**
 * Choosing the target on the map (C7-04, C7-05): the pure part. The rows are the ones the server already offers for the
 * action (`TargetInReach`: the reach, the cover, the mode), said for a token on the map instead of a line of a list; the
 * rules of the taps are the cast sheet's own (`TargetRule`): one target replaces, several toggle, darts add one each.
 */

/** A creature the map offers as a target: why it cannot be one (`''` when it can), and a word about a valid one. */
export interface PickRow {
  readonly id: string;
  readonly label: string;
  /** Why the creature cannot be chosen, short ("cobertura total", "fora de alcance"); `''` when it can. */
  readonly reason: string;
  /** A valid target with a catch: "Longe: desvantagem". */
  readonly note: string;
}

/** The reason of a target the acted-for character does not see; the map says it with the character's name. */
export const NOT_SEEN = 'não vê este alvo';

/** The server's word on a target that cannot be chosen, as the map says it. */
export function reasonOf(t: TargetInReach): string {
  // The master acting for a character: a creature the character does not see is listed and dimmed (C8-01).
  if (t.notSeen) {
    return NOT_SEEN;
  }
  const cover = listing(t);
  if (cover.kind === 'blocked') {
    return cover.text;
  }
  if (cover.kind === 'left-out') {
    return 'cobertura total';
  }
  return t.tooFar ? 'fora de alcance' : '';
}

/** A weapon thrown or fired past its normal range, within the long one, still hits: with disadvantage (the server's reach says
 * `too_far` only past the long range). */
export function farNote(t: TargetInReach, attack: Pick<Attack, 'rangeFt' | 'longRangeFt'>): string {
  const far =
    attack.longRangeFt > attack.rangeFt &&
    t.distanceFt !== undefined &&
    t.distanceFt > attack.rangeFt;
  return far ? 'Longe: desvantagem' : '';
}

/** The rows of an attack's targets, in the order the server sent them. */
export function attackPickRows(
  targets: readonly TargetInReach[],
  attack: Pick<Attack, 'rangeFt' | 'longRangeFt'>,
): PickRow[] {
  return targets.map((t) => {
    const reason = reasonOf(t);
    return { id: t.combatantId, label: t.label, reason, note: reason ? '' : farNote(t, attack) };
  });
}

/** The rows of a spell's targets; the caster is a row only when `self` (a heal, a blessing). */
export function castPickRows(
  targets: readonly TargetInReach[],
  casterId: string,
  self: boolean,
): PickRow[] {
  return targets
    .filter((t) => self || t.combatantId !== casterId)
    .map((t) => ({
      id: t.combatantId,
      label: t.combatantId === casterId ? `${t.label} (você)` : t.label,
      reason: reasonOf(t),
      note: '',
    }));
}

/** The picks after a tap on a valid creature: one target replaces; several toggle up to the limit; darts add one each. */
export function tapped(rule: TargetRule, picks: readonly string[], id: string): string[] {
  switch (rule.kind) {
    case 'single':
      return [id];
    case 'darts':
      return picks.length < rule.max ? [...picks, id] : [...picks];
    default:
      if (picks.includes(id)) {
        return picks.filter((p) => p !== id);
      }
      return picks.length < rule.max ? [...picks, id] : [...picks];
  }
}

/** How many darts (or the one mark) each creature has. */
export function counts(picks: readonly string[]): Map<string, number> {
  const n = new Map<string, number>();
  for (const id of picks) {
    n.set(id, (n.get(id) ?? 0) + 1);
  }
  return n;
}

/** Whether the picks are enough to confirm: every dart placed, or at least the rule's minimum of targets. */
export function enough(rule: TargetRule, picks: readonly string[]): boolean {
  return rule.kind === 'darts' ? picks.length === rule.max : picks.length >= Math.max(rule.min, 1);
}

/** The words of the bar: its title and the line under it. */
export interface PickWords {
  readonly title: string;
  readonly hint: string;
}

/** What the bar says for the action `name`: the ask, the darts placed, or the target named before the roll. `detail` is
 * the attack's numbers ("+5 para acertar · 1d10 de fogo"); `foe` says "inimigo" (an attack) rather than "criatura". */
export function pickWords(i: {
  readonly name: string;
  readonly rule: TargetRule;
  readonly picks: readonly string[];
  readonly labels: ReadonlyMap<string, string>;
  readonly detail: string;
  readonly foe: boolean;
}): PickWords {
  const { name, rule, picks } = i;
  if (rule.kind === 'darts') {
    const unit = rule.rays ? ['raio', 'raios'] : ['dardo', 'dardos'];
    return {
      title: `${name} · ${picks.length} de ${rule.max} ${rule.max === 1 ? unit[0] : unit[1]}`,
      hint: `Cada toque põe um ${unit[0]}; um alvo pode levar vários.`,
    };
  }
  if (rule.kind === 'multi') {
    return {
      title: `${name} · ${picks.length} de até ${rule.max} alvos`,
      hint: 'Toque em quem a magia atinge; toque de novo para tirar.',
    };
  }
  const first = picks[0];
  if (first === undefined) {
    return {
      title: `Escolha o alvo · ${name}`,
      hint: `Toque ${i.foe ? 'num inimigo' : 'numa criatura'} com anel.`,
    };
  }
  return {
    title: `${name} em ${i.labels.get(first) ?? ''}`,
    hint: tight([i.detail, 'toque em outro anel para trocar'].filter(Boolean).join(' · ')),
  };
}

/** The numbers of an attack before the roll: "+5 para acertar · 1d10 de fogo" (a save cantrip: "CD 13 · 1d10 de fogo"). */
export function attackNumbers(attack: Attack): string {
  const bonus =
    attack.saveDc > 0
      ? `CD ${attack.saveDc}`
      : `${attack.attackBonus < 0 ? '−' : '+'}${Math.abs(attack.attackBonus)} para acertar`;
  return tight([bonus, damageText(attack)].filter(Boolean).join(' · '));
}
