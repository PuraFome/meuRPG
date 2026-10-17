import { create } from '@bufbuild/protobuf';

import { DiceRollSchema } from '../../../gen/meurpg/play/v1/combat_pb';
import { rerollShow } from './held-roll-shows';

const d20 = (face: number, rerolled: { from: number; to: number; sourceKey: string }[] = []) =>
  create(DiceRollSchema, {
    diceCount: 1,
    diceSides: 20,
    faces: [face],
    rerolled: rerolled.map((r) => ({ index: 0, ...r })),
  });

describe('the d20 Lucky rolled again, tumbling', () => {
  const rolled = d20(14, [{ from: 1, to: 14, sourceKey: 'trait:lucky' }]);

  it('shows the new face alone, "1 → 14", when the app rolled it', () => {
    const show = rerollShow(rolled, { inApp: true });
    expect(show?.line).toBe('1 → 14');
    expect(show).not.toBeNull();
  });

  it('shows nothing for a typed face, the 1 kept, a plain roll, another feature’s reroll or no roll', () => {
    expect(rerollShow(rolled, { face: 14 })).toBeNull();
    expect(rerollShow(rolled, null)).toBeNull();
    expect(rerollShow(d20(14), { inApp: true })).toBeNull();
    expect(rerollShow(d20(14, [{ from: 1, to: 14, sourceKey: '' }]), { inApp: true })).toBeNull();
    expect(rerollShow(undefined, { inApp: true })).toBeNull();
  });
});
