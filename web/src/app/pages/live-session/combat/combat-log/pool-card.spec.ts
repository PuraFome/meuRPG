import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';

import {
  CombatLogEntrySchema,
  CombatLogKind,
  SpellEffectKind,
  SpellEffectOutcome,
  SpellEffectReason,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import { logGroups } from '../../../../core/combat/combat-log';
import { LogList } from './log-list';

/** Pensantus's Sono as the server sends it to the master: the pool, and every creature's hit points. */
const SLEEP = create(CombatLogEntrySchema, {
  id: 'sono',
  kind: CombatLogKind.SPELL_CAST,
  actorLabel: 'Pensantus',
  keyNamePt: 'Sono',
  spell: {
    slot: { level: 1, pact: false },
    effectKind: SpellEffectKind.POOL,
    effectConditionKey: 'condition:unconscious',
    poolRoll: { diceCount: 5, diceSides: 8, faces: [2, 4, 1, 5, 3], modifier: 0, total: 15 },
    targets: [
      { targetId: 'g1', targetLabel: 'Goblin 1', effect: { outcome: SpellEffectOutcome.AFFECTED, hitPointsBefore: 7, poolLeft: 8, poolOrder: 1 } },
      {
        targetId: 'cap',
        targetLabel: 'Capitão Goblin',
        effect: { outcome: SpellEffectOutcome.NOT_AFFECTED, reason: SpellEffectReason.ABOVE_POOL, hitPointsBefore: 27, poolLeft: 8, poolOrder: 2 },
      },
    ],
  },
});

describe('the master\'s card under Sono in the log (E8-03)', () => {
  function render(master: boolean) {
    const fixture = TestBed.createComponent(LogList);
    const groups = logGroups(
      [{ round: 1, entries: [SLEEP] }] as never,
      1,
      '',
      { master, players: new Set(['Pensantus']) },
    );
    fixture.componentRef.setInput('groups', groups);
    const changed: string[] = [];
    fixture.componentInstance.conditions.subscribe((id) => changed.push(id));
    fixture.detectChanges();
    return { el: fixture.nativeElement as HTMLElement, changed };
  }

  it('shows the dice, each creature with its hit points and the account, the word and the icon', () => {
    const { el } = render(true);
    const card = el.querySelector('app-pool-card')!;
    expect(card.querySelector('.card__roll')!.textContent).toContain('5d8 (2, 4, 1, 5, 3) = 15');
    const rows = [...card.querySelectorAll('.row')].map((r) => [
      r.querySelector('.row__name')!.textContent!.trim(),
      r.querySelector('.row__hp')!.textContent!.trim(),
      r.querySelector('.row__math')!.textContent!.replace(/ /g, ' ').trim(),
      r.querySelector('.row__word mat-icon')!.textContent!.trim(),
      r.querySelector('.row__word')!.textContent!.replace(r.querySelector('.row__word mat-icon')!.textContent!, '').trim(),
    ]);
    expect(rows).toEqual([
      ['Goblin 1', '7 PV', '15 − 7 = 8 restam', 'bedtime', 'Adormeceu · Inconsciente'],
      ['Capitão Goblin', '27 PV', '27 é mais que 8 restantes', 'block', 'Não afetado'],
    ]);
    expect(card.textContent).toContain('Só o mestre vê este quadro');
  });

  it('opens the conditions of the creature that slept from "Mudar as condições"', () => {
    const { el, changed } = render(true);
    const buttons = [...el.querySelectorAll<HTMLButtonElement>('app-pool-card .change')];
    expect(buttons.map((b) => b.textContent!.trim())).toEqual(['Mudar as condições do Goblin 1']);
    buttons[0].click();
    expect(changed).toEqual(['g1']);
  });

  it('has no card at all in a player\'s log, and not a number of an enemy\'s', () => {
    const { el } = render(false);
    expect(el.querySelector('app-pool-card')).toBeNull();
    expect(el.textContent).not.toContain('PV');
    expect(el.textContent).not.toContain('27');
  });
});
