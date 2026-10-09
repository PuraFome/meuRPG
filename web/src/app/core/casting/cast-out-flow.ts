import {
  CastEffect,
  CastingEffectKind,
  type CastingReach,
  type CastingSpell,
  type CastingTarget,
  type OutsideCast,
  type OutsideCastTarget,
  OutsideCastEnd,
  OutsideCastStatus,
  RestThatEnds,
} from '../../../gen/meurpg/play/v1/casting_pb';
import { circleLabel } from '../combat/combat-options';
import { joinDots } from '../format/text';
import { metersText } from '../units';

/**
 * The steps of the "Conjurar" sheet outside a combat (SRD 5.1, "Spellcasting"): the spell, the way (a slot or a ritual),
 * the targets and what the cast did. Small pure functions, so the choices, the times and the sentences are tested
 * without a DOM. The server decides what is allowed (the slots free, who is in reach, who may cast a ritual); this
 * only words it.
 */

const SECONDS_PER_ROUND = 6;
const SECONDS_PER_MINUTE = 60;
const MINUTES_PER_HOUR = 60;
const SECONDS_PER_HOUR = 3600;
const SECONDS_PER_DAY = 86400;
/** The most targets one cast takes, the master's too. */
export const MAX_OUTSIDE_TARGETS = 10;

function plural(n: number, one: string, many: string): string {
  return `${n} ${n === 1 ? one : many}`;
}

/** A game duration in words: "8 horas", "1 minuto", "1 rodada". Never a clock time: durations are game time. */
export function durationWords(seconds: number): string {
  if (seconds >= SECONDS_PER_DAY && seconds % SECONDS_PER_DAY === 0) {
    return plural(seconds / SECONDS_PER_DAY, 'dia', 'dias');
  }
  if (seconds >= SECONDS_PER_HOUR && seconds % SECONDS_PER_HOUR === 0) {
    return plural(seconds / SECONDS_PER_HOUR, 'hora', 'horas');
  }
  if (seconds >= SECONDS_PER_MINUTE && seconds % SECONDS_PER_MINUTE === 0) {
    return plural(seconds / SECONDS_PER_MINUTE, 'minuto', 'minutos');
  }
  return plural(Math.round(seconds / SECONDS_PER_ROUND), 'rodada', 'rodadas');
}

/** "11 minutos", "1 hora e 10 minutos": how long a casting takes. */
export function minutesWords(minutes: number): string {
  const hours = Math.floor(minutes / MINUTES_PER_HOUR);
  const rest = minutes % MINUTES_PER_HOUR;
  if (hours > 0 && rest > 0) {
    return `${plural(hours, 'hora', 'horas')} e ${plural(rest, 'minuto', 'minutos')}`;
  }
  return hours > 0 ? plural(hours, 'hora', 'horas') : plural(rest, 'minuto', 'minutos');
}

/** What a spell lasts, as its line says it: "concentração, até 1 minuto", "dura 8 horas"; "" for an instantaneous one. */
export function lastsWords(spell: CastingSpell): string {
  if (!spell.lasts) {
    return spell.spell?.concentration ? 'concentração' : '';
  }
  const time = spell.durationSeconds > 0 ? durationWords(spell.durationSeconds) : '';
  if (spell.spell?.concentration) {
    return time ? `concentração, até ${time}` : 'concentração';
  }
  return time ? `dura ${time}` : 'até ser dissipada';
}

/** How far a spell reaches, for the line of a spell that lasts nothing: "toque", "pessoal", "9 m". */
export function reachWords(spell: CastingSpell): string {
  switch (spell.rangeKind) {
    case 'self':
      return 'pessoal';
    case 'touch':
      return 'toque';
    case 'ranged':
      return spell.rangeFt > 0 ? metersText(spell.rangeFt) : '';
    default:
      return '';
  }
}

/** The spell's line in the list: "1º nível · 1 ação · concentração, até 1 minuto · ritual". */
export function spellLine(spell: CastingSpell): string {
  const level = spell.spell?.level ?? 0;
  return joinDots(
    [
      level === 0 ? 'Truque' : circleLabel(level),
      spell.castingTimePt,
      lastsWords(spell) || reachWords(spell),
      spell.spell?.ritual ? 'ritual' : '',
    ].filter((p) => p !== ''),
  );
}

/** Which way a spell is cast: with a slot (or a cantrip) or as a ritual. */
export type CastWay = 'slot' | 'ritual';

/** The ways a spell can be cast now: the normal one when it has a slot (or is a cantrip), the ritual when the class casts it. */
export function waysOf(spell: CastingSpell): CastWay[] {
  const out: CastWay[] = [];
  if (spell.canCast) {
    out.push('slot');
  }
  if (spell.ritualAllowed) {
    out.push('ritual');
  }
  return out;
}

/** The way a spell opens with: the normal one, the ritual when that is all there is. */
export function defaultWay(spell: CastingSpell): CastWay {
  return spell.canCast ? 'slot' : 'ritual';
}

/** Whether a spell's casting takes minutes or hours (a ritual always does): it is cast in two steps. */
export function isLong(spell: CastingSpell, way: CastWay): boolean {
  return (way === 'ritual' ? spell.ritualMinutes : spell.castingMinutes) > 0;
}

/** The minutes the casting takes the chosen way. */
export function minutesOf(spell: CastingSpell, way: CastWay): number {
  return way === 'ritual' ? spell.ritualMinutes : spell.castingMinutes;
}

/**
 * How a ritual's time is worked out, written from the spell: its own casting time plus 10 minutes (SRD 5.1,
 * "Rituals"): "1 ação + 10 minutos", "1 minuto + 10 = 11 minutos".
 */
export function ritualTimeText(spell: CastingSpell): string {
  const own = spell.castingTimePt || 'a ação';
  if (spell.castingMinutes === 0) {
    return `${own} + 10 minutos`;
  }
  return `${own} + 10 = ${minutesWords(spell.ritualMinutes)}`;
}

/** The rest that ends a spell, for "o descanso longo termina": "" when none does. */
export function restWords(rest: RestThatEnds): string {
  switch (rest) {
    case RestThatEnds.SHORT:
      return 'um descanso curto a termina';
    case RestThatEnds.LONG:
      return 'o descanso longo a termina';
    default:
      return '';
  }
}

// ---- the targets ----

/** A target of the sheet: who, how far, and whether the spell reaches it. */
export interface TargetRow {
  readonly id: string;
  readonly name: string;
  readonly npc: boolean;
  /** "a 1,5 m", "NPC que o mestre mostrou · a 3 m"; without a map, the NPC line only. */
  readonly detail: string;
  readonly enabled: boolean;
  /** Why it cannot be picked ("Fora do alcance do toque (1,5 m)."); empty when it can. */
  readonly reason: string;
  readonly self: boolean;
}

/** What the spell asks of the targets: nobody (it stays on the caster, or the master narrates), one, or up to `max`. */
export interface TargetRuleOut {
  readonly kind: 'none' | 'one' | 'many';
  readonly max: number;
}

/**
 * How many targets the spell takes with the slot chosen: `maxTargets` at its own level and `targetsPerLevel` more for
 * each level above it (SRD 5.1, each spell's text). A spell that reaches only the caster, or one the server only
 * records (nothing to apply), asks for nobody. The master is not held to the number, only to the 10 a cast takes.
 */
export function targetRuleOf(
  spell: CastingSpell,
  slotLevel: number,
  master: boolean,
): TargetRuleOut {
  if (spell.casterOnly) {
    return { kind: 'none', max: 0 };
  }
  const own = spell.spell?.level ?? 0;
  const base = spell.maxTargets + spell.targetsPerLevel * Math.max(slotLevel - own, 0);
  const max = base === 0 ? MAX_OUTSIDE_TARGETS : Math.min(base, MAX_OUTSIDE_TARGETS);
  if (spell.effect === CastingEffectKind.NARRATED && spell.maxTargets === 0) {
    return { kind: 'many', max: MAX_OUTSIDE_TARGETS };
  }
  return master && base > 0
    ? { kind: base === 1 ? 'one' : 'many', max: MAX_OUTSIDE_TARGETS }
    : { kind: max === 1 ? 'one' : 'many', max };
}

/** "Quem você toca (uma criatura, até 1,5 m)": the heading of the target step. */
export function targetHeading(spell: CastingSpell, rule: TargetRuleOut): string {
  const who = rule.kind === 'one' ? 'uma criatura' : `até ${rule.max} criaturas`;
  let far = '';
  if (spell.rangeKind === 'touch') {
    far = ', até 1,5 m';
  } else if (spell.rangeKind === 'ranged' && spell.rangeFt > 0) {
    far = `, até ${metersText(spell.rangeFt)}`;
  }
  return `${spell.rangeKind === 'touch' ? 'Quem você toca' : 'Quem recebe'} (${who}${far})`;
}

/** The rows of the target step: each person the caller may pick, with the reach the server worked out for this spell. */
export function targetRows(
  spell: CastingSpell,
  targets: readonly CastingTarget[],
  casterId: string,
): TargetRow[] {
  const reach = new Map<string, CastingReach>(spell.reach.map((r) => [r.characterId, r]));
  return targets.map((t) => {
    const r = reach.get(t.characterId);
    const self = t.characterId === casterId;
    const distance = t.distanceKnown && !self ? `a ${metersText(t.distanceFt)}` : '';
    const detail = joinDots(
      [t.npc ? 'NPC que o mestre mostrou' : '', distance].filter((x) => x !== ''),
    );
    return {
      id: t.characterId,
      name: t.name,
      npc: t.npc,
      detail,
      enabled: r?.inRange ?? true,
      reason: r && !r.inRange ? r.reasonPt : '',
      self,
    };
  });
}

/** Whether the cast needs a target before it can go: a spell the server applies to someone does. */
export function needsTarget(spell: CastingSpell): boolean {
  return !spell.casterOnly && spell.effect !== CastingEffectKind.NARRATED;
}

/** The toggle of a target in the chosen list, held to the rule (a radio replaces; a box list stops at the maximum). */
export function toggledTarget(
  rule: TargetRuleOut,
  chosen: readonly string[],
  id: string,
): string[] {
  if (chosen.includes(id)) {
    return chosen.filter((c) => c !== id);
  }
  if (rule.kind === 'one') {
    return [id];
  }
  return chosen.length >= rule.max ? [...chosen] : [...chosen, id];
}

/** "Conjurar Ajuda em 2", "Curar Ferimentos em Toren": the button of the target step. A healing spell is named by what it does. */
export function castButton(
  name: string,
  way: CastWay,
  long: boolean,
  chosen: readonly TargetRow[],
  heals = false,
): string {
  if (way === 'ritual') {
    return 'Começar o ritual';
  }
  if (long) {
    return 'Começar a conjuração';
  }
  const verb = heals ? name : `Conjurar ${name}`;
  if (chosen.length === 0) {
    return `Conjurar ${name}`;
  }
  const who = chosen.length === 1 ? chosen[0].name : String(chosen.length);
  return `${verb} em ${who}`;
}

// ---- what a cast did ----

/** Who the cast reached, by name: "Toren", "Toren e Brisa", "Toren, Brisa e Kai". */
export function namesList(names: readonly string[]): string {
  if (names.length <= 1) {
    return names[0] ?? '';
  }
  return `${names.slice(0, -1).join(', ')} e ${names[names.length - 1]}`;
}

/** The roll of a healing or temporary-hit-points cast, as the sheet writes it: "1d8 (6) + 3 (modificador) + 3 (Discípulo da Vida: 2 + nível da magia) = 12". */
export function rollLine(cast: OutsideCast, extra: number): string {
  if (cast.diceCount === 0 && cast.rollTotal === 0) {
    return '';
  }
  const sum = cast.faces.reduce((a, b) => a + b, 0);
  const modifier = cast.rollTotal - sum;
  const parts = [`${cast.diceCount}d${cast.diceSides} (${cast.faces.join(', ')})`];
  if (modifier !== 0) {
    parts.push(`${modifier > 0 ? '+' : '−'} ${Math.abs(modifier)} (modificador)`);
  }
  if (extra > 0) {
    parts.push(`+ ${extra} (Discípulo da Vida: 2 + nível da magia)`);
  }
  return `${parts.join(' ')} = ${cast.rollTotal + extra}`;
}

/** A heading for the result: "Toren foi curado", "Ajuda conjurada", "Armadura Arcana em Pensantus". */
export function resultTitle(cast: OutsideCast): string {
  const names = namesList(cast.targets.map((t) => t.name));
  const effect = cast.targets[0]?.effect ?? CastEffect.NARRATED;
  switch (effect) {
    case CastEffect.HEAL:
      return cast.targets.length === 1 ? `${names} foi curado` : `${names} foram curados`;
    case CastEffect.ARMOR_CLASS:
      return `${cast.spellNamePt} em ${names}`;
    default:
      return `${cast.spellNamePt} conjurada`;
  }
}

/** What the cast did to its targets, one line each, as the viewer is allowed to read it. */
export function resultSentences(cast: OutsideCast): string[] {
  const out: string[] = [];
  for (const t of cast.targets) {
    out.push(targetSentence(t));
  }
  if (cast.targets.length === 0) {
    out.push(RECORDED);
  }
  return out;
}

/** What the screen says of a cast the server only records: it is not applied, the master narrates it. */
export const RECORDED = 'Conjuração registrada. O efeito é narrado pelo mestre.';

function targetSentence(t: OutsideCastTarget): string {
  switch (t.effect) {
    case CastEffect.HEAL:
      return t.amount > 0 ? `${t.name} recuperou ${t.amount} PV.` : `${t.name} foi curado.`;
    case CastEffect.TEMPORARY_HIT_POINTS:
      return t.amount > 0
        ? `${t.name} ganhou ${t.amount} PV temporários.`
        : `${t.name} ficou com os PV temporários que já tinha.`;
    case CastEffect.MAX_HIT_POINTS:
      return `${t.name}: o máximo de PV e os PV atuais sobem ${t.amount}.`;
    case CastEffect.ARMOR_CLASS:
      return t.armorClass > 0
        ? `${t.name} fica com CA 13 + Destreza (${t.armorClass}).`
        : `${t.name} fica com CA 13 + Destreza.`;
    default:
      return `${t.name}: conjuração registrada.`;
  }
}

// ---- the casts that go on ----

/** Whether the cast is still being cast (its time has not passed). */
export function isCasting(cast: OutsideCast): boolean {
  return cast.status === OutsideCastStatus.CASTING;
}

/** Whether the cast took effect and lasts. */
export function isActive(cast: OutsideCast): boolean {
  return cast.status === OutsideCastStatus.ACTIVE;
}

/** Whether the cast failed (the casting did not finish; no slot was spent). */
export function isFailed(cast: OutsideCast): boolean {
  return cast.status === OutsideCastStatus.FAILED;
}

/** The chip of a spell that lasts: "dura 8 horas", "concentração · até 1 minuto". */
export function activeChip(cast: OutsideCast): string {
  const time = cast.durationSeconds > 0 ? durationWords(cast.durationSeconds) : '';
  if (cast.concentrating) {
    return time ? `concentração · até ${time}` : 'concentração';
  }
  return time ? `dura ${time}` : 'até ser dissipada';
}

/**
 * What ends a spell that lasts, the note under "Magias ativas": game time, never the wall clock. The master's long rest is
 * 8 hours of game time, so the effects of 8 hours or less end there (SRD 5.1, "Resting").
 */
export const ACTIVE_NOTE =
  'As magias de até 8 horas terminam no descanso longo do mestre (8 horas de jogo); a concentração termina quando um dano a quebra ou quando o mestre a encerra. Nada aqui usa o relógio da parede.';

/** "Falhou: a concentração se perdeu" and the other reasons a cast is over, for the log. */
export function endWords(cast: OutsideCast): string {
  switch (cast.endReason) {
    case OutsideCastEnd.INTERRUPTED:
      return 'a conjuração foi interrompida; o espaço não foi gasto.';
    case OutsideCastEnd.CONCENTRATION:
      return 'terminou: a concentração acabou.';
    case OutsideCastEnd.DISMISSED:
      return 'foi encerrada.';
    case OutsideCastEnd.REST:
      return 'terminou no descanso.';
    case OutsideCastEnd.CASTER_GONE:
      return 'terminou: quem conjurou saiu da campanha.';
    default:
      return '';
  }
}

/** "Isso encerra Bênção": which concentration a new cast ends, `null` when it ends none. */
export function endsConcentrationOf(
  spell: CastingSpell,
  way: CastWay,
  current: OutsideCast | undefined,
): string | null {
  const concentrates = (spell.spell?.concentration ?? false) || isLong(spell, way);
  return concentrates && current ? current.spellNamePt : null;
}

// ---- the log ----

/** One line of the session's casts, as the viewer may read it: "Ilaria conjurou Curar Ferimentos em Toren e Brisa". */
export function logLine(cast: OutsideCast): string {
  const who = cast.casterName;
  const names = namesList(cast.targets.map((t) => t.name));
  const on = names ? ` em ${names}` : '';
  const ritual = cast.ritual ? ' como ritual' : '';
  if (isCasting(cast)) {
    return `${who} está conjurando ${cast.spellNamePt}${ritual}${on}.`;
  }
  if (isFailed(cast)) {
    return `${who}: a conjuração de ${cast.spellNamePt} falhou; ${endWords(cast) || 'o espaço não foi gasto.'}`;
  }
  const end =
    cast.status === OutsideCastStatus.ENDED && cast.lasts
      ? ` (${endWords(cast)})`.replace(' ()', '')
      : '';
  return `${who} conjurou ${cast.spellNamePt}${ritual}${on}${end}.`;
}

// ---- the master's queue ----

/** The queue card's title: the spell, and "(ritual)" when it is cast as one ("Alarme (ritual)"). */
export function queueTitle(cast: OutsideCast): string {
  return cast.ritual ? `${cast.spellNamePt} (ritual)` : cast.spellNamePt;
}

/**
 * The queue card's line under the title: who casts, the slot (none for a ritual or a cantrip) and how long the casting
 * takes ("Pensantus · 11 minutos", "Pensantus · espaço de 3º nível · 1 hora"). The app keeps no clock, so it never says how
 * much of that time has passed: the master decides when it has.
 */
export function queueLine(cast: OutsideCast): string {
  const slot =
    cast.slotLevel > 0
      ? cast.slotPact
        ? `espaço de pacto de ${circleLabel(cast.slotLevel)}`
        : `espaço de ${circleLabel(cast.slotLevel)}`
      : '';
  return joinDots(
    [
      cast.casterName,
      slot,
      cast.castingMinutes > 0 ? minutesWords(cast.castingMinutes) : '',
    ].filter((p) => p !== ''),
  );
}

/** The note under the queue: outside a combat there is no clock, so the master is the one who says the time has passed. */
export const QUEUE_NOTE_BEFORE = 'Fora do combate não há relógio: a conjuração termina quando ';
export const QUEUE_NOTE_STRONG = 'você confirma';
export const QUEUE_NOTE_AFTER = ' que o tempo passou.';
