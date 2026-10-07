/**
 * The editor toolbar's text changes, as pure functions (E5-28): each takes
 * the text and the selection and says what to replace and where the
 * selection goes after. The editor applies the replacement to the
 * `<textarea>` (through `execCommand('insertText')` when it can, so Ctrl+Z
 * still works).
 */
export interface Edit {
  /** The range of the old text to replace. */
  readonly from: number;
  readonly to: number;
  readonly insert: string;
  /** The selection after the change, in the new text. */
  readonly selStart: number;
  readonly selEnd: number;
}

export function applyEdit(text: string, edit: Edit): string {
  return text.slice(0, edit.from) + edit.insert + text.slice(edit.to);
}

/** The start of the line at `pos` and the end of the line (before "\n"). */
function lineBounds(text: string, pos: number): { start: number; end: number } {
  const start = text.lastIndexOf('\n', pos - 1) + 1;
  const nl = text.indexOf('\n', pos);
  return { start, end: nl === -1 ? text.length : nl };
}

/** "Título": the line gets `## `; on a heading already, a second press
 * takes it off (and any other `#` level becomes `##`). */
export function toggleHeading(text: string, selStart: number): Edit {
  const { start, end } = lineBounds(text, selStart);
  const line = text.slice(start, end);
  const match = /^#{1,3} +/.exec(line);
  const replaced = match ? match[0] : '';
  const next = match && match[0] === '## ' ? '' : '## ';
  return {
    from: start,
    to: start + replaced.length,
    insert: next,
    selStart: Math.max(start, selStart - replaced.length + next.length),
    selEnd: Math.max(start, selStart - replaced.length + next.length),
  };
}

/** "Negrito" (`**`) and "Itálico" (`*`): wraps the selection, or writes
 * the marker pair around a placeholder that stays selected. */
export function wrap(text: string, selStart: number, selEnd: number, marker: string): Edit {
  const chosen = text.slice(selStart, selEnd);
  if (chosen === '') {
    const placeholder = 'texto';
    return {
      from: selStart,
      to: selEnd,
      insert: marker + placeholder + marker,
      selStart: selStart + marker.length,
      selEnd: selStart + marker.length + placeholder.length,
    };
  }
  return {
    from: selStart,
    to: selEnd,
    insert: marker + chosen + marker,
    selStart: selStart + marker.length,
    selEnd: selStart + marker.length + chosen.length,
  };
}

/** "Lista": every line touched by the selection gets `- `; if all of them
 * already have it, it comes off. */
export function toggleList(text: string, selStart: number, selEnd: number): Edit {
  const first = lineBounds(text, selStart);
  const last = lineBounds(text, Math.max(selStart, selEnd));
  const block = text.slice(first.start, last.end);
  const lines = block.split('\n');
  const content = lines.filter((l) => l.trim() !== '');
  const allListed = content.length > 0 && content.every((l) => l.startsWith('- '));
  const changed = lines
    .map((l) => (l.trim() === '' ? l : allListed ? l.slice(2) : `- ${l}`))
    .join('\n');
  return {
    from: first.start,
    to: last.end,
    insert: changed,
    selStart: first.start,
    selEnd: first.start + changed.length,
  };
}

/** Writes `[label](target)` at the selection; the selected text, if any,
 * becomes the label. */
export function insertLink(
  text: string,
  selStart: number,
  selEnd: number,
  fallbackLabel: string,
  target: string,
): Edit {
  const label = cleanLabel(text.slice(selStart, selEnd)) || cleanLabel(fallbackLabel);
  const insert = `[${label}](${target})`;
  const caret = selStart + insert.length;
  return { from: selStart, to: selEnd, insert, selStart: caret, selEnd: caret };
}

/** Writes `![name](image:<id>)` on a paragraph of its own, with the caret
 * on the paragraph after it. */
export function insertImage(text: string, selStart: number, selEnd: number, name: string, id: string): Edit {
  const line = `![${cleanLabel(name)}](image:${id})`;
  const before = text.slice(0, selStart);
  const after = text.slice(selEnd);
  const lead = before === '' || before.endsWith('\n\n') ? '' : before.endsWith('\n') ? '\n' : '\n\n';
  // Always a blank line after, so what the master writes next is a new paragraph.
  const tail = after.startsWith('\n\n') ? '' : after.startsWith('\n') ? '\n' : '\n\n';
  const insert = lead + line + tail;
  const caret = selStart + insert.length;
  return { from: selStart, to: selEnd, insert, selStart: caret, selEnd: caret };
}

/** A label that cannot break the link: no brackets, no line breaks. */
function cleanLabel(label: string): string {
  return label.replace(/[[\]]+/g, '').replace(/\s+/g, ' ').trim();
}

/** The most the server takes (UpdateCampaignDocumentRequest.body). */
export const MAX_BODY_BYTES = 204_800;

/** The body as the server counts it: line breaks changed to "\n", then the
 * bytes of its UTF-8. */
export function normalizeBody(body: string): string {
  return body.replace(/\r\n?/g, '\n');
}

export function bodyBytes(body: string): number {
  return new TextEncoder().encode(normalizeBody(body)).length;
}

/** "184 KB de 200 KB", shown from 90% of the limit. */
export function bytesNotice(bytes: number): string | null {
  if (bytes < MAX_BODY_BYTES * 0.9) {
    return null;
  }
  const kb = (n: number) => `${Math.round(n / 1024)} KB`;
  return `${kb(bytes)} de ${kb(MAX_BODY_BYTES)}`;
}
