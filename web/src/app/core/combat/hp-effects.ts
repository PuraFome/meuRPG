import {
  SpellEffectGain,
  SpellEffectKind,
  SpellEffectOutcome,
  SpellEffectReason,
} from '../../../gen/meurpg/play/v1/combat_pb';
import type { DiceRoll } from '../../../gen/meurpg/play/v1/combat_pb';
import { rollFormula } from './combat-dice';
import { type SpellDetails, SpellHitPointEffectKind } from '../../../gen/meurpg/rules/v1/rules_pb';
import { conditionName } from './conditions';

/**
 * The spells that read hit points (MR-014, E8-03) as the screens say them: Sono,
 * Leque Cromático, the two Palavras de Poder, Estabilizar and Cura
 * Completa. The server applies them and sends what happened to each target
 * (`SpellEffectResult`); what is written here is the Portuguese. Which spells
 * these are, and what a pool rolls, come from the spell's own details
 * (`SpellDetails.hit_point_effect`): the browser keeps no copy of the rules.
 *
 * What each one may see is the server's decision (RN-20): a player has no hit
 * points, no pool order and no reason, and only the master gets them. The
 * functions below write what is there and never ask who is looking.
 */

export type HpSpellKind = 'pool' | 'threshold' | 'zero' | 'heal';

const KIND_FROM_GEN: Partial<Record<SpellHitPointEffectKind, HpSpellKind>> = {
  [SpellHitPointEffectKind.POOL]: 'pool',
  [SpellHitPointEffectKind.THRESHOLD]: 'threshold',
  [SpellHitPointEffectKind.ZERO_HP]: 'zero',
  [SpellHitPointEffectKind.FLAT_HEAL]: 'heal',
  // Vitalidade Falsa rolls a die before the cast, as a pool does; Ajuda only names who it touches.
  [SpellHitPointEffectKind.TEMP_HP]: 'pool',
  [SpellHitPointEffectKind.MAX_HP]: 'heal',
};

/** What kind of hit-point spell this is, from the rule `GetSpellDetails` sends, or `null` for any other spell. */
export function hpSpellKind(details: SpellDetails | null): HpSpellKind | null {
  return (
    KIND_FROM_GEN[details?.hitPointEffect?.kind ?? SpellHitPointEffectKind.UNSPECIFIED] ?? null
  );
}

/** What a spell that changes hit points gave one target, in words: "recupera 7 PV" for a heal, "ganha 7 PV
 * temporários" for Vitalidade Falsa, and for Ajuda what that target got (`gain`): "ganha 5 PV máximos" for
 * an NPC or a creature, "ganha 5 PV temporários" for a character standing, "volta com 5 PV" for one at 0. */
export function gainWords(kind: SpellEffectKind, amount: number, gain?: SpellEffectGain): string {
  switch (kind) {
    case SpellEffectKind.TEMP_HP:
      return `ganha ${amount} PV temporários`;
    case SpellEffectKind.MAX_HP:
      switch (gain) {
        case SpellEffectGain.TEMPORARY:
          return `ganha ${amount} PV temporários`;
        case SpellEffectGain.CURRENT:
          return `volta com ${amount} PV`;
        default:
          return `ganha ${amount} PV máximos`;
      }
    default:
      return `recupera ${amount} PV`;
  }
}

/** The line under a target of the cast sheet: the amount and what it became ("PV máximo +5"). */
export function gainLine(kind: SpellEffectKind, amount: number, gain?: SpellEffectGain): string {
  switch (kind) {
    case SpellEffectKind.TEMP_HP:
      return `${amount} PV temporários`;
    case SpellEffectKind.MAX_HP:
      switch (gain) {
        case SpellEffectGain.TEMPORARY:
          return `${amount} PV temporários`;
        case SpellEffectGain.CURRENT:
          return `volta com ${amount} PV`;
        default:
          return `PV máximo +${amount}`;
      }
    default:
      return `${amount} PV recuperados`;
  }
}

/** The pool a spell rolls when cast with a slot of `slotLevel`: Sono is 5d8 at
 * the 1st circle and 7d8 at the 2nd. `null` when the spell has no pool. */
export function poolDice(
  details: SpellDetails | null,
  slotLevel: number,
): { count: number; sides: number } | null {
  const fx = details?.hitPointEffect;
  if (
    !fx ||
    (fx.kind !== SpellHitPointEffectKind.POOL && fx.kind !== SpellHitPointEffectKind.TEMP_HP)
  ) {
    return null;
  }
  const own = details?.spell?.level ?? 1;
  return {
    count: fx.poolDiceCount + fx.poolDicePerLevel * Math.max(0, slotLevel - own),
    sides: fx.poolDiceSides,
  };
}

// ---- what happened to one target, in words ----

/** One target's outcome: the verb as the log writes it ("adormece") and as the
 * sheet does ("Adormeceu"), and the icon of the pill. */
export interface EffectWords {
  /** "adormece", "fica atordoado", "está estável": for a sentence. */
  readonly present: string;
  /** "Adormeceu", "Ficou atordoado", "Ficou estável": the pill. */
  readonly past: string;
  readonly icon: string;
  readonly affected: boolean;
}

const CONDITION_WORDS: Readonly<Record<string, { present: string; past: string; icon: string }>> = {
  'condition:unconscious': { present: 'adormece', past: 'Adormeceu', icon: 'bedtime' },
  'condition:blinded': { present: 'fica cego', past: 'Ficou cego', icon: 'visibility_off' },
  'condition:stunned': { present: 'fica atordoado', past: 'Ficou atordoado', icon: 'bolt' },
};

/** Whether the label is a feminine name ("Brisa"), for "afetada". */
function feminine(label: string): boolean {
  const first = label.trim().split(/\s+/)[0].toLowerCase();
  return /a$/.test(first);
}

export function notAffected(label: string, perfect = true): string {
  const word = feminine(label) ? 'afetada' : 'afetado';
  return perfect ? `não foi ${word}` : `não é ${word}`;
}

/** What the outcome of one target says. `condition` is the cast's
 * `effect_condition_key` ("" for a kill, a stabilise and a heal). */
export function effectWords(
  kind: SpellEffectKind,
  condition: string,
  outcome: SpellEffectOutcome,
  label = '',
  gain?: SpellEffectGain,
): EffectWords {
  if (outcome !== SpellEffectOutcome.AFFECTED) {
    return {
      present: notAffected(label, false),
      past: label && feminine(label) ? 'Não foi afetada' : 'Não foi afetado',
      icon: 'block',
      affected: false,
    };
  }
  const known = CONDITION_WORDS[condition];
  if (known) {
    return { ...known, affected: true };
  }
  if (condition) {
    const name = conditionName(condition).toLowerCase();
    return { present: `fica ${name}`, past: `Ficou ${name}`, icon: 'label', affected: true };
  }
  switch (kind) {
    case SpellEffectKind.ZERO_HP:
      return { present: 'está estável', past: 'Ficou estável', icon: 'favorite', affected: true };
    case SpellEffectKind.FLAT_HEAL: {
      const f = feminine(label);
      return {
        present: f ? 'é curada' : 'é curado',
        past: f ? 'Foi curada' : 'Foi curado',
        icon: 'healing',
        affected: true,
      };
    }
    case SpellEffectKind.TEMP_HP:
      return { present: 'ganha PV', past: 'Ganhou PV', icon: 'shield', affected: true };
    case SpellEffectKind.MAX_HP:
      // Ajuda: what the target got depends on who it is (maximum, temporary, or the hit points of one who got up).
      switch (gain) {
        case SpellEffectGain.TEMPORARY:
          return {
            present: 'ganha PV temporários',
            past: 'Ganhou PV temporários',
            icon: 'shield',
            affected: true,
          };
        case SpellEffectGain.CURRENT:
          return {
            present: 'volta com PV',
            past: 'Voltou com PV',
            icon: 'favorite',
            affected: true,
          };
        default:
          return {
            present: 'ganha PV máximos',
            past: 'Ganhou PV máximos',
            icon: 'shield',
            affected: true,
          };
      }
    default:
      // A threshold with no condition is Palavra de Poder Matar.
      return { present: 'morre', past: 'Morreu', icon: 'heart_broken', affected: true };
  }
}

/** Why the master is told a target was not affected, in words (only he gets the reason). */
export function reasonWords(
  reason: SpellEffectReason,
  hitPoints: number | undefined,
  left: number | undefined,
  threshold: number | undefined,
): string {
  switch (reason) {
    case SpellEffectReason.ABOVE_POOL:
      return left === undefined || hitPoints === undefined
        ? 'mais PV do que sobrou do total'
        : `${hitPoints} é mais que ${left} restantes`;
    case SpellEffectReason.ABOVE_LIMIT:
      return threshold === undefined
        ? 'acima do limite'
        : `${hitPoints ?? ''} PV, acima do limite de ${threshold}`.trim();
    case SpellEffectReason.SKIPPED:
      return 'já estava inconsciente ou a 0 PV';
    case SpellEffectReason.NOT_AT_ZERO:
      return 'não estava a 0 PV';
    default:
      return '';
  }
}

/** The caster's pool: `5d8 (2, 4, 1, 5, 3) = 15`, or with a physical die, where only the sum was
 * typed, `5d8 = 15 · dado físico`. */
export function poolRollText(roll: DiceRoll): string {
  if (roll.physical || roll.faces.length === 0) {
    return `${roll.diceCount}d${roll.diceSides} = ${roll.total} · dado físico`;
  }
  return rollFormula(roll);
}
