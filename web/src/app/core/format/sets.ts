/** A copy of the set with the key in it if it was out and out of it if it was in. */
export function toggledIn(current: ReadonlySet<string>, key: string): Set<string> {
  const next = new Set(current);
  if (next.has(key)) {
    next.delete(key);
  } else {
    next.add(key);
  }
  return next;
}

/** The set after a click when it takes at most `cap`: a third click while two are in does nothing, so the player
 * unticks one first. */
export function toggledCapped(current: ReadonlySet<string>, key: string, cap: number): Set<string> {
  return current.has(key) || current.size < cap ? toggledIn(current, key) : new Set(current);
}
