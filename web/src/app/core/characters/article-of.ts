const MASCULINE_IN_A = new Set(['mapa']);

/** The article a catalog name takes at the start of a sentence: "A Varinha de teia", "O Bordão" (the catalog carries no gender, so the first word's ending decides). */
export function articleOf(name: string): 'A' | 'O' {
  const first = (name.trim().split(/\s+/)[0] ?? '').toLowerCase();
  return first.endsWith('a') && !MASCULINE_IN_A.has(first) ? 'A' : 'O';
}
