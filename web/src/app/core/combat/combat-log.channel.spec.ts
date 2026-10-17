import { type MessageInitShape, create } from '@bufbuild/protobuf';

import {
  type CombatLogEntry,
  CombatLogEntrySchema,
  CombatLogKind,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { logLine } from './combat-log';

const entry = (over: MessageInitShape<typeof CombatLogEntrySchema>): CombatLogEntry =>
  create(CombatLogEntrySchema, {
    id: 'x',
    kind: CombatLogKind.ACTION,
    actorLabel: 'Brisa',
    ...over,
  });

describe('the Canalizar Divindade lines of the log', () => {
  it('lists the hit points Preservar a Vida gave each creature', () => {
    const line = logLine(
      entry({
        key: 'feature:channel-divinity-preserve-life',
        channelDivinity: {
          heals: [{ label: 'Toren', asked: 20, healed: 19, hitPointsAfter: 24 }],
        },
      }),
    );
    expect(line?.notes).toEqual(['Toren: recuperou 19 PV (pedido 20), agora com 24 PV']);
  });

  it('lists the save of each undead and who was expulso', () => {
    const line = logLine(
      entry({
        key: 'feature:channel-divinity-turn-undead',
        channelDivinity: {
          saveDc: 14,
          turns: [
            { label: 'Zumbi A', total: 9, turned: true },
            { label: 'Zumbi B', total: 17, turned: false },
          ],
        },
      }),
    );
    expect(line?.notes).toEqual([
      'Zumbi A: Sabedoria 9 contra CD 14, foi expulso',
      'Zumbi B: Sabedoria 17 contra CD 14, resistiu',
    ]);
  });

  it('adds no note to an action that channelled nothing', () => {
    expect(logLine(entry({ key: 'standard:dash' }))?.notes).toBeUndefined();
  });
});
