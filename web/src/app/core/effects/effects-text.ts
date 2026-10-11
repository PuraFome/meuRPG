import {
  type CatalogEffect,
  type EffectSave,
  EffectDurationKind,
  EffectAudience,
  EffectPhase,
  type LastingEffect,
  type TurnClockEntry,
} from '../../../gen/meurpg/play/v1/lasting_effects_pb';
import type { DurationSpec } from './effects-client';

/** The server's limits for what the master types. */
export const MIN_ROUNDS = 1;
export const MAX_ROUNDS = 600;
export const MAX_LABEL = 30;
export const MIN_DC = 1;
export const MAX_DC = 40;
export const MAX_TARGETS = 10;
export const MAX_SECONDS = 86400;
export const SECONDS_PER_ROUND = 6;
const SECONDS_PER_MINUTE = 60;
const SECONDS_PER_HOUR = 3600;

/** "Passar o tempo": what a tap puts in the field, in seconds. */
export const TIME_PRESETS: readonly { readonly label: string; readonly seconds: number }[] = [
  { label: '1 rodada (6 s)', seconds: SECONDS_PER_ROUND },
  { label: '1 minuto', seconds: SECONDS_PER_MINUTE },
  { label: '10 minutos', seconds: 600 },
  { label: '1 hora', seconds: SECONDS_PER_HOUR },
];

/** The six levels of exhaustion (SRD 5.1, Conditions): each one adds to the ones under it. */
export const EXHAUSTION_LEVELS: readonly {
  readonly level: number;
  readonly title: string;
  readonly text: string;
}[] = [
  { level: 0, title: 'Sem exaustão', text: '' },
  { level: 1, title: 'Nível 1', text: 'Desvantagem em testes de habilidade' },
  { level: 2, title: 'Nível 2', text: 'Deslocamento pela metade (e os de baixo)' },
  {
    level: 3,
    title: 'Nível 3',
    text: 'Desvantagem em ataques e testes de resistência (e os de baixo)',
  },
  { level: 4, title: 'Nível 4', text: 'PV máximos pela metade (e os de baixo)' },
  { level: 5, title: 'Nível 5', text: 'Deslocamento 0 (e os de baixo)' },
  { level: 6, title: 'Nível 6', text: 'Morte (e os de baixo)' },
];

export const EXHAUSTION_HINT =
  'Os níveis somam. O nível 4 corta os PV máximos pela metade e ajusta os atuais ao novo máximo; baixar o nível não cura. O nível 6 é a morte.';

export const MAX_EXHAUSTION = 6;

/** The three ways the master picks how long an effect lasts when it is not a number of rounds already. */
export type DurationMode = 'rounds' | 'turn' | 'dismissed';

/** "10 rodadas, concentração": what the catalog starts a new effect with. */
export function defaultDurationText(c: CatalogEffect): string {
  const base = durationKindText(c.defaultDurationKind, c.defaultRounds);
  return c.concentration ? `${base}, concentração` : base;
}

function durationKindText(kind: EffectDurationKind, rounds: number): string {
  switch (kind) {
    case EffectDurationKind.ROUNDS:
      return rounds === 1 ? '1 rodada' : `${rounds} rodadas`;
    case EffectDurationKind.UNTIL_START_OF_TURN_OF:
      return 'Até o começo do turno de alguém';
    case EffectDurationKind.UNTIL_END_OF_TURN_OF:
      return 'Até o fim do turno de alguém';
    case EffectDurationKind.CONCENTRATION:
      return 'Enquanto se concentra';
    case EffectDurationKind.LONG_REST:
      return 'Até um descanso longo';
    default:
      return 'Até o mestre encerrar';
  }
}

/** Whether `value` is a whole number of rounds the server accepts. */
export function validRounds(value: number): boolean {
  return Number.isInteger(value) && value >= MIN_ROUNDS && value <= MAX_ROUNDS;
}

/** Whether `value` is a saving throw DC the server accepts. */
export function validDc(value: number): boolean {
  return Number.isInteger(value) && value >= MIN_DC && value <= MAX_DC;
}

/** The seconds the field may hold. */
export function validSeconds(value: number): boolean {
  return Number.isInteger(value) && value >= 1 && value <= MAX_SECONDS;
}

/** The duration the form holds, as the server's choice. `turnEnd` picks "o fim do turno" over "o começo" for a turn. */
export function durationSpec(
  mode: DurationMode,
  rounds: number,
  anchorId: string,
  turnEnd: boolean,
  roundsAnchorId = '',
): DurationSpec {
  switch (mode) {
    case 'rounds':
      return {
        kind: EffectDurationKind.ROUNDS,
        rounds,
        anchorCombatantId: roundsAnchorId || undefined,
      };
    case 'turn':
      return {
        kind: turnEnd
          ? EffectDurationKind.UNTIL_END_OF_TURN_OF
          : EffectDurationKind.UNTIL_START_OF_TURN_OF,
        anchorCombatantId: anchorId,
      };
    default:
      return { kind: EffectDurationKind.UNTIL_DISMISSED };
  }
}

/** The mode of the form that starts as the catalog's default duration. */
export function modeOfKind(kind: EffectDurationKind): DurationMode {
  switch (kind) {
    case EffectDurationKind.ROUNDS:
      return 'rounds';
    case EffectDurationKind.UNTIL_START_OF_TURN_OF:
    case EffectDurationKind.UNTIL_END_OF_TURN_OF:
      return 'turn';
    default:
      return 'dismissed';
  }
}

/** "5 efeitos · rodada 3 · vez de Nael". */
export function panelSubtitle(count: number, round: number, turnLabel: string): string {
  const parts = [count === 1 ? '1 efeito' : `${count} efeitos`];
  if (round > 0) {
    parts.push(`rodada ${round}`);
  }
  if (turnLabel) {
    parts.push(`vez de ${turnLabel}`);
  }
  return parts.join(' · ');
}

/** "Toren, Brisa, Ragna (3 alvos)", or the one name. */
export function targetsText(e: LastingEffect): string {
  const labels = e.targetLabels;
  return labels.length > 1 ? `${labels.join(', ')} (${labels.length} alvos)` : (labels[0] ?? '');
}

/** "Toren, Brisa e Ragna". */
export function namesList(names: readonly string[]): string {
  if (names.length < 2) {
    return names.join('');
  }
  return `${names.slice(0, -1).join(', ')} e ${names[names.length - 1]}`;
}

/** "de Orla" from "De Orla": the origin inside a sentence (the server writes it with a capital). */
export function lowerFirst(text: string): string {
  return text ? text.charAt(0).toLowerCase() + text.slice(1) : text;
}

/** "Sim: Paralisado", "Sim", "Sim: só o dono do alvo" or "Não": what the players see of an effect now. */
export function visibilityText(e: LastingEffect): string {
  if (!e.playerVisible) {
    return 'Não';
  }
  if (e.audience === EffectAudience.OWNER) {
    return 'Sim: só o dono do alvo';
  }
  return e.playersSeePt ? `Sim: ${e.playersSeePt}` : 'Sim';
}

/** The saving throw an effect asks, as a line for the master: "Teste de Sabedoria, CD 14, no fim do turno de Goblin 2." */
export function saveLine(e: LastingEffect): string {
  const save = e.endSave ?? e.startSave;
  if (!save) {
    return '';
  }
  return saveSentence(save, e.targetLabels);
}

function saveSentence(save: EffectSave, targets: readonly string[]): string {
  const dc = save.dc === undefined ? '' : `, CD ${save.dc}`;
  const phase = save.phase === EffectPhase.START ? 'no começo' : 'no fim';
  const whose = targets.length === 1 ? `de ${targets[0]}` : 'de cada alvo';
  return `Teste de ${save.abilityNamePt}${dc}, ${phase} do turno ${whose}.`;
}

/** What "Duração e fim" says: how it ends, in the master's words. */
export function endLine(e: LastingEffect): string {
  return e.endTextPt || e.clockTextPt;
}

/** The effect's tags and consequences, for the line under the end. */
export function tagsLine(e: LastingEffect): string {
  return e.tagsPt.join(', ');
}

/** "Em Goblin 2 · de Orla" for the phone's card. */
export function cardSubtitle(e: LastingEffect): string {
  const who = targetsLabelShort(e);
  return e.originPt ? `Em ${who} · ${lowerFirst(e.originPt)}` : `Em ${who}`;
}

function targetsLabelShort(e: LastingEffect): string {
  return e.targetLabels.join(', ');
}

/** The title of a clock entry: "Fim do turno de Goblin 2 (rodada 3)". An effect that runs out by rounds says "Turno de X". */
export function clockTitle(entry: TurnClockEntry, effects: readonly LastingEffect[]): string {
  const effect = effects.find((e) => e.id === entry.effectId);
  const byRounds =
    !entry.isSave &&
    effect !== undefined &&
    (effect.durationKind === EffectDurationKind.ROUNDS ||
      effect.durationKind === EffectDurationKind.CONCENTRATION);
  const head = byRounds
    ? 'Turno'
    : entry.phase === EffectPhase.END
      ? 'Fim do turno'
      : 'Começo do turno';
  return `${head} de ${entry.combatantLabel} (rodada ${entry.round})`;
}

/** The sentence that confirms ending a concentration: who loses it, what goes and from whom. */
export function endConcentrationText(
  caster: string,
  names: readonly string[],
  targets: readonly string[],
): string {
  const who = caster ? `A concentração de ${caster} acaba` : 'A concentração acaba';
  const from = namesList(targets);
  const goes =
    names.length === 1
      ? `${who} e ${names[0]} sai de ${from}.`
      : `${who} e saem de ${from}: ${namesList(names)}.`;
  return `${goes} Isto não se desfaz.`;
}

/** The effects of the same casting as `effect`, `effect` included. */
export function groupOf(effect: LastingEffect, all: readonly LastingEffect[]): LastingEffect[] {
  return effect.groupId ? all.filter((e) => e.groupId === effect.groupId) : [effect];
}

/** The names of what the sentence of "Encerrar" lists, once each, in order. */
export function uniqueNames(effects: readonly LastingEffect[]): string[] {
  return [...new Set(effects.map((e) => e.sourceNamePt))];
}

/** The targets of the effects, once each, in order. */
export function uniqueTargets(effects: readonly LastingEffect[]): string[] {
  return [...new Set(effects.flatMap((e) => e.targetLabels))];
}

/** "1 rodada", "2 rodadas". */
export function roundsText(n: number): string {
  return n === 1 ? '1 rodada' : `${n} rodadas`;
}

/** "Passou 1 minuto": the time that went by, from seconds, the way the presets say it. */
export function elapsedText(seconds: number): string {
  if (seconds % SECONDS_PER_HOUR === 0) {
    const h = seconds / SECONDS_PER_HOUR;
    return h === 1 ? '1 hora' : `${h} horas`;
  }
  if (seconds % SECONDS_PER_MINUTE === 0) {
    const m = seconds / SECONDS_PER_MINUTE;
    return m === 1 ? '1 minuto' : `${m} minutos`;
  }
  return seconds === 1 ? '1 segundo' : `${seconds} segundos`;
}

/** What the line under "Passar o tempo" says once the time went by. */
export function advanceDoneText(seconds: number, ended: number): string {
  const time = `Passou ${elapsedText(seconds)}.`;
  if (ended === 0) {
    return `${time} Nenhum efeito acabou.`;
  }
  return `${time} ${ended === 1 ? '1 efeito acabou' : `${ended} efeitos acabaram`}.`;
}
