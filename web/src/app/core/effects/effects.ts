import {
  type CharacterEffect,
  type EffectSave,
  type EffectSavePrompt,
  type ExtraDie,
  type LastingEffect,
  EffectModifierKind,
  EffectPhase,
  EffectRollKind,
} from '../../../gen/meurpg/play/v1/lasting_effects_pb';
import { dcLabel } from '../format/save-dc';
import { upFirst } from '../format/text';
import { abilityName } from './ability-choice';
import { zoneSheetWords } from './zone-save';

/**
 * The effects that last (RN-22) as the player reads them: the cards of "Seus efeitos", the exhaustion card, the
 * saving throw an effect asks at the end of the turn and the dice an effect adds to a roll. The server decides what
 * a person may read and writes the words (`source_name_pt`, `origin_pt`, `clock_text_pt`, `tags_pt`, `changes_pt`,
 * `text_pt`); these functions only arrange what was sent. Pure, tested without a DOM.
 */

/** One card of "Seus efeitos": the name, who it comes from, the small labels, the clock and what it changes. */
export interface EffectCardView {
  readonly key: string;
  readonly name: string;
  /** "De Tavo", "Da ação Esquivar", "De alguém que você não vê"; empty outside a combat. */
  readonly origin: string;
  readonly tags: readonly string[];
  /** "Restam 8 rodadas: acaba no turno de Tavo, na rodada 11." or "Dura 1 minuto". */
  readonly clock: string;
  /** "No fim de cada turno seu: teste de resistência de Sabedoria." */
  readonly save: string;
  /** The lines of "O que isso muda". */
  readonly changes: readonly string[];
}

/** The ability of a save, with "(CD 14)" only when the server sent the DC (the table's rule; RN-10, RN-20). */
function saveWords(save: EffectSave): string {
  const dc = dcLabel(save.dc);
  return `${save.abilityNamePt}${dc ? ` (${dc})` : ''}`;
}

/** The line of the saving throw a card asks again: the ability, and the DC when the server sent it. */
function saveLine(effect: LastingEffect): string {
  if (effect.endSave) {
    return `No fim de cada turno seu: teste de resistência de ${saveWords(effect.endSave)}.`;
  }
  if (effect.startSave) {
    return `No começo de cada turno seu: teste de resistência de ${saveWords(effect.startSave)}.`;
  }
  return '';
}

/** The cards of the effects on a combatant, as the caller may read them. */
export function combatCards(effects: readonly LastingEffect[] | undefined): EffectCardView[] {
  return (effects ?? []).map((e) => ({
    key: e.id,
    name: e.sourceNamePt,
    origin: e.originPt,
    tags: [...e.tagsPt],
    clock: e.clockTextPt,
    save: saveLine(e),
    changes: [...e.changesPt],
  }));
}

/** The cards of the effects on a character outside a combat: what the master left visible, with the time left. */
export function characterCards(effects: readonly CharacterEffect[]): EffectCardView[] {
  return effects.map((e) => ({
    key: e.id,
    name: e.sourceNamePt,
    origin: '',
    tags: [...e.tagsPt],
    clock: upFirst(e.durationTextPt),
    save: '',
    changes: [],
  }));
}

/** The names of the conditions the effects hold, once each, for the label under the cards. */
export function characterConditions(effects: readonly CharacterEffect[]): string[] {
  return [...new Set(effects.flatMap((e) => e.conditionNamesPt))];
}

/** The six levels of exhaustion (SRD 5.1, Conditions): each one adds to the ones below it. */
export const EXHAUSTION_LEVELS: readonly string[] = [
  'Desvantagem em testes de habilidade',
  'Deslocamento pela metade',
  'Desvantagem em ataques e testes de resistência',
  'PV máximos pela metade',
  'Deslocamento 0',
  'Morte',
];

/** The label of the header: "Exaustão 4". */
export function exhaustionLabel(level: number): string {
  return `Exaustão ${level}`;
}

/** What a level of exhaustion does, from the first level up to it. */
export function exhaustionLines(level: number): readonly string[] {
  return EXHAUSTION_LEVELS.slice(0, Math.max(0, Math.min(level, EXHAUSTION_LEVELS.length)));
}

/** A die an effect adds to or takes from a roll, as a field of the sheet asks for it. */
export interface ExtraDieField {
  readonly label: string;
  /** The effect that adds it ("Bênção"). */
  readonly name: string;
  readonly faces: number;
  readonly sign: number;
}

/** The dice the effects on a combatant add to (sign 1) or take from (sign -1) a kind of roll, as the caller may read them. */
export function rollDiceOf(
  effects: readonly LastingEffect[] | undefined,
  kind: EffectRollKind,
): readonly { readonly name: string; readonly faces: number; readonly sign: number }[] {
  return (effects ?? []).flatMap((e) =>
    e.modifiers
      .filter((m) => m.kind === EffectModifierKind.ROLL_DIE && m.appliesTo.includes(kind))
      .map((m) => ({ name: e.sourceNamePt, faces: m.die, sign: m.sign })),
  );
}

/** How many dice a refusal says the roll still takes ("the roll takes 1 more die(s)"), or 0 for another message. */
export function missingDice(message: string): number {
  const found = /takes (\d+) more die/.exec(message);
  return found ? Number(found[1]) : 0;
}

/** The label of a d4 field: "Resultado do d4 (Bênção)". */
export function dieFieldLabel(faces: number, name: string): string {
  return `Resultado do d${faces}${name ? ` (${name})` : ''}`;
}

/** "Bênção soma 1d4 a esta jogada. Role um d4 além do d20." */
export function dieFieldHint(f: ExtraDieField): string {
  const verb = f.sign < 0 ? 'subtrai' : 'soma';
  const to = f.sign < 0 ? 'desta' : 'a esta';
  return `${f.name || 'Um efeito'} ${verb} 1d${f.faces} ${to} jogada. Role um d${f.faces} além do d20.`;
}

/** The fields the effects ask, one die each; empty when no effect adds one. */
export function dieFields(
  dice: readonly { readonly name: string; readonly faces: number; readonly sign: number }[],
): ExtraDieField[] {
  return dice.map((d) => ({
    label: dieFieldLabel(d.faces, d.name),
    name: d.name,
    faces: d.faces,
    sign: d.sign,
  }));
}

/** The generic fields for the dice a refusal asked for when the effects behind them are not shown to this player. */
export function genericDieFields(count: number, faces: number): ExtraDieField[] {
  return Array.from({ length: count }, () => ({
    label: dieFieldLabel(faces, 'Outra fonte'),
    name: 'Outra fonte',
    faces,
    sign: 1,
  }));
}

/** "+1" and "−1" for a modifier. */
export function signed(n: number): string {
  return n < 0 ? `−${Math.abs(n)}` : `+${n}`;
}

/** "1d20 (12) + 1": the d20 of an effect's saving throw with its modifier and the dice effects added. */
export function saveFormula(result: {
  readonly d20: number;
  readonly modifier: number;
  readonly extraDice: readonly ExtraDie[];
  /** The Bardic Inspiration die the player added after the d20 (0 when none). */
  readonly inspirationSides?: number;
  readonly inspirationFace?: number;
}): string {
  const mod =
    result.modifier === 0 ? '' : ` ${result.modifier < 0 ? '−' : '+'} ${Math.abs(result.modifier)}`;
  const dice = result.extraDice
    .filter((d) => d.face > 0)
    .map((d) => ` ${d.sign < 0 ? '−' : '+'} 1d${d.faces} (${d.face})`)
    .join('');
  const inspiration = result.inspirationFace
    ? ` + 1d${result.inspirationSides ?? 0} de Inspiração (${result.inspirationFace})`
    : '';
  return `1d20 (${result.d20})${mod}${dice}${inspiration}`;
}

/** What the sheet of an effect's saving throw shows before the roll. */
export interface EffectSaveView {
  readonly title: string;
  readonly subtitle: string;
  /** The ability's name, in bold in the sentence. */
  readonly ability: string;
  /** What a pass and a failure do, as the server wrote it, after the ability sentence. */
  readonly consequence: string;
  /** "Seu modificador: +1." (empty when the app does not know the bonus). */
  readonly modifier: string;
  readonly modeLine: string;
  readonly mode: 'normal' | 'advantage' | 'disadvantage';
  readonly autoFail: boolean;
  readonly handed: boolean;
  /** "CD 14" when the server sent the DC to this player, else empty. */
  readonly dc: string;
  readonly dice: readonly ExtraDie[];
  /** "Raio Lunar, de Bartolo" for a zone's save; empty otherwise. */
  readonly source: string;
}

/** "Teste de resistência de Sabedoria." is the first sentence of `text_pt`; the rest is what a pass does. */
function afterAbility(text: string): string {
  // The ability may be named or empty in the server's sentence ("de ."): either way the lead is its first sentence.
  const lead = /^Teste de resistência de [^.]*\.\s*/.exec(text);
  return lead ? text.slice(lead[0].length).trim() : '';
}

/** The sheet of the saving throw an effect asks (RN-22), from the window's prompt. */
export function effectSaveView(
  prompt: EffectSavePrompt,
  round: number,
  /** The caster of the zone the save is about, from the encounter's zones; empty when not seen. */
  zoneCaster = '',
): EffectSaveView {
  const start = prompt.phase === EffectPhase.START;
  const mode =
    prompt.mode === 'advantage' || prompt.mode === 'disadvantage' ? prompt.mode : 'normal';
  const after = afterAbility(prompt.textPt);
  const ability = abilityName(prompt.ability, prompt.abilityNamePt);
  const base: EffectSaveView = {
    title: start ? 'Começo do seu turno' : 'Fim do seu turno',
    subtitle: prompt.sourceNamePt || `Rodada ${round}`,
    ability,
    consequence: after,
    source: '',
    modifier: prompt.bonusKnown ? `Seu modificador: ${signed(prompt.modifier)}.` : '',
    modeLine: modeLineOf(prompt, mode),
    mode,
    autoFail: prompt.autoFail,
    handed: prompt.handedToMaster,
    dc: dcLabel(prompt.dc),
    dice: prompt.extraDice,
  };
  return prompt.zoneId ? { ...base, ...zoneSheetWords(prompt, zoneCaster, after) } : base;
}

/** "Você tem vantagem neste teste (Esquivando: vantagem)." and its two siblings. */
function modeLineOf(prompt: EffectSavePrompt, mode: EffectSaveView['mode']): string {
  const sources = prompt.modeSourcesPt.length > 0 ? ` (${prompt.modeSourcesPt.join('; ')})` : '';
  if (mode === 'advantage') {
    return `Você tem vantagem neste teste${sources}.`;
  }
  return mode === 'disadvantage'
    ? `Você tem desvantagem neste teste${sources}.`
    : 'Você não tem vantagem nem desvantagem neste teste.';
}

/** A note of the turn split at its first sentence, which is read in bold ("Você está Paralisada." / "Não age nem se move neste turno."). */
export function noteParts(text: string): { readonly lead: string; readonly rest: string } {
  const at = text.indexOf('. ');
  return at < 0
    ? { lead: text, rest: '' }
    : { lead: text.slice(0, at + 1), rest: text.slice(at + 2) };
}

/** The short labels the order and the map show for the effects on a combatant ("Paralisado", "Preso numa teia"), once each,
 * after the names of its conditions (which the labels may repeat). The server sends only what the caller may read. */
export function labelTags(
  conditions: readonly string[],
  labels: readonly { readonly textPt: string }[] | undefined,
): string[] {
  const known = new Set(conditions);
  // A condition the master marked is also an effect whose label is the names of its conditions joined by ", ":
  // that label says nothing the conditions do not, so it is not a tag of its own.
  const repeats = (text: string): boolean => text.split(', ').every((part) => known.has(part));
  const extra = (labels ?? []).map((l) => l.textPt).filter((text) => text !== '' && !repeats(text));
  return [...new Set([...conditions, ...extra])];
}
