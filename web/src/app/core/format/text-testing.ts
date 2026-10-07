/** The text of a node as a person reads it, for the specs (never imported by the app): every piece of text trimmed and
 * joined with one space, so `<b>Tocha</b><span>6 m</span>` reads "Tocha 6 m", and a non-breaking space is a space. */
export function textOf(node: Node | null | undefined): string {
  if (!node) {
    return '';
  }
  const parts: string[] = [];
  const walker = document.createTreeWalker(node, NodeFilter.SHOW_TEXT);
  for (let n = walker.nextNode(); n; n = walker.nextNode()) {
    const text = (n.textContent ?? '')
      .replace(/\s+/g, ' ')
      .replace(/\u00a0/g, ' ')
      .trim();
    if (text) {
      parts.push(text);
    }
  }
  return parts.join(' ');
}
