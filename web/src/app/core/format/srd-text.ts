import { type Inline, parseInline } from '../../shared/markdown/markdown';

/**
 * The markdown of the SRD texts (spells and magic items), parsed to blocks the template draws with
 * text bindings, never `innerHTML`. One paragraph of the API is one line of the source: a line that
 * starts with "- " is a list item, a line that starts with "|" is a row of a table (the "|---|" row is
 * the separator), "#" lines are small headings, and everything else is a paragraph. Bold (`**`),
 * italics (`*` and `_`) come from the campaign documents' inline parser.
 */

export type SrdBlock =
  | { readonly type: 'paragraph'; readonly children: readonly Inline[] }
  | { readonly type: 'heading'; readonly children: readonly Inline[] }
  | { readonly type: 'list'; readonly items: readonly (readonly Inline[])[] }
  | {
      readonly type: 'table';
      readonly head: readonly (readonly Inline[])[];
      readonly rows: readonly (readonly (readonly Inline[])[])[];
    };

const SEPARATOR = /^\|?[\s:|-]+\|?$/;
const HEADING = /^#{1,6}\s+/;
const LIST_ITEM = /^[-*]\s+/;
/** `_italic_` at word edges, to the `*italic*` the inline parser knows. */
const UNDERSCORE_ITALIC = /(^|[^\p{L}\p{N}_])_([^_\n]+)_(?![\p{L}\p{N}_])/gu;

function inline(text: string): Inline[] {
  return parseInline(text.replace(UNDERSCORE_ITALIC, '$1*$2*'));
}

function cells(row: string): string[] {
  const body = row.trim().replace(/^\|/, '').replace(/\|$/, '');
  return body.split('|').map((c) => c.trim());
}

export function parseSrdText(paragraphs: readonly string[]): SrdBlock[] {
  const blocks: SrdBlock[] = [];
  let list: Inline[][] | null = null;
  let tableRows: string[] | null = null;

  const flushList = () => {
    if (list) {
      blocks.push({ type: 'list', items: list });
      list = null;
    }
  };
  const flushTable = () => {
    if (!tableRows) {
      return;
    }
    const rows = tableRows.filter((r) => !(SEPARATOR.test(r.trim()) && r.includes('-')));
    tableRows = null;
    if (rows.length === 0) {
      return;
    }
    const [head, ...body] = rows;
    blocks.push({
      type: 'table',
      head: cells(head).map(inline),
      rows: body.map((r) => cells(r).map(inline)),
    });
  };

  for (const raw of paragraphs) {
    const line = raw.trim();
    if (line === '') {
      continue;
    }
    if (line.startsWith('|')) {
      flushList();
      (tableRows ??= []).push(line);
      continue;
    }
    flushTable();
    if (LIST_ITEM.test(line)) {
      (list ??= []).push(inline(line.replace(LIST_ITEM, '')));
      continue;
    }
    flushList();
    blocks.push(
      HEADING.test(line)
        ? { type: 'heading', children: inline(line.replace(HEADING, '')) }
        : { type: 'paragraph', children: inline(line) },
    );
  }
  flushList();
  flushTable();
  return blocks;
}
