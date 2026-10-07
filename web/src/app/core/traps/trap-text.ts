import { type Timestamp, timestampDate } from '@bufbuild/protobuf/wkt';

import {
  type MapPoint,
  MapPointKind,
  TrapRevealHow,
  TrapState,
} from '../../../gen/meurpg/maps/v1/maps_pb';
import {
  Ability,
  type TrapDamage,
  type TrapEffect,
  TrapPassOutcome,
  TrapSaveApplies,
  TrapTargets,
  TrapTrigger,
} from '../../../gen/meurpg/rules/v1/rules_pb';
import { joinDots, tight } from '../format/text';
import { listNames } from '../maps/scene-clues';
import { formatClock } from '../../shared/session-time/session-time';

/**
 * What a trap on the map says on the master's screen (E9-08, MR-035), written in
 * Portuguese from the point the server sends. Pure functions: the screen reads
 * the numbers and the words; the rules (who notices, what a trap does) are the
 * server's.
 */

/** Whether a map point is a trap, a treasure or a light: the kinds the session page keeps off the
 * old markers and draws as its own marks (`app-map-pins`). */
export function isPinKind(kind: MapPointKind): boolean {
  return (
    kind === MapPointKind.TRAP || kind === MapPointKind.TREASURE || kind === MapPointKind.LIGHT
  );
}

export function trapPoints(points: readonly MapPoint[]): readonly MapPoint[] {
  return points.filter((p) => p.kind === MapPointKind.TRAP);
}

export function treasurePoints(points: readonly MapPoint[]): readonly MapPoint[] {
  return points.filter((p) => p.kind === MapPointKind.TREASURE);
}

const ABILITY: Partial<Record<Ability, string>> = {
  [Ability.STRENGTH]: 'Força',
  [Ability.DEXTERITY]: 'Destreza',
  [Ability.CONSTITUTION]: 'Constituição',
  [Ability.INTELLIGENCE]: 'Inteligência',
  [Ability.WISDOM]: 'Sabedoria',
  [Ability.CHARISMA]: 'Carisma',
};

/** "Armada", "Disparada às 21:31" (when the time is known), "Desarmada". */
export function trapStateWord(state: TrapState, firedAt: Date | null = null): string {
  switch (state) {
    case TrapState.TRIGGERED:
      return firedAt ? `Disparada às ${formatClock(firedAt)}` : 'Disparada';
    case TrapState.DISARMED:
      return 'Desarmada';
    default:
      return 'Armada';
  }
}

/** The glyph of the state chip. */
export function trapStateIcon(state: TrapState): string {
  switch (state) {
    case TrapState.TRIGGERED:
      return 'warning';
    case TrapState.DISARMED:
      return 'check_circle';
    default:
      return 'crisis_alert';
  }
}

export interface TrapVisibility {
  /** `'secret'` only the master knows it; `'some'` some characters know it; `'all'` everyone does. */
  readonly kind: 'secret' | 'some' | 'all';
  /** "Só você vê", "Visível para todos", "Só Toren e Brisa sabem". */
  readonly label: string;
}

/** Who knows the trap, as the master's card says it. A fired trap is public. */
export function trapVisibility(point: MapPoint): TrapVisibility {
  if (point.revealed || point.trap?.state === TrapState.TRIGGERED) {
    return { kind: 'all', label: 'Visível para todos' };
  }
  const names = point.trapRevealedTo.map((r) => r.characterName).filter((n) => n !== '');
  if (point.trapRevealedTo.length === 0) {
    return { kind: 'secret', label: 'Só você vê' };
  }
  return {
    kind: 'some',
    label:
      names.length > 0
        ? `Só ${listNames(names)} ${names.length === 1 ? 'sabe' : 'sabem'}`
        : 'Alguns jogadores sabem',
  };
}

/** Why a character already has the trap, in the reveal dialog. */
export function knownReason(how: TrapRevealHow): string {
  switch (how) {
    case TrapRevealHow.SEARCHED:
      return 'já achou esta armadilha';
    case TrapRevealHow.NOTICED:
      return 'já notou esta armadilha';
    default:
      return 'você já revelou';
  }
}

export function triggerWord(trigger: TrapTrigger): string {
  return trigger === TrapTrigger.MANUAL ? 'Manual' : 'Ao entrar na área';
}

/** "área de 2 × 2", "o quadrado só". */
export function areaWords(size: number): string {
  return size <= 1 ? 'um quadrado só' : `área de ${size} × ${size}`;
}

function damageText(d: TrapDamage): string {
  const adjective = /^(cortante|perfurante|contundente)$/.test(d.damageTypePt);
  return `${d.dice} de ${d.damageTypePt ? (adjective ? `dano ${d.damageTypePt}` : d.damageTypePt) : 'dano'}`;
}

function damagesText(damage: readonly TrapDamage[]): string {
  return damage.map(damageText).join(' e ');
}

/** What the trap does, in one or two sentences ("Dano: 2d6 de concussão."). Empty when it only describes. */
export function effectText(effect: TrapEffect | undefined): string {
  if (!effect) {
    return '';
  }
  const parts: string[] = [];
  if (effect.attack) {
    const a = effect.attack;
    const n = a.count > 1 ? ` (${a.count} ataques)` : '';
    parts.push(`Ataque +${a.bonus}${n}: ${a.damage ? damageText(a.damage) : 'sem dano'}`);
  }
  if (effect.damage.length > 0) {
    parts.push(`Dano: ${damagesText(effect.damage)}`);
  }
  if (effect.save) {
    const s = effect.save;
    const fail = [
      s.onFail?.damage.length ? damagesText(s.onFail.damage) : '',
      s.onFail?.condition ? `fica ${s.onFail.condition.conditionPt}` : '',
    ].filter(Boolean);
    const pass = s.onPass === TrapPassOutcome.HALF ? 'metade do dano' : 'nada';
    const who = s.appliesTo === TrapSaveApplies.HIT ? ' (de quem foi atingido)' : '';
    parts.push(
      `Resistência de ${ABILITY[s.ability] ?? 'habilidade'} CD ${s.dc}${who}: ${fail.join(' e ') || 'sem efeito'}; quem passa leva ${pass}`,
    );
  }
  if (effect.conditions.length > 0) {
    parts.push(`Deixa ${listNames(effect.conditions.map((c) => c.conditionPt))}`);
  }
  if (parts.length === 0) {
    return '';
  }
  const manual = effect.targets === TrapTargets.MANUAL ? ' Você escolhe quem é atingido.' : '';
  return tight(`${parts.join('. ')}.${manual}`);
}

/** The effect for a value cell: no period at the end, and "Queda" for the pits (the preset says what it is). */
export function effectLine(point: MapPoint): string {
  const text = effectText(point.trap?.effect).replace(/\.$/, '');
  return point.trap?.presetKey.includes('pit') ? text.replace(/^Dano:/, 'Queda:') : text;
}

/** "CD para notar 15 · achar CD 15 · gatilho manual": the line of a closed card. */
export function trapDcLine(point: MapPoint): string {
  const t = point.trap;
  if (!t) {
    return '';
  }
  return joinDots([
    t.noticeDc > 0 ? `CD para notar ${t.noticeDc}` : 'sem CD para notar',
    `achar CD ${t.findDc}`,
    `gatilho ${triggerWord(t.trigger).toLowerCase()}`,
  ]);
}

/** The first line of the point's description: where it is, in the master's words. */
export function firstLine(text: string): string {
  return text.split('\n')[0].trim();
}

/** When a firing happened, from the activity line's time. */
export function clockOf(at: Timestamp | undefined): Date | null {
  return at ? timestampDate(at) : null;
}
