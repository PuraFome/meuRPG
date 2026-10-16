import {
  Ability,
  ActionEconomy,
  type SpellDetails,
  SpellDamageChoice,
  SpellRangeKind,
  SpellSaveSuccess,
} from '../../../gen/meurpg/rules/v1/rules_pb';
import { metersText } from '../units';
import { joinDots, tight } from '../format/text';
import { poolDice } from './hp-effects';

/** What a spell does, which decides what the sheet asks and what it shows. */
export type SpellKind = 'attack' | 'save' | 'darts' | 'heal' | 'plain' | 'pool' | 'hp';

// ---- what the sheet says ----

const ABILITY_PT: Partial<Record<Ability, string>> = {
  [Ability.STRENGTH]: 'Força',
  [Ability.DEXTERITY]: 'Destreza',
  [Ability.CONSTITUTION]: 'Constituição',
  [Ability.INTELLIGENCE]: 'Inteligência',
  [Ability.WISDOM]: 'Sabedoria',
  [Ability.CHARISMA]: 'Carisma',
};

/** The ability of a spell's saving throw, in Portuguese ("Constituição"); `''` without a save. */
export function saveAbility(details: SpellDetails | null): string {
  return ABILITY_PT[details?.save?.ability ?? Ability.UNSPECIFIED] ?? '';
}

/**
 * The dice a spell makes at a slot level ("8d6"), from its details; `''` when there are none.
 * A cantrip grows with the caster's level, which the details do not know: the server sends the
 * caster's dice with the attack (`Attack.spellDice`), and without them a cantrip shows no dice
 * rather than the first row of its table.
 */
export function damageDice(
  details: SpellDetails | null,
  slotLevel: number,
  cantripDice = '',
): string {
  const d = details?.damage[0];
  if (!d) {
    return '';
  }
  const own = details?.spell?.level ?? 0;
  if (own === 0 && Object.keys(d.byCharacterLevel).length > 0) {
    return cantripDice;
  }
  return d.bySlotLevel[slotLevel] ?? d.bySlotLevel[own] ?? cantripDice;
}

/**
 * What a spell deals, in words, with each damage type's dice: "5d6 de fogo e 4d6 de radiante". For a spell
 * that scales by choice (Coluna de Chamas) every type is dealt and the dice a higher slot adds go to the
 * picked type (`pick`, a damage type key; the first when empty), the others staying at the spell's own
 * circle; for one that deals only the picked type (Guardiões Espirituais) only that one is said.
 */
export function damageText(
  details: SpellDetails | null,
  slotLevel: number,
  cantripDice = '',
  pick = '',
): string {
  const all = details?.damage ?? [];
  if (all.length === 0) {
    return '';
  }
  const own = details?.spell?.level ?? 0;
  const choice = details?.damageChoice ?? SpellDamageChoice.UNSPECIFIED;
  const picked = all.find((d) => d.damageTypeKey === pick) ?? all[0];
  const shown = choice === SpellDamageChoice.ALTERNATIVE ? [picked] : all;
  return shown
    .flatMap((d) => {
      const level = choice === SpellDamageChoice.SCALE && d !== picked ? own : slotLevel;
      const dice = damageDice(details ? { ...details, damage: [d] } : null, level, cantripDice);
      return dice ? [`${dice}${d.damageTypePt ? ` de ${d.damageTypePt}` : ''}`] : [];
    })
    .join(' e ');
}

/** Magic Missile's darts when the cast does not say how many: three, at the spell's own circle. */
const DEFAULT_DARTS = 3;

/** "Ação", "Ação bônus" or "Reação". */
function economyWord(economy: ActionEconomy): string {
  switch (economy) {
    case ActionEconomy.BONUS_ACTION:
      return 'Ação bônus';
    case ActionEconomy.REACTION:
      return 'Reação';
    default:
      return 'Ação';
  }
}

/** "alcance 36 m", "toque" or "pessoal"; empty for a range with none of them. */
function rangeWord(details: SpellDetails | null): string {
  const range = details?.range;
  if (range?.kind === SpellRangeKind.RANGED && range.distanceFt > 0) {
    return `alcance ${metersText(range.distanceFt)}`;
  }
  if (range?.kind === SpellRangeKind.TOUCH) {
    return 'toque';
  }
  return range?.kind === SpellRangeKind.SELF ? 'pessoal' : '';
}

/** What the sheet asks for besides the range, by the kind of spell: the dice it rolls, the save, the heal, the pool. */
interface KindPart {
  readonly details: SpellDetails | null;
  readonly slotLevel: number;
  readonly darts: number;
  readonly type: string;
  readonly dice: string;
}

const KIND_PARTS: Partial<Record<SpellKind, (p: KindPart) => string>> = {
  darts: (p) =>
    `${p.darts || DEFAULT_DARTS} dardos de 1d4 + 1 de ${p.type || 'energia'}, sempre acertam`,
  attack: (p) => `ataque de magia${p.dice ? ` · ${p.dice}` : ''}`,
  save: (p) => {
    const ability = ABILITY_PT[p.details?.save?.ability ?? Ability.UNSPECIFIED];
    const half = p.details?.save?.onSuccess === SpellSaveSuccess.HALF ? ', metade se resistir' : '';
    return `resistência${ability ? ` de ${ability}` : ''}${p.dice ? ` · ${p.dice}${half}` : ''}`;
  },
  heal: () => 'cura',
  pool: (p) => {
    const pool = poolDice(p.details, p.slotLevel);
    return pool ? `${pool.count}d${pool.sides} de pontos de vida` : 'pontos de vida';
  },
};

/** "Ação · alcance 36 m · 3 dardos de 1d4 + 1 de energia, sempre acertam". */
export function castSubtitle(
  economy: ActionEconomy,
  kind: SpellKind,
  details: SpellDetails | null,
  slotLevel: number,
  darts: number,
  cantripDice = '',
  damageType = '',
  shape = '',
): string {
  const range = rangeWord(details);
  const kindPart = KIND_PARTS[kind]?.({
    details,
    slotLevel,
    darts,
    type: details?.damage[0]?.damageTypePt ?? '',
    dice: damageText(details, slotLevel, cantripDice, damageType),
  });
  // An area placed on the map says its form and size ("esfera de 6 m de raio").
  return tight(joinDots([economyWord(economy), range, shape, kindPart ?? ''].filter(Boolean)));
}
