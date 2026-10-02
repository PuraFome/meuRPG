import { expect, type Page } from '@playwright/test';

// Layout checks that axe does not make (docs/design.md#como-uma-tela-é-feita).
// They catch what a screenshot review missed in PR #56: a search field's
// words over its magnifier, radio tiles with their words stuck to the top,
// and button icons a pixel or two below their words. Each check measures the
// page; none compares pictures, so a new screen needs no reference image.
//
// 1. An icon with words in the same row (a button, a tag, a notice): the
//    icon's middle lines up with the middle of the capitals, within 1px
//    (with the first line, in a row aligned to the top). A button's or a
//    chip's content sits in the middle of it, within 2px.
// 2. No text runs over an icon by more than 2px.
// 3. A radio or checkbox drawn as a tile (it has a border): its words keep
//    at least 6px from the tile's top and bottom edges. Centred or
//    aligned to the top are both fine (the design decides); words stuck to
//    an edge are not.
//
// None of these asks for things to be centred in general: a button's own
// line is centred because a button is a fixed-height control, and an icon
// lines up with its own words. Everything else follows the approved design.

export interface LayoutIssue {
  kind: 'icon-text' | 'off-centre' | 'overlap' | 'tile';
  where: string;
  detail: string;
}

/** Runs in the page (it is serialized, so it uses nothing from outside). */
function auditLayout(): LayoutIssue[] {
  const issues: LayoutIssue[] = [];
  const ctx = document.createElement('canvas').getContext('2d')!;

  const visible = (el: Element): boolean => {
    const r = el.getBoundingClientRect();
    // 1px or less: a visually hidden label (clipped), for screen readers.
    if (r.width <= 1 || r.height <= 1) return false;
    const cs = getComputedStyle(el);
    return cs.visibility !== 'hidden' && cs.display !== 'none' && Number(cs.opacity) !== 0;
  };
  const describe = (el: Element): string => {
    const name = (el.getAttribute('aria-label') || el.textContent || '').replace(/\s+/g, ' ').trim();
    return `<${el.tagName.toLowerCase()}> "${name.slice(0, 50)}"`;
  };
  // The text a person reads in an element: not the icons' ligature names
  // ("arrow_back") nor the visually hidden words for screen readers.
  const textNodesOf = (el: Element): Text[] => {
    const out: Text[] = [];
    const walker = document.createTreeWalker(el, NodeFilter.SHOW_TEXT);
    for (let n = walker.nextNode(); n; n = walker.nextNode()) {
      const p = n.parentElement;
      if (!p || !n.textContent?.trim()) continue;
      if (p.closest('mat-icon, .mat-icon, .mr-visually-hidden, .cdk-visually-hidden')) continue;
      if (!visible(p)) continue;
      out.push(n as Text);
    }
    return out;
  };
  // Where a text's capitals are on screen: from the baseline up to the
  // height of an "H" in its font. Icons line up with that band.
  const capitals = (node: Text) => {
    const range = document.createRange();
    range.selectNodeContents(node);
    const rects = [...range.getClientRects()].filter((r) => r.width > 0 && r.height > 0);
    if (rects.length === 0) return null;
    const cs = getComputedStyle(node.parentElement!);
    ctx.font = `${cs.fontStyle} ${cs.fontWeight} ${cs.fontSize} ${cs.fontFamily}`;
    const h = ctx.measureText('H');
    const baseline = rects[0].top + h.fontBoundingBoxAscent;
    return {
      top: baseline - h.actualBoundingBoxAscent,
      bottom: baseline,
      lines: new Set(rects.map((r) => Math.round(r.top))).size,
    };
  };

  // 1a. Every icon with words in the same row: the icon's middle lines up
  //     with the middle of the capitals. In a row centred on its cross axis
  //     (a button, a tag, "Salvo."), only when the words fit on one line; in
  //     a row aligned to the top (a notice), with the first line.
  // A badge pinned to a corner (position: absolute) is placed on purpose.
  const inFlow = (el: Element) => !['absolute', 'fixed'].includes(getComputedStyle(el).position);
  for (const icon of document.querySelectorAll('mat-icon, .mat-icon')) {
    if (!visible(icon) || !inFlow(icon) || !inFlow(icon.parentElement!)) continue;
    if (icon.closest('.mat-mdc-form-field-icon-prefix, .mat-mdc-form-field-icon-suffix')) continue;
    const row = icon.parentElement;
    if (!row) continue;
    const cs = getComputedStyle(row);
    if (!cs.display.includes('flex') || cs.flexDirection.startsWith('column')) continue;
    const align = getComputedStyle(icon).alignSelf === 'auto' ? cs.alignItems : getComputedStyle(icon).alignSelf;
    const texts = textNodesOf(row)
      .map(capitals)
      .filter((t) => t !== null);
    if (texts.length === 0) continue;
    const topAligned = align === 'flex-start' || align === 'start';
    const oneLine = texts.every((t) => t.lines === 1) && new Set(texts.map((t) => Math.round(t.top))).size === 1;
    if (!topAligned && (!oneLine || align !== 'center')) continue;
    const first = texts.reduce((a, b) => (b.top < a.top ? b : a));
    const r = icon.getBoundingClientRect();
    const d = r.top + r.height / 2 - (first.top + first.bottom) / 2;
    if (Math.abs(d) > 1) {
      issues.push({
        kind: 'icon-text',
        where: describe(row),
        detail: `the icon's middle is ${Math.abs(d).toFixed(1)}px ${d > 0 ? 'below' : 'above'} the middle of the capitals`,
      });
    }
  }

  // 1b. A control's content (icon and one line of words) sits in the middle
  //     of the control.
  for (const c of document.querySelectorAll('button, a.mat-mdc-button-base, .mat-mdc-chip')) {
    if (!visible(c)) continue;
    const box = c.getBoundingClientRect();
    if (box.height > 72) continue;
    const icons = [...c.querySelectorAll('mat-icon, .mat-icon')].filter(
      (i) => visible(i) && inFlow(i) && inFlow(i.parentElement!),
    );
    const texts = textNodesOf(c)
      .map(capitals)
      .filter((t) => t !== null);
    if (texts.length === 0) continue;
    const oneLine = texts.every((t) => t.lines === 1) && new Set(texts.map((t) => Math.round(t.top))).size === 1;
    if (!oneLine) continue;
    const tops = [texts[0].top, ...icons.map((i) => i.getBoundingClientRect().top)];
    const bottoms = [texts[0].bottom, ...icons.map((i) => i.getBoundingClientRect().bottom)];
    const d = (Math.min(...tops) + Math.max(...bottoms)) / 2 - (box.top + box.height / 2);
    if (Math.abs(d) > 2) {
      issues.push({
        kind: 'off-centre',
        where: describe(c),
        detail: `the content is ${Math.abs(d).toFixed(1)}px ${d > 0 ? 'below' : 'above'} the middle`,
      });
    }
  }

  // 2. Text over an icon.
  for (const icon of document.querySelectorAll('mat-icon, .mat-icon')) {
    if (!visible(icon)) continue;
    const ir = icon.getBoundingClientRect();
    const scope = icon.closest('mat-form-field, button, label, li') ?? icon.parentElement;
    if (!scope) continue;
    for (const n of textNodesOf(scope)) {
      const range = document.createRange();
      range.selectNodeContents(n);
      for (const r of range.getClientRects()) {
        const x = Math.min(r.right, ir.right) - Math.max(r.left, ir.left);
        const y = Math.min(r.bottom, ir.bottom) - Math.max(r.top, ir.top);
        if (x > 2 && y > 2) {
          issues.push({
            kind: 'overlap',
            where: describe(scope),
            detail: `"${n.textContent!.trim().slice(0, 30)}" runs ${x.toFixed(0)}px over the icon "${icon.textContent!.trim()}"`,
          });
        }
      }
    }
  }

  // 3. Radio and checkbox tiles: the words keep clear of the edges. Measured
  //    on the words' lines (the radio's 40px touch area may reach closer, by
  //    design). PR #56 had them 2px from the top and 25px from the bottom.
  for (const tile of document.querySelectorAll('mat-radio-button, mat-checkbox')) {
    if (!visible(tile) || parseFloat(getComputedStyle(tile).borderTopWidth) === 0) continue;
    const lines = textNodesOf(tile).flatMap((n) => {
      const range = document.createRange();
      range.selectNodeContents(n);
      return [...range.getClientRects()].filter((r) => r.height > 0);
    });
    if (lines.length === 0) continue;
    const t = tile.getBoundingClientRect();
    const top = Math.min(...lines.map((r) => r.top)) - t.top;
    const bottom = t.bottom - Math.max(...lines.map((r) => r.bottom));
    if (top < 6 || bottom < 6) {
      issues.push({
        kind: 'tile',
        where: describe(tile),
        detail: `the words are ${top.toFixed(0)}px from the top edge and ${bottom.toFixed(0)}px from the bottom one (at least 6px each)`,
      });
    }
  }
  return issues;
}

/** Lists one line per misplaced piece on the screen as it is now. A soft
 * check: the test goes on to its next screens and fails at the end, so one
 * run shows every screen's findings. */
export async function expectAligned(page: Page, screen: string): Promise<void> {
  await page.evaluate(() => document.fonts.ready);
  const issues = await page.evaluate(auditLayout);
  expect.soft(issues.map((i) => `${screen}: ${i.kind} ${i.where}: ${i.detail}`)).toEqual([]);
}
