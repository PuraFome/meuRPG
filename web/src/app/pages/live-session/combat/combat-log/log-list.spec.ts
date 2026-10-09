import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';

import {
  AttackOutcome,
  CombatLogEntrySchema,
  CombatLogKind,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import { RollMode } from '../../../../../gen/meurpg/play/v1/combat_rolls_pb';
import { logGroups } from '../../../../core/combat/combat-log';
import { LogList } from './log-list';

describe('LogList: the d20 pair and the notes of a line', () => {
  it('draws both faces with the counted one in words, and the mode under the line', () => {
    const attack = create(CombatLogEntrySchema, {
      id: 'a',
      kind: CombatLogKind.ATTACK,
      actorLabel: 'Toren',
      targetLabel: 'Goblin',
      outcome: AttackOutcome.HIT,
      attackRoll: { diceCount: 2, diceSides: 20, faces: [4, 17], total: 22, countedIndex: 1 },
      modeChange: {
        suggestedMode: RollMode.NORMAL,
        mode: RollMode.ADVANTAGE,
        reason: 'ataque descuidado',
      },
    });
    const fixture = TestBed.createComponent(LogList);
    fixture.componentRef.setInput(
      'groups',
      logGroups([{ round: 1, entries: [attack] }] as never, 1, '', {
        master: true,
        players: new Set(),
      }),
    );
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const dice = Array.from(el.querySelectorAll('.die'), (d) =>
      d.textContent?.replace(/\s+/g, ' ').trim(),
    );
    expect(dice).toEqual(['4 não conta', '17 conta']);
    expect(el.querySelector('.die--counts')?.textContent).toContain('17');
    expect(el.querySelector('.line__notes')?.textContent).toContain(
      'Vantagem (sugerido: Normal) — ataque descuidado',
    );
  });

  it('draws a line with a single d20 without a pair', () => {
    const attack = create(CombatLogEntrySchema, {
      id: 'a',
      kind: CombatLogKind.ATTACK,
      actorLabel: 'Toren',
      targetLabel: 'Goblin',
      outcome: AttackOutcome.MISS,
      attackRoll: { diceCount: 1, diceSides: 20, faces: [3], total: 6 },
    });
    const fixture = TestBed.createComponent(LogList);
    fixture.componentRef.setInput(
      'groups',
      logGroups([{ round: 1, entries: [attack] }] as never, 1, '', {
        master: false,
        players: new Set(),
      }),
    );
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelector('.die')).toBeNull();
    expect(el.querySelector('.line__notes')).toBeNull();
  });
});
