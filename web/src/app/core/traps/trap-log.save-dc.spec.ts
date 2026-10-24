import { create } from '@bufbuild/protobuf';

import { SaveOutcome, TrapFiringSchema } from '../../../gen/meurpg/play/v1/combat_pb';
import { TrapActivitySchema } from '../../../gen/meurpg/play/v1/traps_pb';
import { activityLines, trapLogText } from './trap-log';

const firing = (dc: number | undefined) =>
  create(TrapFiringSchema, {
    name: 'Estátua de Fogo',
    caught: [
      {
        targetId: 'b',
        targetLabel: 'Brisa',
        saves: [
          {
            outcome: SaveOutcome.FAILED,
            dc,
            roll: { diceCount: 1, diceSides: 20, faces: [9], modifier: 4, total: 13 },
          },
        ],
      },
    ],
  });

describe('the DC of a trap save in the log', () => {
  it('shows "CD 12" to the rolling player when the server sent it', () => {
    expect(trapLogText(firing(12), false)).toContain('contra CD 12');
  });

  it('shows it to the master', () => {
    expect(trapLogText(firing(12), true)).toContain('contra CD 12');
  });

  it('shows no DC when the server sent none or 0, whoever reads', () => {
    for (const master of [false, true]) {
      expect(trapLogText(firing(undefined), master)).not.toMatch(/CD/);
      expect(trapLogText(firing(0), master)).not.toMatch(/CD/);
    }
  });

  it('shows it in the list of trap activity too, only when present', () => {
    const line = (dc: number | undefined) =>
      activityLines(create(TrapActivitySchema, { id: 'a', firing: firing(dc) }), false)
        .map((l) => l.text)
        .join(' ');
    expect(line(12)).toContain('contra CD 12');
    expect(line(undefined)).not.toMatch(/CD/);
  });
});
