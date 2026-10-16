import { type Box, labelBounds, placeLabel, unionBox } from './map-geometry';

/** The gap between a label and the edge of the image or of the view, in pixels. */
const LABEL_EDGE_PX = 4;

/** Puts every label beside what it names, off the other labels, areas, markers and tokens, and on the image: measured on screen, written straight to the pills (the place is layout, not state). */
export function placeLabels(el: HTMLElement): void {
  const img = el.querySelector('.mv__img')?.getBoundingClientRect();
  if (!img || img.width === 0) {
    return;
  }
  const view = el.getBoundingClientRect();
  const { minLeft, maxRight } = labelBounds(img, view);
  const bounds = {
    minLeft,
    maxRight,
    minTop: Math.max(img.top, view.top) + LABEL_EDGE_PX,
    maxBottom: Math.min(img.bottom, view.bottom) - LABEL_EDGE_PX,
  };
  const boxOf = (node: Element): Box | null => {
    const r = node.getBoundingClientRect();
    return r.width > 0 && r.height > 0
      ? { left: r.left, top: r.top, right: r.right, bottom: r.bottom }
      : null;
  };
  const present = (list: (Box | null)[]): Box[] => list.filter((b): b is Box => b !== null);
  // Everything a label must stay off: the drawn marks (not the hit areas), and then the labels placed so far.
  const marks = Array.from(
    el.querySelectorAll<HTMLElement>('.pt__shape:not(.pt__shape--pin), .tk__disc, .area, .pin'),
  );
  const placed: Box[] = [];
  for (const label of Array.from(el.querySelectorAll<HTMLElement>('.lbl'))) {
    const pill = label.querySelector<HTMLElement>('.lbl__pill');
    const ids = (label.getAttribute('data-ids') ?? '').split(' ');
    if (!pill || ids.length === 0) {
      continue;
    }
    const own = (node: HTMLElement): boolean => {
      const pin = node.getAttribute('data-pin-of');
      const marker = node.closest('app-map-marker')?.getAttribute('data-point');
      return (pin !== null && ids.includes(pin)) || (marker != null && ids.includes(marker));
    };
    const anchor = unionBox(present(marks.filter(own).map((m) => boxOf(m))));
    const origin = label.getBoundingClientRect();
    const size = pill.getBoundingClientRect();
    const around = anchor ?? {
      left: origin.left - 2,
      top: origin.top - 2,
      right: origin.left + 2,
      bottom: origin.top + 2,
    };
    const others = [...present(marks.filter((m) => !own(m)).map((m) => boxOf(m))), ...placed];
    const at = placeLabel(around, { width: size.width, height: size.height }, others, bounds);
    pill.style.setProperty('--lbl-x', `${at.left - origin.left}px`);
    pill.style.setProperty('--lbl-y', `${at.top - origin.top}px`);
    label.setAttribute('data-placed', '');
    placed.push({
      left: at.left,
      top: at.top,
      right: at.left + size.width,
      bottom: at.top + size.height,
    });
  }
}
