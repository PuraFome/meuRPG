import { CreatureSize } from '../../../gen/meurpg/rules/v1/rules_pb';

/**
 * The SRD's creature types, in Portuguese, for a filter (the value is what
 * `ListCreatures` takes as `type`). Shared by "Dar uma criatura" and the bestiary.
 */
export const CREATURE_TYPES: readonly { value: string; label: string }[] = [
  { value: 'aberration', label: 'Aberração' },
  { value: 'beast', label: 'Fera' },
  { value: 'celestial', label: 'Celestial' },
  { value: 'construct', label: 'Constructo' },
  { value: 'dragon', label: 'Dragão' },
  { value: 'elemental', label: 'Elemental' },
  { value: 'swarm of Tiny beasts', label: 'Enxame de feras miúdas' },
  { value: 'fey', label: 'Fada' },
  { value: 'fiend', label: 'Ínfero' },
  { value: 'giant', label: 'Gigante' },
  { value: 'humanoid', label: 'Humanoide' },
  { value: 'monstrosity', label: 'Monstruosidade' },
  { value: 'ooze', label: 'Limo' },
  { value: 'plant', label: 'Planta' },
  { value: 'undead', label: 'Morto-vivo' },
];

/** The six creature sizes of the SRD, smallest first, with the word the bestiary says ("Imenso" is Gargantuan). */
export const CREATURE_SIZES: readonly { value: string; label: string; size: CreatureSize }[] = [
  { value: 'tiny', label: 'Miúdo', size: CreatureSize.TINY },
  { value: 'small', label: 'Pequeno', size: CreatureSize.SMALL },
  { value: 'medium', label: 'Médio', size: CreatureSize.MEDIUM },
  { value: 'large', label: 'Grande', size: CreatureSize.LARGE },
  { value: 'huge', label: 'Enorme', size: CreatureSize.HUGE },
  { value: 'gargantuan', label: 'Imenso', size: CreatureSize.GARGANTUAN },
];

/** Every challenge rating of the SRD, as `ListCreatures` takes them. */
export const CHALLENGE_RATINGS: readonly string[] = ['0', '1/8', '1/4', '1/2', ...Array.from({ length: 30 }, (_, i) => String(i + 1))];

/** What the bestiary's "ND" filter offers besides one exact rating: ranges (the value is `min-max`, either end empty). */
export const CHALLENGE_RANGES: readonly { value: string; label: string }[] = [
  { value: '0-1/2', label: 'ND 0 a 1/2' },
  { value: '1-4', label: 'ND 1 a 4' },
  { value: '5-10', label: 'ND 5 a 10' },
  { value: '11-30', label: 'ND 11 a 30' },
];

/** The `min_cr` and `max_cr` an "ND" filter value stands for: one rating ("1/4") is both; a range ("1-4") is its two ends. */
export function challengeBounds(value: string): { minCr: string; maxCr: string } {
  if (value === '') {
    return { minCr: '', maxCr: '' };
  }
  const range = CHALLENGE_RANGES.find((r) => r.value === value);
  if (range) {
    const [minCr, maxCr] = range.value.split('-');
    return { minCr, maxCr };
  }
  return { minCr: value, maxCr: value };
}
