import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';
import { TestBed } from '@angular/core/testing';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';

import {
  CombatLogEffectChange,
  CombatLogEffectSchema,
  type CombatLogEntry,
  CombatLogEntrySchema,
  CombatLogKind,
  CombatLogRoundSchema,
  CombatantKind,
  EncounterBlockedReason,
  EncounterBlockedSchema,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import { CombatClient } from '../../../../core/combat/combat-client';
import { CombatLogState } from '../../../../core/combat/combat-log-state';
import { CombatState } from '../../../../core/combat/combat-state';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { CombatLogPanel } from './combat-log-panel';

describe('CombatLogPanel: the error of the undo', () => {
  it('is said by a status region that was there first, the notice without a role', () => {
    TestBed.configureTestingModule({
      providers: [
        { provide: CombatClient, useValue: {} },
        { provide: MatDialog, useValue: { open: vi.fn() } },
        { provide: MatBottomSheet, useValue: { open: vi.fn() } },
      ],
    });
    const fixture = TestBed.createComponent(CombatLogPanel);
    fixture.componentRef.setInput('log', new CombatLogState());
    fixture.componentRef.setInput('encounter', encounter({ round: 1 }));
    fixture.componentRef.setInput('campaignId', 'c');
    fixture.componentRef.setInput('state', new CombatState());
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const live = el.querySelector('.mr-live[role="status"]')!;
    expect(live.textContent?.trim()).toBe('');
    (fixture.componentInstance as unknown as { error: { set(v: string): void } }).error.set(
      'Outra pessoa agiu primeiro.',
    );
    fixture.detectChanges();
    const notice = live.querySelector('.mr-notice--warning')!;
    expect(notice.textContent).toContain('Outra pessoa agiu primeiro.');
    expect(notice.hasAttribute('role')).toBe(false);
  });

  it('names who changed when the undo is refused because hit points changed, and says nothing was undone', () => {
    TestBed.configureTestingModule({
      providers: [
        { provide: CombatClient, useValue: {} },
        { provide: MatDialog, useValue: { open: vi.fn() } },
        { provide: MatBottomSheet, useValue: { open: vi.fn() } },
      ],
    });
    const fixture = TestBed.createComponent(CombatLogPanel);
    fixture.componentRef.setInput('log', new CombatLogState());
    fixture.componentRef.setInput(
      'encounter',
      encounter({ combatants: [combatant({ id: 'g1', label: 'Goblin 1' })] }),
    );
    fixture.componentRef.setInput('campaignId', 'c');
    fixture.componentRef.setInput('state', new CombatState());
    const refusal = new ConnectError('changed', Code.FailedPrecondition, undefined, [
      {
        desc: EncounterBlockedSchema,
        value: create(EncounterBlockedSchema, {
          reason: EncounterBlockedReason.UNDO_TARGET_CHANGED,
          combatantIds: ['g1'],
        }),
      },
    ]);
    const panel = fixture.componentInstance as unknown as { undoRefusal(e: unknown): string };
    expect(panel.undoRefusal(refusal)).toMatch(/Goblin 1 mudaram.*Nada foi desfeito/);
    expect(panel.undoRefusal(new ConnectError('x', Code.Aborted))).toBe(
      'Outra ação aconteceu antes: confira o registro.',
    );
  });
});

describe('CombatLogPanel: the notice of a concentration that ended with no hit', () => {
  const ended = (id: string) =>
    create(CombatLogEntrySchema, {
      id,
      kind: CombatLogKind.EFFECT,
      effect: create(CombatLogEffectSchema, {
        sourceKey: 'spell:branding-smite',
        sourceNamePt: 'Clarão na Lâmina',
        targetLabels: ['Brisa'],
        change: CombatLogEffectChange.ENDED,
        reason: 'concentration',
        unused: true,
      }),
    });

  function open(inputs: { master?: boolean; ownLabel?: string }) {
    TestBed.configureTestingModule({
      providers: [
        { provide: CombatClient, useValue: {} },
        { provide: MatDialog, useValue: { open: vi.fn() } },
        { provide: MatBottomSheet, useValue: { open: vi.fn() } },
      ],
    });
    const log = new CombatLogState();
    const fixture = TestBed.createComponent(CombatLogPanel);
    fixture.componentRef.setInput('log', log);
    fixture.componentRef.setInput(
      'encounter',
      encounter({
        id: 'enc',
        round: 1,
        combatants: [combatant({ id: 'p1', label: 'Brisa', kind: CombatantKind.PLAYER })],
      }),
    );
    fixture.componentRef.setInput('campaignId', 'c');
    fixture.componentRef.setInput('state', new CombatState());
    fixture.componentRef.setInput('master', inputs.master ?? false);
    fixture.componentRef.setInput('ownLabel', inputs.ownLabel ?? '');
    fixture.detectChanges();
    const feed = (...entries: CombatLogEntry[]) => {
      log.rounds.set([create(CombatLogRoundSchema, { round: 1, entries })]);
      log.loaded.set(true);
      fixture.detectChanges();
    };
    return { fixture, feed, el: fixture.nativeElement as HTMLElement };
  }

  const notice = (el: HTMLElement) => el.querySelector('[data-testid="lapse-notice"]');

  it('shows the caster the notice above the log, and keeps the log line', () => {
    const { feed, el } = open({ ownLabel: 'Brisa' });
    feed();
    feed(ended('a'));
    expect(notice(el)?.textContent).toContain('A sua concentração acabou.');
    expect(notice(el)?.textContent?.replace(/ /g, ' ')).toContain(
      'O Clarão na Lâmina acabou sem golpe.',
    );
    expect(el.textContent).toContain('acabou sem golpe: a concentração acabou');
    const section = el.querySelector('section.panel')!;
    expect(
      notice(el)!.compareDocumentPosition(section.querySelector('#log-body')!) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it('shows the master the notice with the caster named', () => {
    const { feed, el } = open({ master: true });
    feed();
    feed(ended('a'));
    expect(notice(el)?.textContent?.replace(/ /g, ' ')).toContain(
      'A concentração de Brisa acabou.',
    );
  });

  it('shows another player the log line alone', () => {
    const { feed, el } = open({ ownLabel: 'Tor' });
    feed();
    feed(ended('a'));
    expect(notice(el)).toBeNull();
    expect(el.textContent).toContain('acabou sem golpe: a concentração acabou');
  });

  it('takes the notice away when it is touched, and does not bring it back with the same log', () => {
    const { feed, fixture, el } = open({ ownLabel: 'Brisa' });
    feed();
    feed(ended('a'));
    (notice(el)!.querySelector('button') as HTMLButtonElement).click();
    fixture.detectChanges();
    expect(notice(el)).toBeNull();
    feed(ended('a'));
    expect(notice(el)).toBeNull();
  });
});
