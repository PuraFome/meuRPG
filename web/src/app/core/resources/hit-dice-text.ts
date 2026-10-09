/** Hit dice of one size and how many of them are spent (`CharacterVitals.hit_dice` with `hit_dice_used_by_die`). */
export interface HitDieSize {
  readonly faces: number;
  readonly total: number;
  readonly used: number;
}

/** "d10", "d6": the die by its size. */
export function dieName(faces: number): string {
  return `d${faces}`;
}

/** "a", "a e b", "a, b e c": a list the way it is read. */
export function listWords(items: readonly string[]): string {
  if (items.length <= 1) {
    return items[0] ?? '';
  }
  return `${items.slice(0, -1).join(', ')} e ${items[items.length - 1]}`;
}

/** "5d10 e 1d6": hit dice by size, as the level-up summary says them (PM-08c). "—" when there are none. */
export function hitDiceWords(
  dice: readonly { readonly faces: number; readonly count: number }[],
): string {
  const sizes = dice.filter((d) => d.count > 0).map((d) => `${d.count}${dieName(d.faces)}`);
  return sizes.length === 0 ? '—' : listWords(sizes);
}

/** The dice a character has left of one size. */
export function diceLeft(size: HitDieSize): number {
  return Math.max(0, size.total - size.used);
}

/** "3 de 5d10 e 1 de 1d6": what is left of each size, next to how many there are. */
export function hitDiceLeftWords(sizes: readonly HitDieSize[]): string {
  const parts = sizes
    .filter((s) => s.total > 0)
    .map((s) => `${diceLeft(s)} de ${s.total}${dieName(s.faces)}`);
  return parts.length === 0 ? '—' : listWords(parts);
}

/** How many dice are left in all sizes. */
export function totalDiceLeft(sizes: readonly HitDieSize[]): number {
  return sizes.reduce((sum, s) => sum + diceLeft(s), 0);
}
