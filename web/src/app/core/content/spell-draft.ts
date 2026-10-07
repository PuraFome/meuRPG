import {
  Ability,
  CastingTimeUnit,
  SpellDurationKind,
  SpellDurationUnit,
  SpellRangeKind,
  SpellSaveSuccess,
} from '../../../gen/meurpg/rules/v1/rules_pb';
import type { MessageInitShape } from '@bufbuild/protobuf';

import {
  TableAreaShape,
  type TableSpell,
  type TableSpellDamageSchema,
  type TableSpellSchema,
  TableSpellTargetKind,
} from '../../../gen/meurpg/rules/v1/table_content_pb';
import { formatMeters, feetToMeters } from '../units';
import type { SpellDetailsVm } from '../../shared/spell-details/spell-details.types';
import { formatCastingTime, formatComponents, formatDuration, formatRange } from '../../shared/spell-details/spell-details-format';
import { rangeFeet, rangeMeters } from './effect-draft';

/**
 * The spell editor's form (MR-025, RN-23, E10-01 states 4, 4b and 5) and what it sends. A table spell is a
 * `TableSpell`: the same details an SRD spell has, so the combat that resolves an SRD spell resolves this one
 * (ADR-0018). The mechanic is optional: "Só texto" is always valid. Distances are typed in metres, in steps of 1,5 m,
 * and stored in feet (5 ft = 1,5 m); the server checks every limit, the form only keeps what the person typed.
 */

export type SpellInit = MessageInitShape<typeof TableSpellSchema>;

export type RangeChoice = 'ranged' | 'self' | 'touch' | 'sight' | 'unlimited';
export type DurationChoice = 'instant' | 'timed' | 'until_dispelled';
export type TargetChoice = 'creature' | 'creatures' | 'area' | 'self';
export type MechanicChoice = 'text' | 'attack' | 'save' | 'heal';
export type TimeChoice = 'action' | 'bonus_action' | 'reaction' | 'minute' | 'hour';

export interface SpellDraft {
  name: string;
  /** 0 is a truque. */
  level: number;
  schoolKey: string;
  time: TimeChoice;
  timeAmount: number;
  trigger: string;
  range: RangeChoice;
  rangeM: string;
  duration: DurationChoice;
  durationAmount: number;
  durationUnit: 'round' | 'minute' | 'hour' | 'day';
  upTo: boolean;
  verbal: boolean;
  somatic: boolean;
  material: boolean;
  materialPt: string;
  concentration: boolean;
  ritual: boolean;
  classKeys: string[];
  text: string;
  higher: string;
  target: TargetChoice;
  count: number;
  perSlot: number;
  shape: 'cone' | 'cube' | 'cylinder' | 'line' | 'sphere';
  sizeM: string;
  mechanic: MechanicChoice;
  attack: 'melee' | 'ranged';
  saveAbility: Ability;
  saveOnSuccess: 'half' | 'none';
  dice: string;
  damageType: string;
  /** "+1d8 por nível acima do 1º" (a leveled spell) or at the cantrip tiers (a truque). */
  more: string;
  healDice: string;
  healMore: string;
  healModifier: boolean;
  /** Damage entries after the first, kept as they are (the form edits one). */
  otherDamage: readonly OtherDamage[];
}

export interface OtherDamage {
  readonly damageTypeKey: string;
  readonly dice: string;
  readonly perSlotLevel: string;
  readonly perTier: string;
}

export function emptySpell(): SpellDraft {
  return {
    name: '',
    level: 1,
    schoolKey: 'school:evocation',
    time: 'action',
    timeAmount: 1,
    trigger: '',
    range: 'ranged',
    rangeM: '',
    duration: 'instant',
    durationAmount: 1,
    durationUnit: 'minute',
    upTo: false,
    verbal: true,
    somatic: true,
    material: false,
    materialPt: '',
    concentration: false,
    ritual: false,
    classKeys: [],
    text: '',
    higher: '',
    target: 'creature',
    count: 2,
    perSlot: 0,
    shape: 'cone',
    sizeM: '',
    mechanic: 'text',
    attack: 'ranged',
    saveAbility: Ability.DEXTERITY,
    saveOnSuccess: 'half',
    dice: '',
    damageType: 'damage-type:necrotic',
    more: '',
    healDice: '',
    healMore: '',
    healModifier: true,
    otherDamage: [],
  };
}

const TIME_TO_UNIT: Record<TimeChoice, CastingTimeUnit> = {
  action: CastingTimeUnit.ACTION,
  bonus_action: CastingTimeUnit.BONUS_ACTION,
  reaction: CastingTimeUnit.REACTION,
  minute: CastingTimeUnit.MINUTE,
  hour: CastingTimeUnit.HOUR,
};
const RANGE_TO_KIND: Record<RangeChoice, SpellRangeKind> = {
  ranged: SpellRangeKind.RANGED,
  self: SpellRangeKind.SELF,
  touch: SpellRangeKind.TOUCH,
  sight: SpellRangeKind.SIGHT,
  unlimited: SpellRangeKind.UNLIMITED,
};
const DURATION_UNIT: Record<SpellDraft['durationUnit'], SpellDurationUnit> = {
  round: SpellDurationUnit.ROUND,
  minute: SpellDurationUnit.MINUTE,
  hour: SpellDurationUnit.HOUR,
  day: SpellDurationUnit.DAY,
};
const SHAPE_TO_ENUM: Record<SpellDraft['shape'], TableAreaShape> = {
  cone: TableAreaShape.CONE,
  cube: TableAreaShape.CUBE,
  cylinder: TableAreaShape.CYLINDER,
  line: TableAreaShape.LINE,
  sphere: TableAreaShape.SPHERE,
};

export const TIME_OPTIONS: readonly { value: TimeChoice; label: string }[] = [
  { value: 'action', label: 'Ação' },
  { value: 'bonus_action', label: 'Ação bônus' },
  { value: 'reaction', label: 'Reação' },
  { value: 'minute', label: 'Minutos' },
  { value: 'hour', label: 'Horas' },
];

/** The range offers "Pessoal" and "Toque" beside a distance (E10-01 state 4b; etapa10-web-notes, 10.1b). */
export const RANGE_OPTIONS: readonly { value: RangeChoice; label: string }[] = [
  { value: 'ranged', label: 'Distância' },
  { value: 'self', label: 'Pessoal' },
  { value: 'touch', label: 'Toque' },
  { value: 'sight', label: 'À vista' },
  { value: 'unlimited', label: 'Ilimitado' },
];

export const DURATION_OPTIONS: readonly { value: DurationChoice; label: string }[] = [
  { value: 'instant', label: 'Instantânea' },
  { value: 'timed', label: 'Por um tempo' },
  { value: 'until_dispelled', label: 'Até ser dissipada' },
];

export const DURATION_UNITS: readonly { value: SpellDraft['durationUnit']; label: string }[] = [
  { value: 'round', label: 'Rodadas' },
  { value: 'minute', label: 'Minutos' },
  { value: 'hour', label: 'Horas' },
  { value: 'day', label: 'Dias' },
];

export const TARGET_OPTIONS: readonly { value: TargetChoice; label: string }[] = [
  { value: 'creature', label: 'Uma criatura' },
  { value: 'creatures', label: 'Várias criaturas' },
  { value: 'area', label: 'Área' },
  { value: 'self', label: 'Só quem conjura' },
];

export const MECHANIC_OPTIONS: readonly { value: MechanicChoice; label: string }[] = [
  { value: 'text', label: 'Só texto' },
  { value: 'attack', label: 'Ataque' },
  { value: 'save', label: 'Teste de resistência' },
  { value: 'heal', label: 'Cura' },
];

export const SHAPE_OPTIONS: readonly { value: SpellDraft['shape']; label: string }[] = [
  { value: 'cone', label: 'Cone' },
  { value: 'cube', label: 'Cubo' },
  { value: 'cylinder', label: 'Cilindro' },
  { value: 'line', label: 'Linha' },
  { value: 'sphere', label: 'Esfera' },
];

/** The size's label follows the shape: the length of a cone and a line, the side of a cube, the radius of a sphere and a cylinder. */
export function sizeLabel(shape: SpellDraft['shape']): string {
  switch (shape) {
    case 'cube':
      return 'Lado';
    case 'sphere':
    case 'cylinder':
      return 'Raio';
    default:
      return 'Comprimento';
  }
}

export const SAVE_SUCCESS_OPTIONS: readonly { value: SpellDraft['saveOnSuccess']; label: string }[] = [
  { value: 'half', label: 'Metade do dano' },
  { value: 'none', label: 'Nada' },
];

export const ATTACK_OPTIONS: readonly { value: SpellDraft['attack']; label: string }[] = [
  { value: 'ranged', label: 'Ataque de magia à distância' },
  { value: 'melee', label: 'Ataque de magia corpo a corpo' },
];

export function circleLabel(level: number): string {
  return level === 0 ? 'Truque' : `${level}º nível`;
}

/** "Mais dano por nível acima do 1º", or at the truque's tiers. */
export function moreLabel(level: number): string {
  return level === 0 ? 'Mais dano a cada degrau do truque' : `Mais dano por nível acima do ${level}º`;
}

export function moreHint(level: number): string {
  return level === 0 ? 'Nos níveis 5, 11 e 17 do personagem.' : 'Ao conjurar com um espaço maior.';
}

/** A stored spell, as a form. */
export function spellToDraft(s: TableSpell): SpellDraft {
  const d = emptySpell();
  d.name = s.namePt;
  d.level = s.level;
  d.schoolKey = s.schoolKey;
  const ct = s.castingTime;
  d.time = (Object.entries(TIME_TO_UNIT).find(([, u]) => u === ct?.unit)?.[0] ?? 'action') as TimeChoice;
  d.timeAmount = ct?.amount || 1;
  d.trigger = ct?.triggerPt ?? '';
  d.range = (Object.entries(RANGE_TO_KIND).find(([, k]) => k === s.range?.kind)?.[0] ?? 'ranged') as RangeChoice;
  d.rangeM = rangeMeters(s.range?.distanceFt ?? 0);
  const du = s.duration;
  d.duration = du?.kind === SpellDurationKind.TIMED ? 'timed' : du?.kind === SpellDurationKind.UNTIL_DISPELLED ? 'until_dispelled' : 'instant';
  d.durationAmount = du?.amount || 1;
  d.durationUnit = (Object.entries(DURATION_UNIT).find(([, u]) => u === du?.unit)?.[0] ?? 'minute') as SpellDraft['durationUnit'];
  d.upTo = du?.upTo ?? false;
  d.verbal = s.components?.verbal ?? false;
  d.somatic = s.components?.somatic ?? false;
  d.material = s.components?.material ?? false;
  d.materialPt = s.components?.materialPt ?? '';
  d.concentration = s.concentration;
  d.ritual = s.ritual;
  d.classKeys = [...s.classKeys];
  d.text = s.descPt.join('\n\n');
  d.higher = s.higherLevelPt.join('\n\n');
  const t = s.target;
  switch (t?.kind) {
    case TableSpellTargetKind.CREATURES:
      d.target = 'creatures';
      break;
    case TableSpellTargetKind.AREA:
      d.target = 'area';
      break;
    case TableSpellTargetKind.SELF:
      d.target = 'self';
      break;
    default:
      d.target = 'creature';
  }
  d.count = t?.count || 2;
  d.perSlot = t?.perSlotLevel ?? 0;
  d.shape = (Object.entries(SHAPE_TO_ENUM).find(([, e]) => e === t?.shape)?.[0] ?? 'cone') as SpellDraft['shape'];
  d.sizeM = rangeMeters(t?.sizeFt ?? 0);
  if (s.attack) {
    d.mechanic = 'attack';
    d.attack = s.attack === 'melee' ? 'melee' : 'ranged';
  } else if (s.save) {
    d.mechanic = 'save';
    d.saveAbility = s.save.ability;
    d.saveOnSuccess = s.save.onSuccess === SpellSaveSuccess.NONE ? 'none' : 'half';
  } else if (s.heal) {
    d.mechanic = 'heal';
  }
  const [first, ...rest] = s.damage;
  if (first) {
    d.dice = first.dice;
    d.damageType = first.damageTypeKey;
    d.more = s.level === 0 ? first.perTier : first.perSlotLevel;
    d.otherDamage = rest.map((r) => ({ damageTypeKey: r.damageTypeKey, dice: r.dice, perSlotLevel: r.perSlotLevel, perTier: r.perTier }));
  }
  if (s.heal) {
    d.healDice = s.heal.dice;
    d.healMore = s.heal.perSlotLevel;
    d.healModifier = s.heal.addsModifier;
  }
  return d;
}

/** The paragraphs of a textarea: blank lines between them. */
export function paragraphs(text: string): string[] {
  return text
    .split(/\n\s*\n/)
    .map((p) => p.trim())
    .filter((p) => p !== '');
}

/** The body of a request, from the form. Only what the chosen mechanic uses goes: a spell of "Só texto" has no attack, no
 * saving throw, no damage and no healing; a spell that is not an area has no shape. */
export function draftToSpell(d: SpellDraft): SpellInit {
  const unitOf = TIME_TO_UNIT[d.time];
  const oneShot = d.time === 'action' || d.time === 'bonus_action' || d.time === 'reaction';
  const target: NonNullable<SpellInit['target']> = { kind: TARGET_KIND[d.target] };
  if (d.target === 'creatures') {
    target.count = d.count;
    target.perSlotLevel = d.perSlot;
  } else if (d.target === 'creature' && d.perSlot > 0) {
    target.perSlotLevel = d.perSlot;
  } else if (d.target === 'area') {
    target.shape = SHAPE_TO_ENUM[d.shape];
    target.sizeFt = rangeFeet(d.sizeM);
  }
  const damage: MessageInitShape<typeof TableSpellDamageSchema>[] = [];
  const hasDamage = (d.mechanic === 'attack' || d.mechanic === 'save') && d.dice.trim() !== '';
  if (hasDamage) {
    damage.push({
      damageTypeKey: d.damageType,
      dice: d.dice.trim(),
      ...(d.level === 0 ? { perTier: d.more.trim() } : { perSlotLevel: d.more.trim() }),
    });
    for (const o of d.otherDamage) {
      damage.push({ ...o });
    }
  }
  const body: SpellInit = {
    namePt: d.name.trim(),
    level: d.level,
    schoolKey: d.schoolKey,
    castingTime: {
      unit: unitOf,
      amount: oneShot ? 1 : d.timeAmount,
      ...(d.time === 'reaction' && d.trigger.trim() ? { triggerPt: d.trigger.trim() } : {}),
    },
    range: {
      kind: RANGE_TO_KIND[d.range],
      ...(d.range === 'ranged' ? { distanceFt: rangeFeet(d.rangeM) } : {}),
    },
    duration:
      d.duration === 'instant'
        ? { kind: SpellDurationKind.INSTANTANEOUS }
        : d.duration === 'until_dispelled'
          ? { kind: SpellDurationKind.UNTIL_DISPELLED }
          : { kind: SpellDurationKind.TIMED, amount: d.durationAmount, unit: DURATION_UNIT[d.durationUnit], upTo: d.upTo || d.concentration },
    components: {
      verbal: d.verbal,
      somatic: d.somatic,
      material: d.material,
      ...(d.material && d.materialPt.trim() ? { materialPt: d.materialPt.trim() } : {}),
    },
    concentration: d.concentration,
    ritual: d.ritual,
    classKeys: [...d.classKeys],
    descPt: paragraphs(d.text),
    higherLevelPt: paragraphs(d.higher),
    target,
    damage,
  };
  if (d.mechanic === 'attack') {
    body.attack = d.attack;
  } else if (d.mechanic === 'save') {
    body.save = { ability: d.saveAbility, onSuccess: d.saveOnSuccess === 'none' ? SpellSaveSuccess.NONE : SpellSaveSuccess.HALF };
  } else if (d.mechanic === 'heal') {
    body.heal = {
      dice: d.healDice.trim(),
      ...(d.healMore.trim() ? { perSlotLevel: d.healMore.trim() } : {}),
      addsModifier: d.healModifier,
    };
  }
  return body;
}

const TARGET_KIND: Record<TargetChoice, TableSpellTargetKind> = {
  creature: TableSpellTargetKind.CREATURE,
  creatures: TableSpellTargetKind.CREATURES,
  area: TableSpellTargetKind.AREA,
  self: TableSpellTargetKind.SELF,
};

/** Choosing "Só quem conjura" makes the range "Pessoal" (the server asks for it); leaving it gives the distance back. */
export function withTarget(d: SpellDraft, target: TargetChoice): SpellDraft {
  const next = { ...d, target };
  if (target === 'self') {
    next.range = 'self';
  } else if (d.target === 'self' && d.range === 'self' && target !== 'area') {
    // An area may come out of the caster (a cone from "Pessoal"): it keeps the range; picking creatures gives the distance back.
    next.range = 'ranged';
  }
  return next;
}

/** The target as the preview says it: "Uma criatura", "3 criaturas", "Cone de 4,5 m", "Só quem conjura". */
export function targetText(d: SpellDraft): string {
  switch (d.target) {
    case 'creature':
      return d.perSlot > 0 ? `Uma criatura, mais ${d.perSlot} por nível de espaço` : 'Uma criatura';
    case 'creatures':
      return d.perSlot > 0 ? `${d.count} criaturas, mais ${d.perSlot} por nível de espaço` : `${d.count} criaturas`;
    case 'self':
      return 'Só quem conjura';
    case 'area': {
      const ft = rangeFeet(d.sizeM);
      const shape = SHAPE_OPTIONS.find((s) => s.value === d.shape)?.label ?? '';
      return ft > 0 ? `${shape} de ${formatMeters(feetToMeters(ft))}` : shape;
    }
  }
}

/** One row of "Como os jogadores veem". */
export interface PreviewRow {
  readonly label: string;
  readonly value: string;
}

export type NameOf = (key: string) => string;

/** The damage type as the server names it ("fogo"); the key's last word when the catalog does not know it. */
const keyName: NameOf = (key) => key.replace(/^[a-z-]+:/, '');

/** "2d8 necrótico, +1d8 por nível acima do 1º". */
export function damageText(d: SpellDraft, nameOf: NameOf = keyName): string {
  if (!d.dice.trim()) {
    return '';
  }
  const type = nameOf(d.damageType);
  const more = d.more.trim() ? (d.level === 0 ? `, +${d.more.trim().replace(/^\+/, '')} por degrau do truque` : `, +${d.more.trim().replace(/^\+/, '')} por nível acima do ${d.level}º`) : '';
  return `${d.dice.trim()}${type ? ` ${type}` : ''}${more}`;
}

/** What a player reads of the spell, written as the app writes an SRD spell (the shared formatters of the "?"). */
export function previewRows(d: SpellDraft, nameOf: NameOf = keyName): PreviewRow[] {
  const rows: PreviewRow[] = [];
  const time = formatCastingTime({ amount: d.time === 'minute' || d.time === 'hour' ? d.timeAmount : 1, unit: d.time, trigger: '', raw: '' });
  rows.push({ label: 'Tempo', value: time.text || '—' });
  const range = formatRange({
    kind: d.range,
    distanceFt: d.range === 'ranged' ? rangeFeet(d.rangeM) : 0,
    raw: '',
  } satisfies SpellDetailsVm['range']);
  rows.push({ label: 'Alcance', value: range.text || '—' });
  rows.push({ label: 'Alvo', value: targetText(d) });
  const comps = formatComponents({ verbal: d.verbal, somatic: d.somatic, material: d.material, materialText: '' });
  rows.push({ label: 'Componentes', value: d.material && d.materialPt.trim() ? `${comps.text} (${d.materialPt.trim()})` : comps.text });
  const duration = formatDuration({
    kind: d.duration === 'instant' ? 'instantaneous' : d.duration === 'timed' ? 'timed' : 'until_dispelled',
    amount: d.durationAmount,
    unit: d.durationUnit,
    upTo: d.upTo,
    concentration: d.concentration,
    raw: '',
  });
  rows.push({ label: 'Duração', value: duration.text || '—' });
  if (d.mechanic === 'attack') {
    rows.push({ label: 'Ataque', value: d.attack === 'melee' ? 'Ataque de magia corpo a corpo' : 'Ataque de magia à distância' });
  } else if (d.mechanic === 'save') {
    const ab = nameOf(`ability:${d.saveAbility}`);
    rows.push({ label: 'Teste de resistência', value: `${ab}, ${d.saveOnSuccess === 'half' ? 'metade do dano ao passar' : 'nada ao passar'}` });
  }
  if (d.mechanic === 'attack' || d.mechanic === 'save') {
    const dmg = damageText(d, nameOf);
    if (dmg) {
      rows.push({ label: 'Dano', value: dmg });
    }
  }
  if (d.mechanic === 'heal' && d.healDice.trim()) {
    const more = d.healMore.trim() ? `, +${d.healMore.trim().replace(/^\+/, '')} por nível de espaço` : '';
    rows.push({ label: 'Cura', value: `${d.healDice.trim()}${d.healModifier ? ' + modificador' : ''}${more}` });
  }
  return rows;
}

/** The paths of the inputs the spell form draws right now: where a refusal can land. Anything else goes to its nearest input
 * above, or to the top. The mechanic, the target and the range show their own fields only. */
export function spellFieldPaths(d: SpellDraft): Set<string> {
  const p = 'table_spell';
  const set = new Set<string>([
    `${p}.name_pt`, `${p}.level`, `${p}.school_key`, `${p}.casting_time.unit`, `${p}.range.kind`, `${p}.duration.kind`,
    `${p}.components.verbal`, `${p}.components.somatic`, `${p}.components.material`,
    `${p}.concentration`, `${p}.ritual`, `${p}.class_keys`, `${p}.desc_pt`, `${p}.higher_level_pt`, `${p}.target.kind`,
  ]);
  const add = (...paths: string[]) => paths.forEach((x) => set.add(`${p}.${x}`));
  if (d.time === 'minute' || d.time === 'hour') add('casting_time.amount');
  if (d.time === 'reaction') add('casting_time.trigger_pt');
  if (d.range === 'ranged') add('range.distance_ft');
  if (d.duration === 'timed') add('duration.amount', 'duration.unit', 'duration.up_to');
  if (d.material) add('components.material_pt');
  if (d.target === 'creatures') add('target.count', 'target.per_slot_level');
  if (d.target === 'creature') add('target.per_slot_level');
  if (d.target === 'area') add('target.shape', 'target.size_ft');
  if (d.mechanic === 'attack') add('attack');
  if (d.mechanic === 'save') add('save.ability', 'save.on_success');
  if (d.mechanic === 'attack' || d.mechanic === 'save') {
    add('damage[0].dice', 'damage[0].damage_type_key', d.level === 0 ? 'damage[0].per_tier' : 'damage[0].per_slot_level');
  }
  if (d.mechanic === 'heal') add('heal.dice', 'heal.per_slot_level', 'heal.adds_modifier');
  return set;
}
