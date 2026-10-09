import { TestBed } from '@angular/core/testing';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';
import { Subject } from 'rxjs';

import { DiceMode, DicePreference } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { GroupCheckMemberViewSchema } from '../../../../gen/meurpg/play/v1/contest_types_pb';
import { ContestClient } from '../../../core/combat/contest-client';
import { FakeContestClient, groupCheck, textOf } from '../../../core/combat/contest-testing';
import { GroupCheckCard } from './group-check-card';

const member = (over: object = {}) =>
  create(GroupCheckMemberViewSchema, { characterId: 'b', name: 'Brisa', ...over });

describe('GroupCheckCard', () => {
  let api: FakeContestClient;
  let opened: ReturnType<typeof vi.fn>;

  function setup(view = groupCheck({ youRoll: true, members: [member()] })) {
    api = new FakeContestClient();
    api.group = view;
    opened = vi.fn(() => {
      const done = new Subject<boolean>();
      return { afterClosed: () => done, afterDismissed: () => done };
    });
    TestBed.configureTestingModule({
      providers: [
        { provide: ContestClient, useValue: api.as() },
        { provide: MatDialog, useValue: { open: opened } },
        { provide: MatBottomSheet, useValue: { open: opened } },
      ],
    });
    const fixture = TestBed.createComponent(GroupCheckCard);
    fixture.componentRef.setInput('campaignId', 'c');
    fixture.componentRef.setInput('tick', 0);
    fixture.componentRef.setInput('diceMode', DiceMode.PLAYERS_CHOOSE);
    fixture.componentRef.setInput('preference', DicePreference.APP);
    fixture.detectChanges();
    const settle = async () => {
      for (let i = 0; i < 3; i++) {
        await fixture.whenStable();
        fixture.detectChanges();
      }
    };
    return { fixture, el: fixture.nativeElement as HTMLElement, settle };
  }

  it('opens the sheet by itself once when the master asks the player for a roll, and keeps a card', async () => {
    const { el, settle, fixture } = setup();
    await settle();
    expect(opened).toHaveBeenCalledTimes(1);
    expect(textOf(el)).toContain('Teste em grupo. O mestre pediu um teste de Furtividade de todo o grupo.');
    // The same check never opens the sheet a second time by itself.
    fixture.componentRef.setInput('tick', 1);
    await settle();
    expect(opened).toHaveBeenCalledTimes(1);
    expect(api.calls.filter((c) => c === 'groupCheck')).toHaveLength(2);
  });

  it('brings the sheet back with "Rolar o teste"', async () => {
    const { el, settle } = setup();
    await settle();
    el.querySelector<HTMLButtonElement>('button')!.click();
    expect(opened).toHaveBeenCalledTimes(2);
  });

  it('says "Esperando o mestre" after the roll, with a way back to the sheet', async () => {
    const { el, settle } = setup(
      groupCheck({ youRoll: false, members: [member({ answered: true })] }),
    );
    await settle();
    expect(opened).not.toHaveBeenCalled();
    expect(textOf(el)).toContain('Esperando o mestre. O resultado do grupo aparece quando ele encerrar o teste de Furtividade.');
    expect(textOf(el)).toContain('Abrir a folha');
  });

  it('says nothing for a closed check whose result the master did not show', async () => {
    const { el, settle } = setup(groupCheck({ open: false, youRoll: false }));
    await settle();
    expect(textOf(el)).toBe('');
  });

  it('says the group’s verdict and "Passou" once the master showed the DC, until it is put away', async () => {
    const { el, settle } = setup(
      groupCheck({
        open: false,
        verdictKnown: true,
        groupPassed: true,
        members: [member({ answered: true, passedKnown: true, passed: false })],
      }),
    );
    await settle();
    expect(textOf(el)).toContain('Teste em grupo: Furtividade. Falhou. O grupo passou.');
    el.querySelector<HTMLButtonElement>('button')!.click();
    await settle();
    expect(textOf(el)).toBe('');
  });

  it('shows nothing when the session has no group check', async () => {
    const { el, settle } = setup(null as never);
    api.group = null;
    await settle();
    expect(textOf(el)).toBe('');
    expect(opened).not.toHaveBeenCalled();
  });
});
