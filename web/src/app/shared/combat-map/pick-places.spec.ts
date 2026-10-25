import { type PickPlanInput, planPick } from './pick-places';

const UNIT = 20;
const base = (over: Partial<PickPlanInput>): PickPlanInput => ({
  unit: UNIT,
  columns: 30,
  rows: 20,
  tokens: [],
  tags: [],
  why: null,
  ...over,
});
const tag = (id: string, far = 0) => ({
  id,
  mark: 62,
  far,
  farShort: far ? 56 : 0,
  hasFar: far > 0,
});

describe('planPick: the labels of the target choice', () => {
  it('puts a creature’s tags on its right, and on its left at the right edge', () => {
    const plan = planPick(
      base({
        tokens: [
          { id: 'a', col: 5, row: 5, span: 1 },
          { id: 'b', col: 28, row: 5, span: 1 },
        ],
        tags: [tag('a'), tag('b')],
      }),
    );
    expect(plan.tags.get('a')?.side).toBe('right');
    expect(plan.tags.get('b')?.side).toBe('left');
  });

  it('never puts a tag over another creature’s ring: it goes to the other side', () => {
    // b stands where a’s right-hand tag would be.
    const plan = planPick(
      base({
        tokens: [
          { id: 'a', col: 5, row: 5, span: 1 },
          { id: 'b', col: 7, row: 5, span: 1 },
        ],
        tags: [tag('a', 140)],
      }),
    );
    expect(plan.tags.get('a')?.side).not.toBe('right');
  });

  it('keeps the tags of two creatures apart, and writes "Longe" short when the whole does not fit', () => {
    const plan = planPick(
      base({
        columns: 10,
        rows: 10,
        tokens: [
          { id: 'a', col: 3, row: 4, span: 1 },
          { id: 'b', col: 4, row: 5, span: 1 },
        ],
        tags: [tag('a', 140), tag('b', 140)],
      }),
    );
    const a = plan.tags.get('a')!;
    const b = plan.tags.get('b')!;
    expect(a.side === b.side && a.y === b.y).toBe(false);
  });

  it('puts the reason chip in free map, and gives none when there is no room', () => {
    const open = planPick(
      base({
        tokens: [{ id: 'g2', col: 5, row: 5, span: 1 }],
        why: { id: 'g2', w: 200, h: 30 },
      }),
    );
    expect(open.why).not.toBeNull();
    const tiny = planPick(
      base({
        columns: 4,
        rows: 2,
        tokens: [{ id: 'g2', col: 1, row: 0, span: 1 }],
        why: { id: 'g2', w: 200, h: 30 },
      }),
    );
    expect(tiny.why).toBeNull();
  });
});
