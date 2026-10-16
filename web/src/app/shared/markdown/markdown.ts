/**
 * The campaign document's Markdown subset (MR-018), parsed to a token tree.
 *
 * The body is whatever the master typed, so it is never trusted: this parser
 * only ever produces the tokens below, and `MarkdownView` renders them with
 * Angular templates (text interpolation and property bindings, never
 * `innerHTML`). Anything the subset does not know (raw HTML, other link
 * schemes such as `javascript:`, external images, nested lists, tables)
 * stays as plain text, exactly as typed.
 *
 * What it understands (campaign_document.proto, README-B):
 * - headings `#`, `##` and `###` (`#` and `##` are both a level-2 section:
 *   the page's own title is the h1);
 * - paragraphs (consecutive lines join with a space);
 * - `**bold**` and `*italic*`, nested up to {@link MAX_INLINE_DEPTH} deep;
 * - unordered lists (`- ` or `* `) and ordered lists (`1. `), one level;
 * - `[text](https://...)`, an external link (https only);
 * - `[text](map:<uuid>)` and `[text](character:<uuid>)`, the app's own links;
 * - `![caption](image:<uuid>)` alone on its line, a gallery image.
 * - `\` before a punctuation mark writes it literally.
 *
 * The parser is linear for any input: link text and URLs have a length
 * limit, and a work budget turns the rest of a pathological text into plain
 * text instead of letting it run long.
 */

export type RefKind = 'map' | 'character';

export type Inline =
  | { readonly type: 'text'; readonly text: string }
  | { readonly type: 'bold'; readonly children: readonly Inline[] }
  | { readonly type: 'italic'; readonly children: readonly Inline[] }
  | { readonly type: 'link'; readonly href: string; readonly text: string }
  | { readonly type: 'ref'; readonly kind: RefKind; readonly id: string; readonly text: string };

export type Block =
  | { readonly type: 'heading'; readonly level: 2 | 3; readonly children: readonly Inline[] }
  | { readonly type: 'paragraph'; readonly children: readonly Inline[] }
  | {
      readonly type: 'list';
      readonly ordered: boolean;
      readonly start: number;
      readonly items: readonly (readonly Inline[])[];
    }
  | { readonly type: 'image'; readonly id: string; readonly caption: string };

/** How deep `**` and `*` nest; deeper markers stay as text. */
export const MAX_INLINE_DEPTH = 6;
/** The longest link text and URL the parser looks at. */
const MAX_LINK_TEXT = 500;
const MAX_LINK_URL = 2048;
/** Characters the inline scanner may look at in one document. */
const WORK_BUDGET = 4_000_000;

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const PUNCTUATION = '\\`*_{}[]()#+-.!<>|~';

interface Budget {
  left: number;
}

const HEADING = /^ {0,3}(#{1,3}) +(\S.*)$/;
const GALLERY_IMAGE = /^ {0,3}!\[([^\]\n]{0,500})\]\(image:([^)\s]{1,64})\)$/;
const BULLET = /^ {0,3}[-*] +(\S.*)$/;
const NUMBERED = /^ {0,3}(\d{1,9})[.)] +(\S.*)$/;

type OpenList = { ordered: boolean; start: number; items: Inline[][] };

/** The blocks of one document being read line by line: the open paragraph and list wait here until something ends them. */
class BlockReader {
  readonly blocks: Block[] = [];
  private readonly budget: Budget = { left: WORK_BUDGET };
  private paragraph: string[] = [];
  private list: OpenList | null = null;

  /** One line of the source, in the order the subset tries its rules. */
  feed(raw: string): void {
    const line = raw.trimEnd();
    if (line.trim() === '') {
      this.flush();
      return;
    }
    if (
      this.heading(line) ||
      this.image(line) ||
      this.listItem(line) ||
      this.continuation(raw, line)
    ) {
      return;
    }
    this.flushList();
    this.paragraph.push(line.trim());
  }

  /** The end of the document closes whatever is open. */
  finish(): void {
    this.flush();
  }

  private flush(): void {
    this.flushParagraph();
    this.flushList();
  }

  private flushParagraph(): void {
    if (this.paragraph.length > 0) {
      this.blocks.push({
        type: 'paragraph',
        children: parseInline(this.paragraph.join(' '), this.budget),
      });
      this.paragraph = [];
    }
  }

  private flushList(): void {
    if (this.list) {
      this.blocks.push({
        type: 'list',
        ordered: this.list.ordered,
        start: this.list.start,
        items: this.list.items,
      });
      this.list = null;
    }
  }

  private heading(line: string): boolean {
    const heading = HEADING.exec(line);
    if (!heading) {
      return false;
    }
    this.flush();
    this.blocks.push({
      type: 'heading',
      level: heading[1].length === 3 ? 3 : 2,
      children: parseInline(heading[2].trim(), this.budget),
    });
    return true;
  }

  private image(line: string): boolean {
    const image = GALLERY_IMAGE.exec(line.trim());
    if (!image || !UUID.test(image[2])) {
      return false;
    }
    this.flush();
    this.blocks.push({ type: 'image', id: image[2].toLowerCase(), caption: image[1].trim() });
    return true;
  }

  private listItem(line: string): boolean {
    const bullet = BULLET.exec(line);
    const numbered = bullet ? null : NUMBERED.exec(line);
    if (!bullet && !numbered) {
      return false;
    }
    this.flushParagraph();
    const ordered = numbered !== null;
    if (this.list && this.list.ordered !== ordered) {
      this.flushList();
    }
    this.list ??= { ordered, start: numbered ? Number(numbered[1]) : 1, items: [] };
    this.list.items.push(parseInline((bullet ? bullet[1] : numbered![2]).trim(), this.budget));
    return true;
  }

  /** A plain line ends a list unless it is indented: then it continues the last item. */
  private continuation(raw: string, line: string): boolean {
    if (!this.list || !/^\s/.test(raw)) {
      return false;
    }
    // Appended in place: copying the item's tokens for every line would be
    // quadratic in the number of lines.
    const item = this.list.items[this.list.items.length - 1];
    for (const token of parseInline(' ' + line.trim(), this.budget)) {
      item.push(token);
    }
    return true;
  }
}

/** Parses a document body into blocks. */
export function parseMarkdown(source: string): Block[] {
  const reader = new BlockReader();
  for (const raw of source.replace(/\r\n?/g, '\n').split('\n')) {
    reader.feed(raw);
  }
  reader.finish();
  return reader.blocks;
}

/** What one step of the inline scanner read: some text to keep as typed, or a token; either way up to `end`. */
type Step =
  | { readonly text: string; readonly end: number }
  | { readonly token: Inline; readonly end: number };

/** `\` before a punctuation mark writes the mark literally. */
function readEscape(text: string, i: number): Step | null {
  return i + 1 < text.length && PUNCTUATION.includes(text[i + 1])
    ? { text: text[i + 1], end: i + 2 }
    : null;
}

/** An image that is not a gallery image on its own line (an external image, one inside a sentence) stays as text, "!" and all. */
function readImageText(text: string, i: number, budget: Budget): Step | null {
  if (text[i + 1] !== '[') {
    return null;
  }
  const image = readLink(text, i + 1, budget);
  return image ? { text: text.slice(i, image.end), end: image.end } : null;
}

function readLinkStep(text: string, i: number, budget: Budget): Step | null {
  const link = readLink(text, i, budget);
  return link ? { token: link.token, end: link.end } : null;
}

/** `**bold**` or `*italic*` at `i`; a marker with no valid closing stays as text. */
function readEmphasis(text: string, i: number, budget: Budget, depth: number): Step {
  const bold = text.startsWith('**', i);
  const marker = bold ? '**' : '*';
  const close = findClosing(text, i + marker.length, marker, budget);
  if (close > i + marker.length) {
    const inner = text.slice(i + marker.length, close);
    // Spaces right inside the markers mean it is not emphasis ("2 * 3 *").
    if (inner.trim() === inner) {
      const children = parseInline(inner, budget, depth + 1);
      return {
        token: { type: bold ? 'bold' : 'italic', children },
        end: close + marker.length,
      };
    }
  }
  return { text: marker, end: i + marker.length };
}

function readStep(text: string, i: number, budget: Budget, depth: number): Step | null {
  switch (text[i]) {
    case '\\':
      return readEscape(text, i);
    case '!':
      return readImageText(text, i, budget);
    case '[':
      return readLinkStep(text, i, budget);
    case '*':
      return depth < MAX_INLINE_DEPTH ? readEmphasis(text, i, budget, depth) : null;
    default:
      return null;
  }
}

/** Parses one run of text into inline tokens. */
export function parseInline(
  text: string,
  budget: Budget = { left: WORK_BUDGET },
  depth = 0,
): Inline[] {
  const out: Inline[] = [];
  let buffer = '';
  const flush = () => {
    if (buffer !== '') {
      out.push({ type: 'text', text: buffer });
      buffer = '';
    }
  };

  let i = 0;
  while (i < text.length) {
    if (budget.left <= 0) {
      buffer += text.slice(i);
      break;
    }
    budget.left -= 1;
    const step = readStep(text, i, budget, depth);
    if (!step) {
      buffer += text[i];
      i += 1;
      continue;
    }
    if ('token' in step) {
      flush();
      out.push(step.token);
    } else {
      buffer += step.text;
    }
    i = step.end;
  }
  flush();
  return out;
}

/** The position of the next unescaped closing `marker` at or after `from`,
 * or -1. A single `*` skips over `**` pairs. */
function findClosing(text: string, from: number, marker: string, budget: Budget): number {
  for (let j = from; j < text.length; j += 1) {
    budget.left -= 1;
    if (budget.left <= 0) {
      return -1;
    }
    const c = text[j];
    if (c === '\\') {
      j += 1;
    } else if (c === '[') {
      // Do not close inside a link's brackets: skip to its "]" if close.
      const end = text.indexOf(']', j);
      if (end !== -1 && end - j <= MAX_LINK_TEXT) {
        j = end;
      }
    } else if (c === '*') {
      if (marker === '**') {
        if (text[j + 1] === '*') {
          return j;
        }
      } else if (text[j + 1] === '*') {
        j += 1; // a `**` pair inside: skip it whole
      } else {
        return j;
      }
    }
  }
  return -1;
}

/** The label of a link: `start` is its "[", the result's `end` the "]" (or where the scan stopped). */
function readLabel(
  text: string,
  start: number,
  budget: Budget,
): { label: string; end: number } | null {
  let j = start + 1;
  let label = '';
  for (; j < text.length && j - start <= MAX_LINK_TEXT + 1; j += 1) {
    budget.left -= 1;
    const c = text[j];
    if (c === '\\' && j + 1 < text.length && PUNCTUATION.includes(text[j + 1])) {
      label += text[j + 1];
      j += 1;
    } else if (c === ']') {
      break;
    } else if (c === '[') {
      return null;
    } else {
      label += c;
    }
  }
  return { label, end: j };
}

/** The position of the ")" that ends a link's target from `urlStart`, or -1 when it has whitespace, a "(" or no end. */
function findTargetEnd(text: string, urlStart: number, budget: Budget): number {
  let k = urlStart;
  for (; k < text.length && k - urlStart <= MAX_LINK_URL; k += 1) {
    budget.left -= 1;
    const c = text[k];
    if (c === ')') {
      break;
    }
    if (/\s/.test(c) || c === '(') {
      return -1;
    }
  }
  return text[k] === ')' ? k : -1;
}

/** `[text](target)` at `start`, if it is a link the subset knows. */
function readLink(
  text: string,
  start: number,
  budget: Budget,
): { token: Inline; end: number } | null {
  const read = readLabel(text, start, budget);
  if (!read) {
    return null;
  }
  const { label, end: j } = read;
  if (text[j] !== ']' || text[j + 1] !== '(' || label.trim() === '') {
    return null;
  }
  const urlStart = j + 2;
  const k = findTargetEnd(text, urlStart, budget);
  if (k === -1) {
    return null;
  }
  const token = linkToken(label.trim(), text.slice(urlStart, k));
  return token ? { token, end: k + 1 } : null;
}

function linkToken(label: string, target: string): Inline | null {
  const colon = target.indexOf(':');
  const scheme = colon === -1 ? '' : target.slice(0, colon).toLowerCase();
  if (scheme === 'map' || scheme === 'character') {
    const id = target.slice(colon + 1);
    return UUID.test(id) ? { type: 'ref', kind: scheme, id: id.toLowerCase(), text: label } : null;
  }
  if (scheme === 'https') {
    const href = safeHttps(target);
    return href ? { type: 'link', href, text: label } : null;
  }
  return null;
}

/** The URL normalised by the browser, only if it is a plain https URL. */
export function safeHttps(target: string): string | null {
  try {
    const url = new URL(target);
    if (
      url.protocol !== 'https:' ||
      url.username !== '' ||
      url.password !== '' ||
      url.host === ''
    ) {
      return null;
    }
    return url.href;
  } catch {
    return null;
  }
}

/** A heading's plain text, for the table of contents. */
export function inlineText(children: readonly Inline[]): string {
  return children
    .map((c) => {
      switch (c.type) {
        case 'text':
          return c.text;
        case 'bold':
        case 'italic':
          return inlineText(c.children);
        default:
          return c.text;
      }
    })
    .join('');
}

/** The ids of the maps, sheets and images a document points to. */
export function collectReferences(blocks: readonly Block[]): {
  maps: Set<string>;
  characters: Set<string>;
  images: Set<string>;
} {
  const refs = {
    maps: new Set<string>(),
    characters: new Set<string>(),
    images: new Set<string>(),
  };
  const visit = (children: readonly Inline[]) => {
    for (const c of children) {
      if (c.type === 'ref') {
        (c.kind === 'map' ? refs.maps : refs.characters).add(c.id);
      } else if (c.type === 'bold' || c.type === 'italic') {
        visit(c.children);
      }
    }
  };
  for (const b of blocks) {
    if (b.type === 'image') {
      refs.images.add(b.id);
    } else if (b.type === 'list') {
      b.items.forEach(visit);
    } else {
      visit(b.children);
    }
  }
  return refs;
}

/** The headings the table of contents lists: the level-2 sections. */
export function outline(blocks: readonly Block[]): { index: number; text: string }[] {
  const out: { index: number; text: string }[] = [];
  blocks.forEach((b, index) => {
    if (b.type === 'heading' && b.level === 2) {
      const text = inlineText(b.children).trim();
      if (text !== '') {
        out.push({ index, text });
      }
    }
  });
  return out;
}
