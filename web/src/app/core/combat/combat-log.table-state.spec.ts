import { create } from '@bufbuild/protobuf';

import {
  CombatLogEntrySchema,
  CombatLogStateChangeSchema,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { CombatantStateKind } from '../../../gen/meurpg/play/v1/combat_rolls_pb';
import { stateText } from './combat-log-actions';

const entry = create(CombatLogEntrySchema, { id: 'e', actorLabel: 'Toren' });

describe('the log of a state a table feature defines', () => {
  it('calls the state by the name of its feature and says how it ended', () => {
    const on = create(CombatLogStateChangeSchema, {
      kind: CombatantStateKind.TABLE,
      key: 'f#postura',
      namePt: 'Postura do Salgueiro',
      started: true,
    });
    const off = create(CombatLogStateChangeSchema, {
      kind: CombatantStateKind.TABLE,
      key: 'f#postura',
      namePt: 'Postura do Salgueiro',
      reason: 'incapacitated',
    });
    expect(stateText(entry, on)).toBe('Postura do Salgueiro de Toren começou');
    expect(stateText(entry, off)).toBe('Postura do Salgueiro de Toren acabou (ficou incapacitado)');
  });
});
