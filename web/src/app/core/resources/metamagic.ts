import type { MetamagicOption } from '../../../gen/meurpg/rules/v1/rules_pb';
import { type Pool, pointsText } from './pools';

/** Empowered Spell may join another option on the same casting (SRD 5.1, Metamagic); every other option stands alone. */
export const EMPOWERED_KEY = 'feature:metamagic-empowered-spell';
export const TWINNED_KEY = 'feature:metamagic-twinned-spell';
export const CAREFUL_KEY = 'feature:metamagic-careful-spell';
export const HEIGHTENED_KEY = 'feature:metamagic-heightened-spell';

/** A Metamagic option as the cast sheet lists it. */
export interface MetamagicRow {
  readonly key: string;
  readonly name: string;
  /** "1 ponto", "3 pontos". */
  readonly cost: string;
  readonly summary: string;
  readonly checked: boolean;
  /** Why it cannot be marked (the spell does not take it, or another option is marked); empty when it can. */
  readonly blocked: string;
}

/** What the player filled in for the options that need a creature. */
export interface MetamagicPicks {
  /** Twinned Spell: the second target. */
  readonly twinned: string;
  /** Careful Spell: the creatures that pass on their own. */
  readonly careful: readonly string[];
  /** Heightened Spell: the target with disadvantage. */
  readonly heightened: string;
}

export const NO_PICKS: MetamagicPicks = { twinned: '', careful: [], heightened: '' };

/** The words under an option that cannot be marked because another one is. */
const ONLY_ONE = 'Só uma opção por magia, a não ser a Magia Potencializada, que soma com outra.';

/** Whether `chosen` is a choice the rules take: at most one option, or Empowered Spell with one other (the same check
 * the server makes, `rules.CheckMetamagicChoice`; the server has the last word). */
export function validChoice(chosen: readonly string[]): boolean {
  if (chosen.length <= 1) {
    return true;
  }
  return chosen.length === 2 && chosen.includes(EMPOWERED_KEY);
}

/** The options of a spell with their state: the ones the spell does not take stay grey with the server's reason, and
 * once one is marked the ones that cannot be added to it do too. */
export function metamagicRows(
  options: readonly MetamagicOption[],
  chosen: readonly string[],
): MetamagicRow[] {
  return options.map((o) => {
    const checked = chosen.includes(o.key);
    let blocked = o.allowed ? '' : o.disabledReasonPt;
    if (!blocked && !checked && !validChoice([...chosen, o.key])) {
      blocked = ONLY_ONE;
    }
    return {
      key: o.key,
      name: o.namePt,
      cost: pointsText(o.cost),
      summary: o.summaryPt,
      checked,
      blocked,
    };
  });
}

/** The key list with `key` marked or unmarked. A marked option that cannot join the others is ignored. */
export function toggledOption(chosen: readonly string[], key: string): string[] {
  if (chosen.includes(key)) {
    return chosen.filter((k) => k !== key);
  }
  return validChoice([...chosen, key]) ? [...chosen, key] : [...chosen];
}

/** The sorcery points the chosen options cost together. */
export function metamagicCost(
  options: readonly MetamagicOption[],
  chosen: readonly string[],
): number {
  return options.filter((o) => chosen.includes(o.key)).reduce((n, o) => n + o.cost, 0);
}

/** "Você escolheu: Magia Duplicada (1 ponto)." */
export function chosenLine(options: readonly MetamagicOption[], chosen: readonly string[]): string {
  const picked = options.filter((o) => chosen.includes(o.key));
  if (picked.length === 0) {
    return '';
  }
  return `Você escolheu: ${picked.map((o) => o.namePt).join(' e ')} (${pointsText(metamagicCost(options, chosen))}).`;
}

/** "Conjurar e gastar 1 ponto"; plain "Conjurar X" without a choice. */
export function castLabel(name: string, cost: number): string {
  return cost > 0 ? `Conjurar e gastar ${pointsText(cost)}` : `Conjurar ${name}`;
}

/** What is missing before a Metamagic cast can go, in words; empty when it can: the points, and the creatures the
 * chosen options ask for. */
export function metamagicMissing(
  options: readonly MetamagicOption[],
  chosen: readonly string[],
  picks: MetamagicPicks,
  points: Pool | null,
): string {
  if (chosen.length === 0) {
    return '';
  }
  const cost = metamagicCost(options, chosen);
  if (points && cost > points.left) {
    return `Faltam pontos de feitiçaria: custa ${pointsText(cost)} e você tem ${points.left}.`;
  }
  if (chosen.includes(TWINNED_KEY) && !picks.twinned) {
    return 'Escolha o segundo alvo da Magia Duplicada.';
  }
  if (chosen.includes(CAREFUL_KEY) && picks.careful.length === 0) {
    return 'Escolha as criaturas da Magia Cuidadosa.';
  }
  if (chosen.includes(HEIGHTENED_KEY) && !picks.heightened) {
    return 'Escolha o alvo da Magia Aumentada.';
  }
  return '';
}

/** The `CastSpellRequest.metamagic` entries of the choice. */
export function metamagicChoices(
  chosen: readonly string[],
  picks: MetamagicPicks,
): { key: string; targetIds: string[]; carefulIds: string[]; heightenedId: string }[] {
  return chosen.map((key) => ({
    key,
    targetIds: key === TWINNED_KEY && picks.twinned ? [picks.twinned] : [],
    carefulIds: key === CAREFUL_KEY ? [...picks.careful] : [],
    heightenedId: key === HEIGHTENED_KEY ? picks.heightened : '',
  }));
}

/** "Magia Duplicada · gastou 1 ponto de feitiçaria", for the result and the log. */
export function metamagicSpentLine(
  names: readonly string[],
  spent: number,
  points: Pool | null,
): string {
  const left = points ? ` (restam ${Math.max(0, points.left - spent)} de ${points.total})` : '';
  return `${names.join(' e ')} · gastou ${pointsText(spent)} de feitiçaria${left}`;
}

/** The Portuguese name of each Metamagic option, for the log (the log carries only the keys). */
const METAMAGIC_NAMES: Readonly<Record<string, string>> = {
  'feature:metamagic-careful-spell': 'Magia Cuidadosa',
  'feature:metamagic-distant-spell': 'Magia Distante',
  'feature:metamagic-empowered-spell': 'Magia Potencializada',
  'feature:metamagic-extended-spell': 'Magia Estendida',
  'feature:metamagic-heightened-spell': 'Magia Aumentada',
  'feature:metamagic-quickened-spell': 'Magia Acelerada',
  'feature:metamagic-subtle-spell': 'Magia Sutil',
  'feature:metamagic-twinned-spell': 'Magia Duplicada',
};

/** "Magia Duplicada" for a Metamagic key; a key this app does not know reads "Metamagia". */
export function metamagicName(key: string): string {
  return METAMAGIC_NAMES[key] ?? 'Metamagia';
}
