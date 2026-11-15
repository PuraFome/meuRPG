/** An extra die typed beside the d20 (the effects' d4, the table die), with the sign it counts with. */
export interface TypedExtra {
  readonly face: number;
  readonly sign: number;
}

/** The extra dice typed with all their faces there, or null while one face is missing (or none is asked). */
export function typedExtrasOf(
  fields: readonly { readonly sign: number }[],
  faces: readonly (number | null)[],
): readonly TypedExtra[] | null {
  if (fields.length === 0 || faces.length !== fields.length || faces.some((f) => f === null)) {
    return null;
  }
  return fields.map((f, i) => ({ face: faces[i] as number, sign: f.sign }));
}

/** What the extra dice add to the d20 and the bonus: "12 + 6 + 5 = 23". */
export function typedFormulaOf(
  d20: number,
  bonus: number,
  extras: readonly TypedExtra[],
): { text: string; total: number } {
  const total = d20 + bonus + extrasSum(extras);
  const mod = bonus === 0 ? '' : ` ${bonus < 0 ? '−' : '+'} ${Math.abs(bonus)}`;
  const text = extras.map((e) => ` ${e.sign < 0 ? '−' : '+'} ${e.face}`).join('');
  return { text: `${d20}${mod}${text} = ${total}`, total };
}

/** The signed sum of the extra dice. */
export function extrasSum(extras: readonly TypedExtra[] | null): number {
  return extras?.reduce((n, e) => n + e.sign * e.face, 0) ?? 0;
}
