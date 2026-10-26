import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  CombatantKind,
  CombatantSchema,
  CombatantState,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import { ActionOptionSchema } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { CombatClient } from '../../../../core/combat/combat-client';
import { fakeCombatClient } from '../../../../core/combat/combat-client-testing';
import { CombatState } from '../../../../core/combat/combat-state';
import { encounter } from '../../../../core/combat/combat-testing';
import { TableActionSheet } from './table-action-sheet';

/** An invented table feature: spends 1 to 3 points of a pool and heals 1d6 per point; `bare` has no result at all. */
const option = (
  over: { costMax?: number; heal?: boolean; tempHp?: number; usesLeft?: number } = {},
) =>
  create(ActionOptionSchema, {
    action: {
      key: 'feature:folego-da-mesa',
      namePt: 'Fôlego da Mesa',
      resourceKey: 'reserva_de_teste',
      costMax: over.costMax ?? 3,
      heal: over.heal === false ? undefined : { count: 1, sides: 6 },
      healPerPoint: over.heal === false ? undefined : { count: 1, sides: 6 },
      tempHp: over.tempHp ?? 0,
    },
    usesLeft: over.usesLeft ?? 5,
    enabled: true,
  });

const lira = create(CombatantSchema, {
  id: 'lira',
  label: 'Lira',
  kind: CombatantKind.PLAYER,
  state: CombatantState.HURT,
});
const goblin = create(CombatantSchema, { id: 'gob', label: 'Goblin', kind: CombatantKind.NPC });
const hidden = create(CombatantSchema, {
  id: 'sombra',
  label: 'Sombra',
  kind: CombatantKind.PLAYER,
  hidden: true,
});

describe('TableActionSheet: "Quantos?", who gets it, and the roll', () => {
  function open(
    opt = option(),
    takeAction = vi
      .fn()
      .mockResolvedValue({ encounter: encounter({ id: 'e1' }), healed: 9, spent: 3 }),
  ) {
    const close = vi.fn();
    TestBed.configureTestingModule({
      providers: [
        { provide: CombatClient, useValue: fakeCombatClient({ takeAction }) },
        { provide: MatDialogRef, useValue: { close } },
        {
          provide: MAT_DIALOG_DATA,
          useValue: {
            campaignId: 'c1',
            encounterId: 'e1',
            combatantId: 'me',
            option: opt,
            combatants: [
              create(CombatantSchema, { id: 'me', label: 'Toren', kind: CombatantKind.PLAYER }),
              lira,
              goblin,
              hidden,
            ],
            diceMode: DiceMode.PLAYERS_CHOOSE,
            preference: DicePreference.APP,
            state: new CombatState(),
          },
        },
      ],
    });
    const fixture = TestBed.createComponent(TableActionSheet);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const btn = (label: string | RegExp) =>
      [...el.querySelectorAll<HTMLButtonElement>('button')].find((b) =>
        label instanceof RegExp
          ? label.test(b.getAttribute('aria-label') ?? b.textContent ?? '')
          : (b.getAttribute('aria-label') ?? b.textContent)?.trim() === label,
      )!;
    const click = (b: HTMLElement) => {
      b.click();
      fixture.detectChanges();
    };
    return { fixture, el, btn, click, takeAction, close };
  }

  it('asks "Quantos?" with the range and says in a live region what it costs and heals', () => {
    const { el } = open();
    expect(el.textContent).toContain('Quantos?');
    expect(el.textContent).toContain('de 1 a 3');
    expect(el.querySelector('[role="status"]')?.textContent?.trim()).toContain(
      'Gasta 1 · cura 1d6',
    );
  });

  it('stops the range at what is left', () => {
    expect(open(option({ usesLeft: 2 })).el.textContent).toContain('de 1 a 2');
  });

  it('offers self first and the visible living allies, never a creature or a hidden one', () => {
    const { el } = open();
    const radios = [...el.querySelectorAll<HTMLInputElement>('input[type="radio"]')];
    expect(radios).toHaveLength(2);
    expect(radios[0].checked).toBe(true);
    expect(el.textContent).toContain('Você');
    expect(el.textContent).toContain('Lira');
    expect(el.textContent).not.toContain('Goblin');
    expect(el.textContent).not.toContain('Sombra');
  });

  it('sends the number picked, the ally picked and the app roll', async () => {
    const { btn, click, takeAction, el, fixture } = open();
    click(btn('Gastar um a mais'));
    click(btn('Gastar um a mais'));
    const radios = [...el.querySelectorAll<HTMLInputElement>('input[type="radio"]')];
    radios[1].click();
    fixture.detectChanges();
    click(btn(/Rolar 3d6 no app/));
    await fixture.whenStable();
    const args = takeAction.mock.calls[0];
    expect(args.slice(0, 5)).toEqual(['c1', 'e1', 'me', 'feature:folego-da-mesa', { inApp: true }]);
    expect(args[8]).toBe(3);
    expect(args[9]).toBe('lira');
  });

  it('sends no target for self, and shows what came back', async () => {
    const { btn, click, takeAction, el, fixture } = open();
    click(btn(/no app/));
    await fixture.whenStable();
    fixture.detectChanges();
    expect(takeAction.mock.calls[0][9]).toBe('');
    expect(el.textContent).toContain('9 PV recuperados');
  });

  it('shows no picker when the action gives no result, and no stepper for a fixed cost', () => {
    const { el } = open(option({ heal: false, costMax: 0, tempHp: 0 }));
    expect(el.querySelector('input[type="radio"]')).toBeNull();
    expect(el.textContent).not.toContain('Quantos?');
  });

  it('uses a plain "Usar" with no dice for temporary hit points only', async () => {
    const { btn, click, takeAction, fixture } = open(
      option({ heal: false, costMax: 0, tempHp: 4 }),
    );
    click(btn('Usar'));
    await fixture.whenStable();
    expect(takeAction.mock.calls[0][4]).toBeUndefined();
    expect(takeAction.mock.calls[0][8]).toBe(0);
  });

  it('cannot be closed while the use is on its way', async () => {
    let answer!: (r: unknown) => void;
    const { btn, click, fixture, close } = open(
      option(),
      vi.fn(() => new Promise((r) => (answer = r))),
    );
    click(btn(/no app/));
    (fixture.componentInstance as unknown as { close(): void }).close();
    expect(close).not.toHaveBeenCalled();
    answer({ encounter: encounter({ id: 'e1' }), healed: 4 });
    await fixture.whenStable();
  });
});
