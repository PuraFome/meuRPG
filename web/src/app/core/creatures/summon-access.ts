import type { DerivedSheet } from '../../../gen/meurpg/rules/v1/rules_pb';
import { FIND_FAMILIAR, PACT_OF_THE_CHAIN, SUMMON_SPELLS } from './summon-spells';

/** How a character can cast one summoning spell from the sheet's "Criaturas". */
export interface SummonCastVm {
  readonly key: string;
  readonly name: string;
  readonly level: number;
  /** As a ritual: no slot (Encontrar Familiar). */
  readonly ritual: boolean;
  /** With a slot: the spell is ready (prepared, or one the class knows). */
  readonly slot: boolean;
  /** The Pact of the Chain: its familiars (Diabrete, Pseudodragão, Quasit, Sprite) are on the list too. */
  readonly chain: boolean;
}

/** What the sheet may offer in "Criaturas": the spells it can cast, and whether it is a druid with Wild Shape. */
export interface CreatureAccessVm {
  readonly casts: readonly SummonCastVm[];
  /** The panel exists for a druid (Forma Selvagem: the beast in combat, slice 9.17), even before a first creature. */
  readonly wildShape: boolean;
}

export const NO_CREATURE_ACCESS: CreatureAccessVm = { casts: [], wildShape: false };

/**
 * From the derived sheet, which summoning spells the character can cast and
 * how. The sheet only OFFERS what it can see (the spell on the list, the Pact
 * of the Chain); the server checks the cast (`CheckSummon`) and refuses what
 * the rules do not allow, so a wrong guess here is a message, never a wrong
 * creature. A wizard casts Encontrar Familiar as a ritual from the book; the
 * other casters, from their prepared list; a warlock with the Pact of the Chain
 * casts it as a ritual without the spell.
 */
export function creatureAccess(derived: DerivedSheet): CreatureAccessVm {
  const wizard = derived.classes.some((c) => c.classKey === 'class:wizard');
  const chain = derived.features.some((f) => f.key === PACT_OF_THE_CHAIN);
  const casts: SummonCastVm[] = [];
  for (const def of SUMMON_SPELLS) {
    const own = derived.spells.find((s) => s.spell?.key === def.key);
    const ritualSpell = own?.spell?.ritual ?? false;
    const chainFamiliar = def.key === FIND_FAMILIAR && chain;
    const ritual = chainFamiliar || (ritualSpell && !!own && (wizard || own.prepared));
    const slot = !!own && own.prepared;
    if (ritual || slot) {
      casts.push({ key: def.key, name: def.name, level: def.level, ritual, slot, chain: chainFamiliar });
    }
  }
  return { casts, wildShape: derived.features.some((f) => f.key.startsWith('feature:wild-shape')) };
}
