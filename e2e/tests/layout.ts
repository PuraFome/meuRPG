import { expect, type Page } from '@playwright/test';

// Layout checks that axe does not make (docs/design/README.md#how-a-screen-is-made).
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
// 4. Inside a card (`.mr-panel`), no button or link sits outside the card's
//    box: a row whose columns do not shrink pushed "⋮" and "Revelar aos
//    jogadores" out of the initiative card beside the session's side panels.
// 5. A form field's floating label keeps at least 4px (capitals to baseline)
//    from the words just above it: the outline of the field pushes the label
//    up over its own box, so a heading placed right over the field touched it
//    ("Dano a mais" over "Dados", #577). The approved board leaves about
//    13px; under 4px the two read as touching.
// 6. An attunement chip on a gear row stays on the line of the words before it: a narrow column wraps the name and the
//    chip follows its last word; a chip alone on a line under the name is a finding.
// 7. The heading of a section with a count beside it ("Sintonização" and "Sintonizados: 2 de 3") shares a baseline with
//    it, within 1px.
// 8. A row marked `data-one-line` is one line: none of its words wraps and every box in it shares the first box's line
//    (a gear row ellipsizes its name before anything drops under it). A name keeps at least 24px (three letters) and
//    the row's words end before the button's column starts.
// 9. A box marked `data-whole` shows each thing it holds whole or not at all: an item that is drawn (its top is inside the
//    box) does not reach past the box's left or right edge ("duas mãos" cut to "duas mã", an icon cut in half).
// 10. A group marked `data-equal-width="<selector>"` has every visible element matching the selector the same width, within
//    1px (the buttons of one list, the pair at the foot of a sheet).
// 11. A chip marked `data-chip` sits on the middle of the capitals of the words marked `data-chip-ref` in its row
//    (`data-chip-row`), within 2px.
// 12. A name marked `data-chip-ref` in a `data-one-line` row does not end in an ellipsis while the row's main box has room:
//    the space between two of its items, or after the last one, is at most 4px beyond the box's own gap.
//
// None of these asks for things to be centred in general: a button's own
// line is centred because a button is a fixed-height control, and an icon
// lines up with its own words. Everything else follows the approved design.

export interface LayoutIssue {
  kind: 'icon-text' | 'one-line' | 'unequal-width' | 'chip-baseline' | 'chip-alone' | 'head-baseline' | 'off-centre' | 'overlap' | 'tile' | 'outside-card' | 'label-touch' | 'cut' | 'name-cut-with-room';
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
  const describeEl = (el: Element): string => {
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
        where: describeEl(row),
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
        where: describeEl(c),
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
      // Words clipped by an ancestor (an ellipsized name) are not drawn past its edge.
      let clip: DOMRect | null = null;
      for (let a = n.parentElement; a && a !== scope.parentElement; a = a.parentElement) {
        if (getComputedStyle(a).overflowX !== 'visible') {
          clip = a.getBoundingClientRect();
          break;
        }
      }
      for (const r of range.getClientRects()) {
        const left = clip ? Math.max(r.left, clip.left) : r.left;
        const right = clip ? Math.min(r.right, clip.right) : r.right;
        const x = Math.min(right, ir.right) - Math.max(left, ir.left);
        const y = Math.min(r.bottom, ir.bottom) - Math.max(r.top, ir.top);
        if (x > 2 && y > 2) {
          issues.push({
            kind: 'overlap',
            where: describeEl(scope),
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
        where: describeEl(tile),
        detail: `the words are ${top.toFixed(0)}px from the top edge and ${bottom.toFixed(0)}px from the bottom one (at least 6px each)`,
      });
    }
  }

  // 4. Controls stay inside their card. A card that scrolls on purpose is
  //    left out, and so is a control that is not on screen.
  for (const card of document.querySelectorAll('.mr-panel')) {
    if (!visible(card)) continue;
    const cb = card.getBoundingClientRect();
    for (const c of card.querySelectorAll('button, a[href]')) {
      if (!visible(c) || !inFlow(c) || c.closest('.cdk-visually-hidden, .mr-visually-hidden')) continue;
      const nearest = c.closest('.mr-panel');
      if (nearest !== card) continue;
      // A strip that scrolls sideways on purpose (the tabs of a narrow card) clips its own controls.
      const strip = c.closest('[role="tablist"]');
      if (strip && ['auto', 'scroll'].includes(getComputedStyle(strip).overflowX)) continue;
      const r = c.getBoundingClientRect();
      if (r.right > cb.right + 1 || r.left < cb.left - 1) {
        issues.push({
          kind: 'outside-card',
          where: `<${c.tagName.toLowerCase()}> "${(c.getAttribute('aria-label') || c.textContent || '').replace(/\s+/g, ' ').trim().slice(0, 50)}"`,
          detail: `sticks out of its card by ${Math.max(r.right - cb.right, cb.left - r.left).toFixed(0)}px`,
        });
      }
    }
  }

  // 5. A floating label keeps clear of the words above it. Measured from the
  //    baseline of those words to the top of the label's capitals (the label
  //    is scaled down, so its band is scaled the same way). Only words that
  //    overlap the label's columns and end above the label's capitals count
  //    (words beside it or under it are not "above"); the field's own words,
  //    an open menu and a bar that floats over the page are left out.
  const MIN_LABEL_GAP = 4;
  // 6. A gear row's attunement chip is on the last line of the words before it.
  for (const chip of document.querySelectorAll('.gear-row__att')) {
    if (!visible(chip)) continue;
    const before = chip.previousElementSibling ?? chip.parentElement?.firstElementChild;
    if (!before || before === chip) continue;
    const rects = [...before.getClientRects()].filter((r) => r.width > 0 && r.height > 0);
    if (rects.length === 0) continue;
    const last = rects[rects.length - 1];
    const c = chip.getBoundingClientRect();
    const middle = c.top + c.height / 2;
    if (middle > last.bottom + 1 || middle < last.top - 1) {
      issues.push({
        kind: 'chip-alone',
        where: describeEl(chip.closest('.gear-row') ?? chip),
        detail: `the chip's middle is at ${middle.toFixed(0)}px and the words before it end on a line from ${last.top.toFixed(0)}px to ${last.bottom.toFixed(0)}px: it sits on a line of its own`,
      });
    }
  }

  // 7. The heading and the count of a section head share a baseline.
  for (const head of document.querySelectorAll('.att__head')) {
    if (!visible(head)) continue;
    const title = head.querySelector('.att__title');
    const count = head.querySelector('.att__count');
    if (!title || !count) continue;
    const baselineOf = (el: Element): number | null => {
      const node = textNodesOf(el).filter((n) => n.parentElement!.closest('.mat-icon') === null)[0];
      if (!node) return null;
      return capitals(node)?.bottom ?? null;
    };
    const a = baselineOf(title);
    const b = baselineOf(count);
    if (a !== null && b !== null && Math.abs(a - b) > 1) {
      issues.push({
        kind: 'head-baseline',
        where: describeEl(head),
        detail: `the heading's baseline is at ${a.toFixed(1)}px and the count's at ${b.toFixed(1)}px (within 1px)`,
      });
    }
  }

  // 8. A row marked data-one-line is one line.
  for (const row of document.querySelectorAll('[data-one-line]')) {
    if (!visible(row)) continue;
    const boxes = [...row.children].filter((c) => visible(c) && inFlow(c));
    if (boxes.length === 0) continue;
    const first = boxes[0].getBoundingClientRect();
    const wraps = textNodesOf(row).filter((n) => {
      const r = document.createRange();
      r.selectNodeContents(n);
      return new Set([...r.getClientRects()].filter((q) => q.width > 0 && q.height > 0).map((q) => Math.round(q.top))).size > 1;
    });
    const off = boxes.filter((b) => {
      const r = b.getBoundingClientRect();
      return r.top > first.bottom - 1 || r.bottom < first.top + 1;
    });
    // A name squeezed to nothing is clipped by its own box, so the overlap check never sees it.
    const squeezed = [...row.querySelectorAll('[data-chip-ref]')].filter((n) => n.getBoundingClientRect().width < Math.min(24, n.scrollWidth));
    const button = row.closest('[data-equal-width]')
      ? [...row.querySelectorAll(row.closest('[data-equal-width]')!.getAttribute('data-equal-width')!)].find(visible)
      : undefined;
    // The main box's edge and the right edge of what it holds (a box that does not shrink would stick out of it).
    const main = { right: Math.max(boxes[0].getBoundingClientRect().right, ...[...boxes[0].children].filter(visible).map((c) => c.getBoundingClientRect().right)) };
    const underButton = button && main.right > button.getBoundingClientRect().left + 0.5 ? button : undefined;
    if (squeezed.length > 0) {
      issues.push({
        kind: 'one-line',
        where: describeEl(row),
        detail: `the name "${squeezed[0].textContent!.trim().slice(0, 30)}" is ${squeezed[0].getBoundingClientRect().width.toFixed(0)}px wide (at least 24px)`,
      });
    }
    if (underButton) {
      issues.push({
        kind: 'one-line',
        where: describeEl(row),
        detail: `the row's words end at ${main.right.toFixed(0)}px, over ${describeEl(underButton)} that starts at ${underButton.getBoundingClientRect().left.toFixed(0)}px`,
      });
    }
    if (wraps.length > 0 || off.length > 0) {
      issues.push({
        kind: 'one-line',
        where: describeEl(row),
        detail: wraps.length > 0 ? `the words "${wraps[0].textContent!.trim().slice(0, 30)}" wrap onto a second line` : `${describeEl(off[0])} sits on another line than ${describeEl(boxes[0])}`,
      });
    }
  }

  // 9. A box marked data-whole shows each item whole or not at all.
  for (const box of document.querySelectorAll('[data-whole]')) {
    if (!visible(box)) continue;
    const edge = box.getBoundingClientRect();
    for (const item of box.children) {
      const r = item.getBoundingClientRect();
      if (r.top < edge.bottom - 1 && (r.right > edge.right + 0.5 || r.left < edge.left - 0.5)) {
        issues.push({
          kind: 'cut',
          where: describeEl(box),
          detail: `"${(item.textContent ?? '').trim().slice(0, 30) || item.tagName.toLowerCase()}" spans ${r.left.toFixed(0)}px to ${r.right.toFixed(0)}px in a box from ${edge.left.toFixed(0)}px to ${edge.right.toFixed(0)}px: it is drawn cut`,
        });
      }
    }
  }

  // 10. The elements a group marks have one width.
  for (const group of document.querySelectorAll('[data-equal-width]')) {
    if (!visible(group)) continue;
    const widths = [...group.querySelectorAll(group.getAttribute('data-equal-width')!)]
      .filter(visible)
      .map((el) => ({ el, w: el.getBoundingClientRect().width }));
    if (widths.length < 2) continue;
    const min = widths.reduce((a, b) => (b.w < a.w ? b : a));
    const max = widths.reduce((a, b) => (b.w > a.w ? b : a));
    if (max.w - min.w > 1) {
      issues.push({
        kind: 'unequal-width',
        where: describeEl(group),
        detail: `${describeEl(max.el)} is ${max.w.toFixed(1)}px wide and ${describeEl(min.el)} ${min.w.toFixed(1)}px (equal within 1px)`,
      });
    }
  }

  // 11. A marked chip is on the middle of the capitals of its row's words.
  for (const chip of document.querySelectorAll('[data-chip]')) {
    if (!visible(chip)) continue;
    const ref = chip.closest('[data-chip-row]')?.querySelector('[data-chip-ref]');
    const node = ref && visible(ref) ? textNodesOf(ref)[0] : null;
    const cap = node ? capitals(node) : null;
    if (!cap) continue;
    const c = chip.getBoundingClientRect();
    const diff = Math.abs(c.top + c.height / 2 - (cap.top + cap.bottom) / 2);
    if (diff > 2) {
      issues.push({
        kind: 'chip-baseline',
        where: describeEl(chip),
        detail: `the chip's middle is ${diff.toFixed(1)}px from the middle of the capitals of its row's words (within 2px)`,
      });
    }
  }

  // 12. A name cut with an ellipsis while the row's main box has room left.
  for (const row of document.querySelectorAll('[data-one-line]')) {
    if (!visible(row)) continue;
    const main = [...row.children].find((c) => visible(c) && inFlow(c));
    if (!main) continue;
    const cut = [...row.querySelectorAll('[data-chip-ref]')].find((n) => visible(n) && n.scrollWidth > n.clientWidth);
    if (!cut) continue;
    const box = main.getBoundingClientRect();
    const gap = parseFloat(getComputedStyle(main).columnGap) || 0;
    const items = [...main.children]
      .filter((c) => visible(c) && inFlow(c) && c.getBoundingClientRect().width > 0)
      .map((c) => c.getBoundingClientRect())
      .sort((a, b) => a.left - b.left);
    const free = Math.max(
      ...items.map((r, i) => (i === 0 ? 0 : r.left - items[i - 1].right - gap)),
      items.length > 0 ? box.right - items[items.length - 1].right : 0,
    );
    if (free > 4) {
      issues.push({
        kind: 'name-cut-with-room',
        where: describeEl(row),
        detail: `the name "${cut.textContent!.trim().slice(0, 30)}" is cut with an ellipsis while its row has ${free.toFixed(1)}px free (at most 4px)`,
      });
    }
  }

  // Every line of words on the page, once: its columns and the baseline of its capitals.
  let all: { node: Text; left: number; right: number; baseline: number }[] | null = null;
  const lines = () => {
    if (all) return all;
    all = [];
    for (const node of textNodesOf(document.body)) {
      // Words in an open menu or in a bar that stays on the screen while the page scrolls lie over the form, not above a field.
      if (node.parentElement!.closest('.cdk-overlay-container')) continue;
      let floating = false;
      for (let e: Element | null = node.parentElement; e && !floating; e = e.parentElement) floating = ['fixed', 'sticky'].includes(getComputedStyle(e).position);
      if (floating) continue;
      const cap = capitals(node);
      if (!cap) continue;
      const range = document.createRange();
      range.selectNodeContents(node);
      const rects = [...range.getClientRects()].filter((r) => r.width > 0 && r.height > 0);
      for (const r of rects) all.push({ node, left: r.left, right: r.right, baseline: r.top + (cap.bottom - rects[0].top) });
    }
    return all;
  };
  for (const label of document.querySelectorAll('.mdc-floating-label--float-above')) {
    const field = label.closest('mat-form-field, .mat-mdc-form-field');
    if (!field || !visible(label) || !label.textContent?.trim()) continue;
    const lnode = textNodesOf(label)[0];
    if (!lnode) continue;
    const lr = document.createRange();
    lr.selectNodeContents(lnode);
    const lrect = [...lr.getClientRects()].find((r) => r.width > 0 && r.height > 0);
    if (!lrect) continue;
    const lcs = getComputedStyle(lnode.parentElement!);
    ctx.font = `${lcs.fontStyle} ${lcs.fontWeight} ${lcs.fontSize} ${lcs.fontFamily}`;
    const lh = ctx.measureText('H');
    const scale = lrect.height / (lh.fontBoundingBoxAscent + lh.fontBoundingBoxDescent);
    const capTop = lrect.top + (lh.fontBoundingBoxAscent - lh.actualBoundingBoxAscent) * scale;
    let nearest: { baseline: number; text: string } | null = null;
    for (const l of lines()) {
      if (field.contains(l.node)) continue;
      if (l.right <= lrect.left || l.left >= lrect.right) continue;
      if (l.baseline > capTop) continue;
      if (!nearest || l.baseline > nearest.baseline) nearest = { baseline: l.baseline, text: l.node.textContent!.trim().slice(0, 30) };
    }
    if (nearest && capTop - nearest.baseline < MIN_LABEL_GAP) {
      issues.push({
        kind: 'label-touch',
        where: `<${field.tagName.toLowerCase()}> "${(field.textContent || '').replace(/\s+/g, ' ').trim().slice(0, 50)}"`,
        detail: `the label "${label.textContent!.trim()}" is ${(capTop - nearest.baseline).toFixed(1)}px under "${nearest.text}" (at least ${MIN_LABEL_GAP}px)`,
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
