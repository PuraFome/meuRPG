import {
  Ability,
  type CreatureAction,
  CreatureActionClass,
  CreatureActionKind,
  type CreatureArea,
  CreatureAreaShape,
  type CreatureDamagePart,
  SpellAreaShape,
  SpellSaveSuccess,
} from '../../../gen/meurpg/rules/v1/rules_pb';
import {
  AreaPlacement,
  type CreatureActionResult,
  SaveOutcome,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { abilityName } from '../content/feat-text';
import { dcLabel } from '../format/save-dc';
import { joinDots, tight } from '../format/text';
import { SQUARE_FT, metersText } from '../units';
import { d20Formula } from './roll-mode';
import type { AreaShape } from './spell-area';

/**
 * An NPC's actions that are no attack roll (SRD 5.1, Monsters): a breath weapon or anything else that asks a saving throw,
 * and the actions with no roll at all. The stat block reaches the master only (RN-20). Pure functions, tested without a DOM.
 */

/** The actions the master can use with `UseCreatureAction`: a saving throw or no roll. Attacks and the Multiattack are made as before. */
export function usableActions(actions: readonly CreatureAction[] | undefined): CreatureAction[] {
  return (actions ?? []).filter(
    (a) =>
      a.key !== '' && (a.kind === CreatureActionKind.SAVE || a.kind === CreatureActionKind.OTHER),
  );
}

/** The action's name, in Portuguese when the content has it. */
export function actionName(a: CreatureAction): string {
  return a.namePt || a.name;
}

/** Whether the action asks a saving throw (otherwise it is a reminder that spends the action). */
export function asksSave(a: CreatureAction): boolean {
  return a.kind === CreatureActionKind.SAVE && !!a.save;
}

/** "2d6 de fogo + 1d4 de ácido": the damage parts, in the stat block's order. */
export function damagePartsText(parts: readonly CreatureDamagePart[]): string {
  return parts.map((p) => `${p.dice} de ${p.damageTypePt || 'dano'}`).join(' + ');
}

/** How the SRD words a duration ("1 minute", "10 minutes", "1 hour", "1 round"), in Portuguese; any other wording stays as the SRD has it. */
export function durationPt(text: string): string {
  const m = /^(\d+)\s+(round|minute|hour|day)s?$/i.exec(text.trim());
  if (!m) {
    return text.trim();
  }
  const n = Number(m[1]);
  const unit = { round: 'rodada', minute: 'minuto', hour: 'hora', day: 'dia' }[
    m[2].toLowerCase() as 'round' | 'minute' | 'hour' | 'day'
  ];
  return `${n} ${unit}${n === 1 ? '' : 's'}`;
}

/** The shape of the area in words: "cone de 4,5 m", "linha de 18 m × 1,5 m", "raio de 3 m ao redor dele". */
export function creatureAreaText(area: CreatureArea | undefined): string {
  if (!area) {
    return '';
  }
  const size = metersText(area.sizeFt);
  switch (area.shape) {
    case CreatureAreaShape.CONE:
      return `cone de ${size}`;
    case CreatureAreaShape.LINE:
      return `linha de ${size} × ${metersText(Math.max(area.widthFt, SQUARE_FT))}`;
    case CreatureAreaShape.SPHERE:
      return `esfera de ${size} de raio`;
    case CreatureAreaShape.CUBE:
      return `cubo de ${size}`;
    case CreatureAreaShape.CYLINDER:
      return `cilindro de ${size} de raio`;
    case CreatureAreaShape.RADIUS:
      return `raio de ${size} ao redor dele`;
    default:
      return '';
  }
}

/** Where the master places the area, as `CastSpell` does: a point, a direction, or nothing (centered on the creature). `null`: no area. */
export function areaPlacementOf(a: CreatureAction): AreaPlacement | null {
  switch (a.area?.shape) {
    case CreatureAreaShape.CONE:
    case CreatureAreaShape.LINE:
    case CreatureAreaShape.CUBE:
      return AreaPlacement.DIRECTION;
    case CreatureAreaShape.SPHERE:
    case CreatureAreaShape.CYLINDER:
      return AreaPlacement.POINT;
    case CreatureAreaShape.RADIUS:
      return AreaPlacement.CASTER;
    default:
      return null;
  }
}

/** The area as the picker draws it (the same shape the spells have). */
export function areaShapeOf(a: CreatureAction): AreaShape {
  const area = a.area;
  const shape = {
    [CreatureAreaShape.CONE]: SpellAreaShape.CONE,
    [CreatureAreaShape.LINE]: SpellAreaShape.LINE,
    [CreatureAreaShape.CUBE]: SpellAreaShape.CUBE,
    [CreatureAreaShape.SPHERE]: SpellAreaShape.SPHERE,
    [CreatureAreaShape.CYLINDER]: SpellAreaShape.CYLINDER,
    [CreatureAreaShape.RADIUS]: SpellAreaShape.SPHERE,
  } as Partial<Record<CreatureAreaShape, SpellAreaShape>>;
  return {
    shape: shape[area?.shape ?? CreatureAreaShape.UNSPECIFIED] ?? SpellAreaShape.UNSPECIFIED,
    sizeFt: area?.sizeFt ?? 0,
    widthFt: area?.widthFt ?? 0,
  };
}

/**
 * What the action does, in Portuguese lines, from the plan the server reads: the saving throw and its DC, the damage and what a
 * success does, the area, the condition with its duration. An action with no roll says so. Never a number the plan does not hold.
 */
export function actionLines(a: CreatureAction): string[] {
  if (!asksSave(a)) {
    return ['Sem teste: gasta a ação e lembra o texto.'];
  }
  const s = a.save!;
  const lines: string[] = [];
  const ability = s.ability === Ability.UNSPECIFIED ? '' : abilityName(s.ability);
  const test = ability ? `Salvaguarda de ${ability}` : 'Salvaguarda';
  lines.push(
    joinDots([s.dc > 0 ? `${test}, CD ${s.dc}` : test, creatureAreaText(a.area)].filter(Boolean)),
  );
  if (s.damage.length > 0) {
    const half =
      s.onSuccess === SpellSaveSuccess.HALF
        ? ', metade se passar'
        : s.onSuccess === SpellSaveSuccess.NONE
          ? ', nada se passar'
          : '';
    lines.push(`Dano: ${damagePartsText(s.damage)}${half}`);
  }
  if (s.conditionPt || s.conditionKey) {
    const duration = s.duration ? ` por ${durationPt(s.duration)}` : '';
    const repeat = s.repeatSave ? '; repete o teste no fim de cada turno dele' : '';
    lines.push(`Se falhar: ${s.conditionPt || s.conditionKey}${duration}${repeat}`);
  }
  return lines;
}

/** The note under the lines: how far the app reads the text. `''` for an action the app reads whole. */
export function classNote(a: CreatureAction): string {
  switch (a.planClass) {
    case CreatureActionClass.PARTIAL:
      return 'O texto diz algo que o app não calcula: leia o lembrete.';
    case CreatureActionClass.TEXT:
      return 'O app não calcula esta ação: o texto é o lembrete.';
    default:
      return '';
  }
}

/** Whether the SRD's text is shown as a reminder (the master reads it): anything the plan does not hold whole. */
export function showsText(a: CreatureAction): boolean {
  return (
    a.kind === CreatureActionKind.OTHER ||
    a.planClass === CreatureActionClass.PARTIAL ||
    a.planClass === CreatureActionClass.TEXT
  );
}

/** One creature the action reached: who, how its saving throw went and what the action gave it. */
export interface TargetLine {
  readonly id: string;
  readonly label: string;
  /** "Falhou", "Passou: metade do dano", "Falha automática"; `''` when the target rolled nothing. */
  readonly word: string;
  readonly saved: boolean;
  /** "Salvaguarda: 1d20 (12) + 2 = 14 contra CD 13", or `''`. */
  readonly roll: string;
  /** "Ficou Amedrontado", or `''`. */
  readonly condition: string;
}

/**
 * What a used action did to each creature, from the server's answer: the d20 the target made against the DC (the master reads
 * both), whether it saved (and half the damage when the action says so), and the condition a failure gave.
 */
export function targetLines(
  result: CreatureActionResult,
  labels: ReadonlyMap<string, string>,
  half: boolean,
): TargetLine[] {
  return result.targets.map((t) => {
    const save = t.save;
    const saved = save?.outcome === SaveOutcome.SAVED;
    let word = '';
    if (save) {
      word = saved ? (half ? 'Passou: metade do dano' : 'Passou') : 'Falhou';
      if (save.autoFailed || save.autoFail) {
        word = 'Falha automática';
      }
    }
    const dc = save?.dc ? ` contra ${dcLabel(save.dc)}` : '';
    return {
      id: t.combatantId,
      label: labels.get(t.combatantId) ?? 'Alvo',
      word,
      saved,
      roll: save?.roll ? tight(`Salvaguarda: ${d20Formula(save.roll)}${dc}`) : '',
      condition:
        t.conditionApplied && result.conditionNamePt ? `Ficou ${result.conditionNamePt}` : '',
    };
  });
}

/** The notes of a used action: the duration and the repeated save the stat block words, for the master to keep. */
export function resultNotes(result: CreatureActionResult): string[] {
  const notes: string[] = [];
  if (result.conditionNamePt && result.durationText) {
    notes.push(`${result.conditionNamePt} por ${durationPt(result.durationText)}.`);
  }
  if (result.repeatSave) {
    notes.push('O alvo repete o teste no fim de cada turno dele; o app não conta o tempo.');
  }
  return notes;
}
