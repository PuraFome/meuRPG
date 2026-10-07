import {
  type Combatant,
  CombatantKind,
  CombatantSide,
  CoverDegree,
  CoverSource,
  type TargetInReach,
} from '../../../gen/meurpg/play/v1/combat_pb';

/**
 * Cover on screen (MR-034, D4, E9-07). The server works out each attack's cover
 * and says it with its source; the app only names it, never an object ("Meia
 * cobertura", not "caixotes": it knows the degree) and never a number. Pure
 * functions, tested without a DOM.
 */

/** The name of a degree, or `''` for none. */
export function degreeWord(cover: CoverDegree): string {
  switch (cover) {
    case CoverDegree.HALF:
      return 'Meia cobertura';
    case CoverDegree.THREE_QUARTERS:
      return 'Três quartos';
    case CoverDegree.TOTAL:
      return 'Cobertura total';
    default:
      return '';
  }
}

/** "do mapa" or "marcada pelo mestre". */
export function sourceWord(source: CoverSource): string {
  return source === CoverSource.MARK
    ? 'marcada pelo mestre'
    : source === CoverSource.MAP
      ? 'do mapa'
      : '';
}

/** "Meia cobertura (do mapa)", "Cobertura total (marcada pelo mestre)", or `''`. */
export function coverText(cover: CoverDegree, source: CoverSource): string {
  const word = degreeWord(cover);
  const from = sourceWord(source);
  return word ? (from ? `${word} (${from})` : word) : '';
}

/** What the degree adds to the armor class, in words ("+2 na CA"), or `''` (none, total). The SRD's fixed numbers, said where a player reads the cover that counted. */
export function coverBonusText(cover: CoverDegree): string {
  return cover === CoverDegree.HALF
    ? '+2 na CA'
    : cover === CoverDegree.THREE_QUARTERS
      ? '+5 na CA'
      : '';
}

/** Which pictogram a degree draws (`mr-swatch--half`, `--three`); total has none. */
export function coverMark(cover: CoverDegree): 'half' | 'three' | null {
  return cover === CoverDegree.HALF
    ? 'half'
    : cover === CoverDegree.THREE_QUARTERS
      ? 'three'
      : null;
}

/** What a target list does with a target: list it, list it disabled with the
 * reason (total cover the master marked, on a target the player sees), or leave
 * it out (total cover from the map: a wall on the line, which the list never
 * explains, so as not to say what stands behind it). */
export type CoverListing =
  | { readonly kind: 'listed'; readonly text: string; readonly mark: 'half' | 'three' | null }
  | { readonly kind: 'blocked'; readonly text: string }
  | { readonly kind: 'left-out' };

export function listing(
  t: Pick<TargetInReach, 'cover' | 'coverSource' | 'untargetable'>,
): CoverListing {
  if (t.untargetable) {
    return t.coverSource === CoverSource.MARK
      ? {
          kind: 'blocked',
          text: `${coverText(CoverDegree.TOTAL, CoverSource.MARK)}: não pode ser alvo`,
        }
      : { kind: 'left-out' };
  }
  return { kind: 'listed', text: coverText(t.cover, t.coverSource), mark: coverMark(t.cover) };
}

/** The tags a combatant carries for everyone who sees it: "Aliado" (an NPC the
 * master marked, never secret) and the master's manual cover mark ("Três
 * quartos · marcada pelo mestre"). The map's cover is not a tag: it depends on
 * who attacks, so only the target list and the master's order say it. */
export function markTags(c: Pick<Combatant, 'kind' | 'side' | 'coverMark'>): string[] {
  return [...sideTags(c), ...coverMarkTags(c)];
}

/** "Aliado": a side, not a condition; never secret. */
export function sideTags(c: Pick<Combatant, 'kind' | 'side'>): string[] {
  return c.kind === CombatantKind.NPC && c.side === CombatantSide.PARTY ? ['Aliado'] : [];
}

/** The master's manual cover mark as a tag ("Três quartos · marcada pelo mestre"). */
export function coverMarkTags(c: Pick<Combatant, 'coverMark'>): string[] {
  const word = degreeWord(c.coverMark);
  return word ? [`${word} · marcada pelo mestre`] : [];
}
