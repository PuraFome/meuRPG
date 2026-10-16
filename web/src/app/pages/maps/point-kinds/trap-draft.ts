import type { MessageInitShape } from '@bufbuild/protobuf';

import {
  type MapPoint,
  TrapState,
  type TrapSpecSchema,
} from '../../../../gen/meurpg/maps/v1/maps_pb';
import {
  Ability,
  type TrapEffect,
  type TrapPreset,
  TrapPassOutcome,
  TrapSaveApplies,
  TrapTargets,
  TrapTrigger,
} from '../../../../gen/meurpg/rules/v1/rules_pb';
import { DAMAGE_TYPE_OPTIONS } from '../../../core/characters/character-labels';
import { metersText } from '../../../core/units';
import type { PointChanges } from '../../../core/maps/maps-client';
import type {
  AttackDraft,
  ConditionDraft,
  DamageDraft,
  SaveDraft,
  TrapDraft,
} from './trap-draft-types';

/**
 * What the trap form edits (E9-02, MR-035): the form's own text and choices, kept apart from the saved point
 * until "Salvar ponto". The effect is a list of parts the master adds and removes; each field is the text he
 * typed, so a half-typed number is not lost. The server's rules (maps.proto) are checked first so each field
 * can say what is wrong, and the server stays the authority. Pure functions: tested without a DOM.
 */
export type {
  AttackDraft,
  ConditionDraft,
  DamageDraft,
  SaveDraft,
  TrapDraft,
} from './trap-draft-types';
export { type TrapErrors, hasTrapErrors, trapErrors, validDice } from './trap-errors';

export const MAX_DAMAGE_PARTS = 4;
export const MAX_CONDITION_PARTS = 4;

/** The damage types the SRD names, by the server's key (`damage-type:poison`), with the words the rest of the app uses (`character-labels`). */
export const DAMAGE_TYPES: readonly { readonly key: string; readonly name: string }[] =
  DAMAGE_TYPE_OPTIONS.map((o) => ({
    key: `damage-type:${o.key}`,
    name: o.label,
  }));

export const ABILITIES: readonly { readonly value: Ability; readonly name: string }[] = [
  { value: Ability.STRENGTH, name: 'Força' },
  { value: Ability.DEXTERITY, name: 'Destreza' },
  { value: Ability.CONSTITUTION, name: 'Constituição' },
  { value: Ability.INTELLIGENCE, name: 'Inteligência' },
  { value: Ability.WISDOM, name: 'Sabedoria' },
  { value: Ability.CHARISMA, name: 'Carisma' },
];

/** The skills that find a trap with no word from the master: Percepção (against the DC to notice) and Investigação (against the DC to find). */
export const SKILLS_ALWAYS_FIND = ['skill:perception', 'skill:investigation'] as const;

/** The most skills "Também acham com" takes: the SRD's 18 less the two that always find. */
export const ALSO_FIND_MAX = 16;

/** The Arcanismo key: the SRD's rule that any character may try it against a magic trap, which the app only reminds of. */
export const ARCANA_KEY = 'skill:arcana';

/** The skills "Também acham com" offers, from the rules' skills (key and Portuguese name): every one but the two that always find, as the list's order. */
export function alsoFindOptions<T extends { readonly key: string }>(skills: readonly T[]): T[] {
  return skills.filter((s) => !(SKILLS_ALWAYS_FIND as readonly string[]).includes(s.key));
}

/** The skill picked or put back, kept in the order of the list the master sees. */
export function toggleAlsoFind(
  current: readonly string[],
  key: string,
  order: readonly string[],
): string[] {
  const next = current.includes(key) ? current.filter((k) => k !== key) : [...current, key];
  const rank = (k: string) => (order.includes(k) ? order.indexOf(k) : order.length);
  return next.sort((a, b) => rank(a) - rank(b));
}

const blankDamage = (): DamageDraft => ({ dice: '', typeKey: 'damage-type:bludgeoning' });

/** The parts the "+" actions add. */
export function newDamage(): DamageDraft {
  return blankDamage();
}

export function newCondition(): ConditionDraft {
  return { key: 'condition:prone', duration: '' };
}

export function newAttack(): AttackDraft {
  return { bonus: '', count: '1', damage: { dice: '', typeKey: 'damage-type:piercing' } };
}

export function newSave(): SaveDraft {
  return {
    ability: Ability.DEXTERITY,
    dc: '',
    appliesTo: TrapSaveApplies.CAUGHT,
    failDamage: [],
    failCondition: null,
    onPass: TrapPassOutcome.NONE,
  };
}

/** A trap from nothing ("Começar do zero"): the square alone, fired by entering it, with no effect yet. */
export function blankTrapDraft(name = ''): TrapDraft {
  return {
    presetKey: '',
    name,
    description: '',
    noticeDc: '',
    findDc: '',
    alsoFind: [],
    areaSize: 1,
    trigger: TrapTrigger.ENTER,
    state: TrapState.ARMED,
    targets: TrapTargets.AREA,
    attack: null,
    damage: [],
    conditions: [],
    save: null,
  };
}

function damageDraft(d: { dice: string; damageTypeKey: string }): DamageDraft {
  return { dice: d.dice, typeKey: d.damageTypeKey };
}

function conditionDraft(c: { conditionKey: string; durationPt: string }): ConditionDraft {
  return { key: c.conditionKey, duration: c.durationPt };
}

function attackDraft(attack: NonNullable<TrapEffect['attack']>): AttackDraft {
  return {
    bonus: String(attack.bonus),
    count: String(attack.count),
    damage: attack.damage ? damageDraft(attack.damage) : blankDamage(),
  };
}

function saveDraftOf(save: NonNullable<TrapEffect['save']>): SaveDraft {
  return {
    ability: save.ability,
    dc: String(save.dc),
    appliesTo:
      save.appliesTo === TrapSaveApplies.HIT ? TrapSaveApplies.HIT : TrapSaveApplies.CAUGHT,
    failDamage: (save.onFail?.damage ?? []).map(damageDraft),
    failCondition: save.onFail?.condition ? conditionDraft(save.onFail.condition) : null,
    onPass: save.onPass === TrapPassOutcome.HALF ? TrapPassOutcome.HALF : TrapPassOutcome.NONE,
  };
}

function effectDraft(
  effect: TrapEffect | undefined,
): Pick<TrapDraft, 'targets' | 'attack' | 'damage' | 'conditions' | 'save'> {
  return {
    targets: effect?.targets === TrapTargets.MANUAL ? TrapTargets.MANUAL : TrapTargets.AREA,
    attack: effect?.attack ? attackDraft(effect.attack) : null,
    damage: (effect?.damage ?? []).map(damageDraft),
    conditions: (effect?.conditions ?? []).map(conditionDraft),
    save: effect?.save ? saveDraftOf(effect.save) : null,
  };
}

/** The form filled from a preset of the SRD: it only fills, and everything stays editable. */
export function trapDraftFromPreset(preset: TrapPreset): TrapDraft {
  return {
    presetKey: preset.key,
    name: preset.namePt,
    description: preset.descriptionPt,
    noticeDc: preset.noticeDc > 0 ? String(preset.noticeDc) : '',
    findDc: String(preset.findDc),
    // A preset marks no skill: not even a magic one wants Arcanismo unless the master says so.
    alsoFind: [],
    areaSize: clampArea(preset.areaSize),
    trigger: preset.trigger === TrapTrigger.MANUAL ? TrapTrigger.MANUAL : TrapTrigger.ENTER,
    state: TrapState.ARMED,
    ...effectDraft(preset.effect),
  };
}

function clampArea(n: number): 1 | 2 | 3 | 4 {
  return n >= 4 ? 4 : n === 3 ? 3 : n === 2 ? 2 : 1;
}

/** The trigger the form offers: the saved one when it is manual, "ao entrar" otherwise. */
function triggerOf(trigger: TrapTrigger | undefined): TrapTrigger {
  return trigger === TrapTrigger.MANUAL ? TrapTrigger.MANUAL : TrapTrigger.ENTER;
}

/** The state the form shows: fired or disarmed as saved, armed otherwise. */
function stateOf(state: TrapState | undefined): TrapState {
  return state === TrapState.TRIGGERED || state === TrapState.DISARMED ? state : TrapState.ARMED;
}

/** The form as the saved point has it. */
export function trapDraftOf(point: Pick<MapPoint, 'name' | 'description' | 'trap'>): TrapDraft {
  const t = point.trap;
  return {
    presetKey: t?.presetKey ?? '',
    name: point.name,
    description: point.description,
    noticeDc: t && t.noticeDc > 0 ? String(t.noticeDc) : '',
    findDc: t ? String(t.findDc) : '',
    alsoFind: t?.alsoFindSkillKeys ?? [],
    areaSize: clampArea(t?.areaSize ?? 1),
    trigger: triggerOf(t?.trigger),
    state: stateOf(t?.state),
    ...effectDraft(t?.effect),
  };
}

function damageSpec(d: DamageDraft) {
  return { dice: d.dice.trim(), damageTypeKey: d.typeKey };
}

function conditionSpec(c: ConditionDraft) {
  return { conditionKey: c.key, durationPt: c.duration.trim() };
}

/** What `CreateMapPoint` and `UpdateMapPoint` take as the trap (the whole spec). */
export function trapSpecOf(d: TrapDraft): MessageInitShape<typeof TrapSpecSchema> {
  return {
    presetKey: d.presetKey,
    noticeDc: d.noticeDc.trim() === '' ? 0 : Number(d.noticeDc),
    findDc: Number(d.findDc),
    alsoFindSkillKeys: [...d.alsoFind],
    areaSize: d.areaSize,
    trigger: d.trigger,
    state: d.state,
    effect: {
      targets: d.targets,
      ...(d.attack
        ? {
            attack: {
              bonus: Number(d.attack.bonus),
              count: Number(d.attack.count),
              damage: damageSpec(d.attack.damage),
            },
          }
        : {}),
      damage: d.damage.map(damageSpec),
      conditions: d.conditions.map(conditionSpec),
      ...(d.save
        ? {
            save: {
              ability: d.save.ability,
              dc: Number(d.save.dc),
              appliesTo: d.save.appliesTo,
              onPass: d.save.onPass,
              onFail: {
                damage: d.save.failDamage.map(damageSpec),
                ...(d.save.failCondition ? { condition: conditionSpec(d.save.failCondition) } : {}),
              },
            },
          }
        : {}),
    },
  };
}

/**
 * Whether the form differs from what the point has (the page asks before leaving it). `openedState` is the state the form opened with: a
 * trap that fired while the form was open is not an edit of the master's.
 */
export function isTrapDirty(
  d: TrapDraft,
  point: Pick<MapPoint, 'name' | 'description' | 'trap'>,
  openedState?: TrapState,
): boolean {
  const saved = trapDraftOf(point);
  return (
    JSON.stringify(d) !==
    JSON.stringify(openedState === undefined ? saved : { ...saved, state: openedState })
  );
}

/**
 * What "Salvar ponto" sends; `null` when nothing changed. The spec goes whole, as `UpdateMapPoint` replaces it, except the state: the
 * server keeps the current one unless the master chose another than the form opened with (the live game changes it too).
 */
export function trapChangesOf(
  d: TrapDraft,
  point: Pick<MapPoint, 'name' | 'description' | 'trap'>,
  openedState?: TrapState,
): PointChanges | null {
  if (!isTrapDirty(d, point, openedState)) {
    return null;
  }
  const spec = trapSpecOf(d);
  const changes: { -readonly [K in keyof PointChanges]: PointChanges[K] } = {
    trap:
      openedState !== undefined && d.state === openedState
        ? { ...spec, state: TrapState.UNSPECIFIED }
        : spec,
  };
  if (d.name.trim() !== point.name) {
    changes.name = d.name.trim();
  }
  if (d.description !== point.description) {
    changes.description = d.description;
  }
  return changes;
}

const ABILITY_NAME = new Map(ABILITIES.map((a) => [a.value, a.name]));

/** One line about a preset for its radio: "Queda de 6 m, 2d6", "1 perfurante, 2d10 veneno · resistência de Constituição". The server's
 * words and numbers, only put in a line. */
export function presetSummary(p: TrapPreset): string {
  const e = p.effect;
  const dmg = (d: { dice: string; damageTypePt: string }) => `${d.dice} ${d.damageTypePt}`.trim();
  if (p.fallFt > 0 && e && e.damage.length > 0) {
    return `Queda de ${metersText(p.fallFt)}, ${e.damage[0].dice}`;
  }
  const parts: string[] = [];
  if (e?.attack) {
    parts.push(`ataque +${e.attack.bonus}${e.attack.damage ? `, ${dmg(e.attack.damage)}` : ''}`);
  }
  if (e && e.damage.length > 0) {
    parts.push(e.damage.map(dmg).join(', '));
  }
  if (e?.save) {
    parts.push(`resistência de ${ABILITY_NAME.get(e.save.ability) ?? 'habilidade'}`);
  }
  const text = parts.join(' · ');
  return text === '' ? 'Só descrição' : text.charAt(0).toUpperCase() + text.slice(1);
}
