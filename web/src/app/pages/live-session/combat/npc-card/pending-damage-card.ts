import {
  type Combatant,
  type Encounter,
  type PendingDamage,
  PendingDamageStatus,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import { hitPointsAfter, hitPointsLine } from '../../../../core/combat/attack-flow';
import {
  damageFormula,
  diceName,
  extraDiceOf,
  sumRange,
} from '../../../../core/combat/combat-dice';
import { article } from '../../../../core/combat/combat-log';
import { isPlayer } from '../../../../core/combat/combat-view';
import { criticalHint, criticalTypedHint, fixedParts } from '../../../../core/combat/critical';
import { heldReason } from '../../../../core/combat/reaction-master';

/**
 * Which part of a trap's hit a card is: "Armadilha: perfurante (2 de 4)". A trap that fires several times (darts) opens
 * one card per damage type per hit, all on the same target and trap point; the card carries no hit number, so the
 * position among the cards of the same target, point and type, in the order they were opened, stands for it.
 */
export function trapLabel(p: PendingDamage, all: readonly PendingDamage[]): string {
  if (!p.trapPointId) {
    return '';
  }
  const same = all.filter(
    (q) =>
      q.trapPointId === p.trapPointId &&
      q.targetId === p.targetId &&
      q.damageTypeKey === p.damageTypeKey,
  );
  const word = `${p.trapName || 'Armadilha'}: ${p.damageTypePt || 'dano'}`;
  return same.length > 1 ? `${word} (${same.indexOf(p) + 1} de ${same.length})` : word;
}

/** The target's hit points before and after `total` damage: the points of the form a druid in Wild Shape wears. */
function plainLine(target: Combatant, total: number): string {
  if (target.hitPointsMax === undefined) {
    return '';
  }
  const own = target.hitPointsCurrent ?? 0;
  return hitPointsLine(
    target.label,
    own,
    target.hitPointsMax,
    hitPointsAfter(own, target.hitPointsTemporary ?? 0, total),
  );
}

/** The target's hit points before and after the damage. A druid in Wild Shape loses the beast's points first
 * ("Lobo: 11 de 11 PV, depois 4"); what the beast cannot take passes to the druid (SRD 5.1, "Wild Shape"). */
export function effectLine(target: Combatant, total: number): string {
  const beastCurrent = target.wildShapeHitPointsCurrent;
  const beastMax = target.wildShapeHitPointsMax;
  if (target.wildShapeBeastKey === '' || beastCurrent === undefined || beastMax === undefined) {
    return plainLine(target, total);
  }
  const line = hitPointsLine(
    target.wildShapeBeastNamePt || 'Forma animal',
    beastCurrent,
    beastMax,
    hitPointsAfter(beastCurrent, 0, total),
  );
  const over = total - beastCurrent;
  if (over > 0 && target.hitPointsMax !== undefined) {
    return `${line}; o resto (${over}) passa para ${plainLine(target, over)}`;
  }
  return line;
}

/** Why "Rolar dano" is off: a window the combat waits on holds the damage (the answer is in the master's cards,
 * `ReactionQueue`), or the reaction of the target is awaited with no window to name. */
function heldText(p: PendingDamage, e: Encounter, target: Combatant | undefined): string {
  const awaiting = p.status === PendingDamageStatus.AWAITING_REACTION;
  const rolled = p.status === PendingDamageStatus.ROLLED;
  const window = awaiting || !rolled ? heldReason(e) : '';
  if (window || !awaiting) {
    return window;
  }
  return `Espere a reação ${article(target?.label ?? '') === 'a' ? 'da' : 'do'} ${target?.label ?? 'alvo'}.`;
}

/** What the roll form of a damage shows: its dice, the sum a typed roll may be and the critical's line (RN-24). */
function rollForm(p: PendingDamage) {
  const name = diceName(p.diceCount, p.diceSides);
  const range = sumRange(p.diceCount, p.diceSides);
  return {
    dice: name,
    range,
    label: p.diceCount > 1 ? `Role ${name} para o dano: some os dois` : `Role ${name} para o dano`,
    line: p.roll ? damageFormula(p.roll, p.damageTypePt, extraDiceOf(p), p.criticalMax) : '',
    // A critical hit follows the table's rule: what to roll, said the way it asks, and what the typed sum is.
    crit: p.critical
      ? (criticalHint(p.criticalRule, p.diceCount, p.diceSides, p.criticalMax)?.line ?? '')
      : '',
    typedHint: criticalTypedHint(p.criticalRule, name, range.min, range.max, p.criticalMax),
    modifier: p.bonus + p.criticalMax,
    fixedText: fixedParts(p.criticalMax, p.bonus),
  };
}

/** "Zona de magia: Nuvem de Adagas" for a damage a spell zone owes: the server marks no zone and no reason on it, so the
 * card matches the zone the caster has for that spell (the cast's own damage has a cast id, a trap's a trap point). Generic
 * words: whether the creature came in or began its turn there is in the log only. */
export function zoneLabel(p: PendingDamage, e: Encounter): string {
  if (p.castId || p.trapPointId) {
    return '';
  }
  const zone = e.zones.find(
    (z) => z.spellKey === p.attackKey && z.ownerCombatantId === p.attackerId,
  );
  return zone ? `Zona de magia: ${zone.spellNamePt}` : '';
}

/** One pending damage as its card shows it. */
export function pendingCard(p: PendingDamage, e: Encounter, all: readonly PendingDamage[]) {
  const attacker = e.combatants.find((c) => c.id === p.attackerId);
  const target = e.combatants.find((c) => c.id === p.targetId);
  const rolled = p.status === PendingDamageStatus.ROLLED;
  // What "Aplicar" applies: the damage after resistance, vulnerability and immunity when a step changed it.
  const total = p.amountAfterSteps ?? p.amount;
  return {
    p,
    total,
    /** The extras the master may take out: the ones the player could choose, while they count. */
    removable: new Set((p.parts ?? []).filter((x) => x.choosable).map((x) => x.key)),
    rolls: p.partRolls ?? [],
    steps: p.steps ?? [],
    rolled,
    awaiting: p.status === PendingDamageStatus.AWAITING_REACTION,
    held: heldText(p, e, target),
    // A player rolls the damage of their own attack; the master waits.
    waitsForPlayer: !rolled && !!attacker && isPlayer(attacker),
    trapLabel: trapLabel(p, all),
    zoneLabel: zoneLabel(p, e),
    attacker: attacker?.label ?? '',
    target: target?.label ?? '',
    ...rollForm(p),
    effect: rolled && target ? effectLine(target, total) : '',
  };
}
