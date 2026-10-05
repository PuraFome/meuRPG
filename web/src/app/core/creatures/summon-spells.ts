/**
 * The three spells that summon creatures outside a combat (MR-037), as the
 * cast sheet needs them: which kinds of creature each one offers and how the
 * count follows the slot.
 *
 * This is the one place the web keeps a copy of rules data: the server has no
 * read that offers the summon choices yet (`rules.SummonOptions` is internal),
 * so this mirrors the `summon` entries of `effects/spells.json`. It only decides
 * what the sheet OFFERS; the server checks every choice (`CastSummon` answers
 * `SUMMON_CHOICE_INVALID` for anything else) and nothing else here is a rule.
 * When a read for the options exists, this file goes away.
 */

export type SummonKind = 'familiar' | 'undead' | 'beasts';

export interface SummonSpellDef {
  readonly key: string;
  readonly name: string;
  /** The spell's own circle. */
  readonly level: number;
  /** How long it takes: "1 hora", "1 minuto", "1 ação". */
  readonly time: string;
  readonly kind: SummonKind;
  /** The creatures it offers (`familiar`, `undead`). */
  readonly forms: readonly string[];
  /** Forms a feature adds (the Pact of the Chain's familiars). */
  readonly extraForms: readonly string[];
  readonly extraFeature: string;
}

export const FIND_FAMILIAR = 'spell:find-familiar';
export const ANIMATE_DEAD = 'spell:animate-dead';
export const CONJURE_ANIMALS = 'spell:conjure-animals';
export const PACT_OF_THE_CHAIN = 'feature:pact-of-the-chain';

export const SUMMON_SPELLS: readonly SummonSpellDef[] = [
  {
    key: FIND_FAMILIAR,
    name: 'Encontrar Familiar',
    level: 1,
    time: '1 hora',
    kind: 'familiar',
    forms: [
      'bat', 'cat', 'crab', 'frog', 'hawk', 'lizard', 'octopus', 'owl', 'poisonous-snake', 'quipper', 'rat', 'raven', 'sea-horse', 'spider', 'weasel',
    ].map((k) => `monster:${k}`),
    extraForms: ['imp', 'pseudodragon', 'quasit', 'sprite'].map((k) => `monster:${k}`),
    extraFeature: PACT_OF_THE_CHAIN,
  },
  {
    key: ANIMATE_DEAD,
    name: 'Animar os Mortos',
    level: 3,
    time: '1 minuto',
    kind: 'undead',
    forms: ['monster:skeleton', 'monster:zombie'],
    extraForms: [],
    extraFeature: '',
  },
  {
    key: CONJURE_ANIMALS,
    name: 'Conjurar Animais',
    level: 3,
    time: '1 ação',
    kind: 'beasts',
    forms: [],
    extraForms: [],
    extraFeature: '',
  },
];

export function summonSpell(key: string): SummonSpellDef | undefined {
  return SUMMON_SPELLS.find((s) => s.key === key);
}

/** One of Conjurar Animais' options: how many creatures, up to which challenge rating. */
export interface BeastOption {
  readonly index: number;
  readonly count: number;
  readonly maxCr: string;
}

const BEAST_OPTIONS: readonly { count: number; maxCr: string }[] = [
  { count: 1, maxCr: '2' },
  { count: 2, maxCr: '1' },
  { count: 4, maxCr: '1/2' },
  { count: 8, maxCr: '1/4' },
];

/** What a slot's circle multiplies Conjurar Animais' counts by (5th: x2, 7th: x3, 9th: x4). */
function beastMultiplier(circle: number): number {
  return circle >= 9 ? 4 : circle >= 7 ? 3 : circle >= 5 ? 2 : 1;
}

export function beastOptions(circle: number): readonly BeastOption[] {
  const m = beastMultiplier(circle);
  return BEAST_OPTIONS.map((o, index) => ({ index, count: o.count * m, maxCr: o.maxCr }));
}

/** How many undead Animar os Mortos raises with a slot: 1, and 2 more for each circle above the 3rd. */
export function undeadCount(circle: number): number {
  return 1 + 2 * Math.max(0, circle - 3);
}
