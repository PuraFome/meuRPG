import { joinDots } from '../format/text';
import { SQUARE_FT, formatMeters, feetToMeters } from '../units';
import { type CreatureAttackDraft } from './creature-draft';
import { rangeFeet } from './effect-draft';

/**
 * What the creature editor and the list say in words: the echo under a formula, the line of a collapsed attack and the support
 * line of a list row. Pure functions, so they are tested without a screen.
 */

type NameOf = (key: string) => string;

const ABILITIES: Readonly<Record<string, string>> = {
  str: 'Força',
  strength: 'Força',
  dex: 'Destreza',
  dexterity: 'Destreza',
  con: 'Constituição',
  constitution: 'Constituição',
  int: 'Inteligência',
  intelligence: 'Inteligência',
  wis: 'Sabedoria',
  wisdom: 'Sabedoria',
  cha: 'Carisma',
  charisma: 'Carisma',
};

const WORDS: readonly [RegExp, (...m: string[]) => string][] = [
  [/\blevel\(\s*\)/g, () => 'nível do dono'],
  [/\bprof\(\s*\)/g, () => 'bônus de proficiência'],
  [/\bslot\(\s*\)/g, () => 'círculo da magia'],
  [/\bspeedwalk\(\s*\)/g, () => 'deslocamento a pé'],
  [/\barmor\(\s*\)/g, () => 'categoria da armadura'],
  [/\bshield\(\s*\)/g, () => 'escudo'],
  [
    /\bmod\(\s*["']([a-z]+)["']\s*\)/g,
    (_, a) => (ABILITIES[a] ? `modificador de ${ABILITIES[a]}` : _),
  ],
  [/\bscore\(\s*["']([a-z]+)["']\s*\)/g, (_, a) => (ABILITIES[a] ? `valor de ${ABILITIES[a]}` : _)],
  [/\bclassLevel\(\s*["']([^"']+)["']\s*\)/g, (_, c) => `nível de ${c}`],
];

/** A formula in words, "4 + 4 × level()" as "4 + 4 × nível do dono": the live echo under a formula field. Empty for an empty formula. */
export function formulaWords(formula: string): string {
  const src = formula.trim();
  if (src === '') {
    return '';
  }
  let out = src;
  for (const [re, to] of WORDS) {
    out = out.replace(re, to);
  }
  return out
    .replace(/\s*[*×]\s*/g, ' × ')
    .replace(/\s*\/\s*/g, ' ÷ ')
    .replace(/\s*\+\s*/g, ' + ')
    .replace(/(?<=[\p{L}\d)])\s*-\s*/gu, ' − ')
    .replace(/\s+/g, ' ')
    .trim();
}

/** "= 4 + 4 × nível do dono · de 1 a 9.999", or nothing for an empty formula. */
export function formulaEcho(formula: string, range: string): string {
  const words = formulaWords(formula);
  return words === '' ? '' : `= ${words} · ${range}`;
}

/** The range the server clamps each formula to, in the words the echo ends with. */
export const RANGES = {
  hitPoints: 'de 1 a 9.999',
  armorClass: 'de 1 a 40',
  damageBonus: 'de −10 a 30',
  attackBonus: 'de −10 a 30',
} as const;

/** A distance in metres for a collapsed row: "1,5 m". */
function metres(ft: number): string {
  return formatMeters(feetToMeters(ft));
}

/** "Corpo a corpo, 1,5 m" or "À distância, 9 m e 18 m". */
export function attackShape(a: CreatureAttackDraft): string {
  if (a.melee) {
    const ft = rangeFeet(a.reachM);
    return `Corpo a corpo, ${metres(ft > 0 ? ft : SQUARE_FT)}`;
  }
  const range = rangeFeet(a.rangeM);
  const long = rangeFeet(a.longRangeM);
  const far = range > 0 ? metres(range) : '';
  return `À distância${far ? `, ${far}` : ''}${long > 0 ? ` e ${metres(long)}` : ''}`;
}

/** The line of a collapsed attack: "Corpo a corpo, 1,5 m · 2d6 contundente · acerto e dano da criatura". */
export function attackSummary(a: CreatureAttackDraft, nameOf: NameOf): string {
  const dice =
    a.diceCount.trim() !== '' && a.diceSides !== '' ? `${a.diceCount.trim()}d${a.diceSides}` : '';
  const type = a.damageType === '' ? '' : nameOf(a.damageType).toLowerCase();
  const hit = a.toHit.trim() !== '' ? `acerto ${formulaWords(a.toHit)}` : '';
  const dmg = a.damageBonus.trim() !== '' ? `dano ${formulaWords(a.damageBonus)}` : '';
  const own =
    hit === '' && dmg === ''
      ? 'acerto e dano da criatura'
      : [hit || 'acerto da criatura', dmg || 'dano da criatura'].join(', ');
  return joinDots([attackShape(a), [dice, type].filter((x) => x).join(' '), own].filter((x) => x));
}

/** The line of a collapsed trait: its text on one line. */
export function traitLine(text: string): string {
  return text.replace(/\s+/g, ' ').trim();
}

/** The list row's support: "Base Lobo · números do dono", "Base Corvo · PV 4, CA 13". */
export function creatureSupport(
  c: {
    baseKey: string;
    hitPoints: number;
    armorClass: number;
    hitPointsFormula: string;
    armorClassFormula: string;
    damageBonusFormula: string;
    attackBonusFormula: string;
  },
  nameOf: NameOf,
): string {
  const owner = [
    c.hitPointsFormula,
    c.armorClassFormula,
    c.damageBonusFormula,
    c.attackBonusFormula,
  ].some((f) => f !== '');
  const fixed = [
    c.hitPoints > 0 ? `PV ${c.hitPoints}` : '',
    c.armorClass > 0 ? `CA ${c.armorClass}` : '',
  ]
    .filter((x) => x)
    .join(', ');
  const numbers = owner ? 'números do dono' : fixed;
  return joinDots([`Base ${nameOf(c.baseKey)}`, numbers].filter((x) => x));
}
