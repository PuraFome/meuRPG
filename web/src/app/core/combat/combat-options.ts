import {
  type ActionOption,
  type Attack,
  AttackKind,
  type DisabledReason,
  DisabledReasonCode,
  Recharge,
  type SpellOption,
  type TurnOptions,
} from '../../../gen/meurpg/rules/v1/rules_pb';
import { ActionEconomy } from '../../../gen/meurpg/rules/v1/rules_pb';
import { feetToMeters, formatMeters, joinDots, tight } from './combat-grid';

/**
 * What "Sua vez" says about the options the server works out
 * (`GetTurnOptions`): the Portuguese for every disabled reason, the line
 * under an attack, the groups by economy. Pure functions.
 */

/** The ordinal circle: "2º círculo". */
export function circleLabel(level: number): string {
  return level === 0 ? 'Truque' : `${level}º círculo`;
}

function rechargeWords(recharge: Recharge): string {
  switch (recharge) {
    case Recharge.SHORT_REST:
      return 'volta no descanso curto';
    case Recharge.LONG_REST:
      return 'volta no descanso longo';
    case Recharge.DAWN:
      return 'volta ao amanhecer';
    default:
      return 'só o mestre devolve';
  }
}

/** Why an option is disabled, in words, by code. `''` when there is none. */
export function reasonText(reason: DisabledReason | undefined): string {
  if (!reason) {
    return '';
  }
  switch (reason.code) {
    case DisabledReasonCode.ACTION_USED:
      return 'Ação já usada';
    case DisabledReasonCode.BONUS_ACTION_USED:
      return 'Ação bônus já usada';
    case DisabledReasonCode.REACTION_USED:
      return 'Reação já usada';
    case DisabledReasonCode.ATTACKS_USED:
      // Extra Attack: the attacks of this Attack action are all made.
      return 'Ataques desta ação já usados';
    case DisabledReasonCode.NO_SLOT:
      return reason.minLevel > 0
        ? `Sem espaço de ${reason.minLevel}º círculo ou maior`
        : 'Sem espaço de magia livre';
    case DisabledReasonCode.NO_USES:
      return `Sem usos: ${rechargeWords(reason.recharge)}`;
    case DisabledReasonCode.REACTION_ONLY_WHEN_HIT:
      return 'Só quando você for atingido';
    case DisabledReasonCode.REACTION_ONLY:
      return 'Só quando o gatilho acontecer';
    case DisabledReasonCode.CASTING_TIME_TOO_LONG:
      return 'Demora demais para um combate';
    case DisabledReasonCode.NOT_YOUR_TURN:
      return 'Não é a sua vez';
    case DisabledReasonCode.COMBAT_NOT_ACTIVE:
      return 'O combate não está em andamento';
    case DisabledReasonCode.COMBATANT_DOWN:
      return 'Caído: não pode agir';
    case DisabledReasonCode.COMBATANT_DEFEATED:
      return 'Derrotado: fora do combate';
    default:
      return 'Indisponível agora';
  }
}

/** A reaction row has no button by design: "Escudo" waits to be hit. Its
 * reason is a hint, not a block, so the row doesn't read as disabled. */
export function isReactionHint(reason: DisabledReason | undefined): boolean {
  return (
    reason?.code === DisabledReasonCode.REACTION_ONLY_WHEN_HIT ||
    reason?.code === DisabledReasonCode.REACTION_ONLY
  );
}

/** "1d4 + 2 perfurante" for a weapon, "1d10 de fogo" for a cantrip: the dice
 * as the sheet writes them, spaced, and the damage type. */
export function damageText(attack: Attack): string {
  const dice = attack.damage.replace(/\s*([+-])\s*/g, (_, sign: string) => ` ${sign === '-' ? '−' : '+'} `);
  const type = attack.damageTypePt;
  const adjective = /^(cortante|perfurante|contundente)$/.test(type);
  if (!type) {
    return dice;
  }
  // A cantrip's dice grow with the level and have no modifier: "de fogo".
  return /[+−]/.test(dice) || adjective ? `${dice} ${type}` : `${dice} de ${type}`;
}

/** "corpo a corpo" for a close attack, "alcance 36 m" for the rest. With
 * `reach`, a close attack says its reach too ("corpo a corpo, 1,5 m"). */
export function rangeText(attack: Attack, reach = false): string {
  const near = attack.rangeFt <= 5 && attack.longRangeFt === 0;
  if (near) {
    return reach ? `corpo a corpo, ${formatMeters(feetToMeters(attack.rangeFt || 5))}` : 'corpo a corpo';
  }
  return `alcance ${formatMeters(feetToMeters(attack.rangeFt))}`;
}

/** "+6 para acertar · 1d10 de fogo · alcance 36 m". */
export function attackDetail(attack: Attack, reach = false): string {
  const bonus = `${attack.attackBonus < 0 ? '−' : '+'}${Math.abs(attack.attackBonus)} para acertar`;
  return tight(joinDots([bonus, damageText(attack), rangeText(attack, reach)]));
}

/** "Cimitarra +5": the master's radio rows. */
export function attackTitle(attack: Attack): string {
  return `${attack.namePt || attack.name} ${attack.attackBonus < 0 ? '−' : '+'}${Math.abs(attack.attackBonus)}`;
}

/** An attack's name for a button and a sentence. */
export function attackName(attack: Attack): string {
  return attack.namePt || attack.name;
}

export function isCantrip(attack: Attack): boolean {
  return attack.kind === AttackKind.SPELL;
}

/** The line under a spell: school, concentration, and the warning when this
 * casting would use the last free slot of the spell. */
export function spellDetail(o: SpellOption): string {
  const parts = [o.spell?.schoolNamePt ?? ''];
  if (o.spell?.concentration) {
    parts.push('Concentração');
  }
  if (o.spell?.ritual) {
    parts.push('Ritual');
  }
  const level = o.spell?.level ?? 0;
  const free = o.slots.reduce((sum, s) => sum + s.free, 0);
  if (level > 0 && o.enabled && free === 1) {
    parts.push(`gasta o último espaço de ${o.slots[0].level}º círculo`);
  }
  return parts.filter(Boolean).join(' · ');
}

/** The economy a group of the screen stands for. */
export type EconomyGroup = 'action' | 'bonus' | 'reaction';

export function groupOf(economy: ActionEconomy): EconomyGroup | null {
  switch (economy) {
    case ActionEconomy.ACTION:
      return 'action';
    case ActionEconomy.BONUS_ACTION:
      return 'bonus';
    case ActionEconomy.REACTION:
      return 'reaction';
    default:
      return null;
  }
}

/** The options of one economy, split the way the screen draws them. */
export interface GroupOptions {
  readonly spells: readonly SpellOption[];
  readonly features: readonly ActionOption[];
}

export function optionsFor(options: TurnOptions, group: EconomyGroup): GroupOptions {
  return {
    spells: options.spells.filter((s) => groupOf(s.economy) === group),
    features: options.featureActions.filter((a) => groupOf(a.action?.economy ?? 0) === group),
  };
}

/** The word on a group's pill. */
export function groupState(used: boolean): string {
  return used ? 'Usada' : 'Disponível';
}

/** "Encerrar turno" is outlined while the action or the bonus action is
 * still available, and the filled button once both are used (timeline.md,
 * shared decision 2). */
export function endTurnIsPrimary(own: { actionUsed: boolean; bonusActionUsed: boolean }): boolean {
  return own.actionUsed && own.bonusActionUsed;
}
