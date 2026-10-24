/** "CD 14" (no-break space): the DC of a saving throw as the screens say it, or `''` when the server sent none (absent or 0: never made up here). */
export function dcLabel(dc: number | undefined): string {
  return dc !== undefined && dc > 0 ? `CD\u00a0${dc}` : '';
}
