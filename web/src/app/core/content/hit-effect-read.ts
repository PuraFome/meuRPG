import { draftToEffect } from './spell-effect-draft';
import { TableEffectAudience } from '../../../gen/meurpg/rules/v1/table_content_pb';
import { conditionName } from '../combat/conditions';
import { formatDuration } from '../../shared/spell-details/spell-details-format';
import { hitAvailable, type HitDraft, type HitLasts, type HitTriggerKey } from './hit-effect-draft';
import type { SpellDraft } from './spell-types';

/**
 * What a player reads of a spell's "Efeito no próximo golpe" (#577), from the stored `weapon_hit_damage` modifier (through the
 * spell's draft, so the editor's preview and the player's page are one text). Pure Portuguese. Only the weapon-hit modifier is
 * read: the effect's other modifiers and its conditions are not hit text. A spell that does not last has no such effect.
 */

export const HIT_SECTION_TITLE = 'Efeito no próximo golpe';
export const HIT_VISIBILITY_LABEL = 'Quem vê o efeito';
export const VISIBILITY_PUBLIC_PT = 'Todos na mesa';
export const VISIBILITY_OWNER_PT = 'Só quem conjura e você';

/** A piece of the save's sentence: the condition's name is the part a reader looks for. */
export interface HitSentencePart {
  readonly text: string;
  readonly strong?: boolean;
}

export interface ReadHit {
  readonly title: string;
  /** "Em quem conjura: o próximo golpe corpo a corpo, por até 1 minuto." */
  readonly where: string;
  /** "No próximo golpe com arma:", the lead of the editor's preview line. */
  readonly lead: string;
  /** "enquanto durar": each hit and the marked target go on while the spell lasts; the line of the player ends with it. */
  readonly whileLasts: boolean;
  /** The extra damage: "+2d8 frio". */
  readonly damage: string;
  /** What it adds per level and the most: "+1d8 por nível acima do 2º, até 5d8"; empty when it adds none. */
  readonly more: string;
  /** "O alvo faz um teste de resistência de Força; se falhar, fica Impedido até o fim do próximo turno dele."; empty with no test. */
  readonly save: readonly HitSentencePart[];
  /** Who sees the effect on the creature, as the server sends it. */
  readonly visibility: string;
}

const WHERE: Record<HitTriggerKey, string> = {
  next_hit: 'o próximo golpe com arma',
  next_melee_hit: 'o próximo golpe corpo a corpo',
  each_hit: 'cada golpe com arma',
  marked_target: 'cada golpe com arma no alvo marcado',
};

const LEAD: Record<HitTriggerKey, string> = {
  next_hit: 'No próximo golpe com arma:',
  next_melee_hit: 'No próximo golpe corpo a corpo:',
  each_hit: 'Em cada golpe com arma:',
  marked_target: 'Em cada golpe com arma no alvo marcado:',
};

/** The save's wire key ("con") by the catalog's key for the ability. */
const ABILITY_CATALOG: Readonly<Record<string, string>> = {
  str: 'ability:strength',
  dex: 'ability:dexterity',
  con: 'ability:constitution',
  int: 'ability:intelligence',
  wis: 'ability:wisdom',
  cha: 'ability:charisma',
};

/** How long the spell lasts, after "por": "até 1 minuto", "até 2 horas", "até ser dissipada" (a spell with no end). */
function lastsText(d: SpellDraft): string {
  if (d.duration !== 'timed') {
    return 'até ser dissipada';
  }
  const length = formatDuration({
    kind: 'timed',
    amount: d.durationAmount,
    unit: d.durationUnit,
    upTo: false,
    concentration: false,
    raw: '',
  }).text;
  return length ? `até ${length.replace(/^até /, '')}` : 'até ser dissipada';
}

export function damageChip(h: HitDraft, nameOf: (key: string) => string): string {
  const type = h.damageType ? nameOf(h.damageType) : 'do tipo da arma';
  return `+${h.dice}d${h.die} ${type}`;
}

export function moreChip(h: HitDraft, level: number): string {
  const perSlot = Number(h.perSlot);
  // A truque has no slot level to climb.
  if (level === 0 || !(perSlot > 0)) return '';
  const cap = Number(h.cap) > 0 ? `, até ${h.cap}d${h.die}` : '';
  return `+${perSlot}d${h.die} por nível acima do ${level}º${cap}`;
}

/** The conditions by their names ("Impedido"); a key this app does not know is left out. Exhaustion is had, not "fica". */
export function conditionPhrases(keys: readonly string[]): string[] {
  return keys
    .filter((k) => conditionName(k) !== '')
    .map((k) => (k === 'condition:exhaustion' ? `com ${conditionName(k)}` : conditionName(k)));
}

/** How long a condition a failed test gives lasts: "até o fim da magia" or "até o fim do próximo turno dele". */
export function hitUntil(lasts: HitLasts): string {
  return lasts === 'spell_end' ? 'até o fim da magia' : 'até o fim do próximo turno dele';
}

function saveSentence(h: HitDraft, nameOf: (key: string) => string): readonly HitSentencePart[] {
  if (h.saveAbility === '') return [];
  const ability = nameOf(ABILITY_CATALOG[h.saveAbility] ?? '');
  const test = `O alvo faz um teste de resistência${ability ? ` de ${ability}` : ''}`;
  const names = conditionPhrases(h.condition ? [h.condition, ...h.extraConditions] : []);
  if (names.length === 0) return [{ text: `${test}.` }];
  const until = hitUntil(h.conditionLasts);
  return [
    { text: `${test}; se falhar, fica ` },
    ...names.flatMap<HitSentencePart>((n, i) => [
      ...(i === 0 ? [] : [{ text: i === names.length - 1 ? ' e ' : ', ' }]),
      { text: n, strong: true },
    ]),
    { text: ` ${until}.` },
  ];
}

/** "Todos na mesa" or "Só quem conjura e você": the audience the server sends; unset is the owner's (the proto's own default). */
export function visibilityText(audience: TableEffectAudience | undefined): string {
  return audience === TableEffectAudience.PUBLIC ? VISIBILITY_PUBLIC_PT : VISIBILITY_OWNER_PT;
}

/** The hit effect of a spell as a player reads it, or `null` when the spell has none (or does not last, or has no trigger yet). */
export function readHit(d: SpellDraft, nameOf: (key: string) => string): ReadHit | null {
  const h = d.hit;
  // A trigger left unset says nothing yet: the form asks for it before the save.
  if (!h.enabled || !hitAvailable(d) || h.trigger === '') {
    return null;
  }
  return {
    title: HIT_SECTION_TITLE,
    where: `Em quem conjura: ${WHERE[h.trigger]}, por ${lastsText(d)}.`,
    lead: LEAD[h.trigger],
    whileLasts: h.trigger === 'each_hit' || h.trigger === 'marked_target',
    damage: damageChip(h, nameOf),
    more: moreChip(h, d.level),
    save: saveSentence(h, nameOf),
    visibility: visibilityText((draftToEffect(d.effect) ?? d.effect.base)?.visibility),
  };
}
