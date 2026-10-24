/** "o" or "a" before a name: by its first word's ending, which is right for
 * the names this table uses (Brisa, Toren, Rapieira, Machado, Raio). */
export function article(name: string): 'o' | 'a' {
  // "Aranha-lobo gigante" is read by "Aranha", the word the article agrees with.
  const first = name.trim().split(/\s+/)[0].split('-')[0].toLowerCase();
  return /a$/.test(first) || /^(foice|rede|clava|mace)$/.test(first) ? 'a' : 'o';
}
