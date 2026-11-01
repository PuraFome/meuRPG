import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  EncounterBlockedReason,
  EncounterBlockedSchema,
  ReactionKind,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import { CombatClient } from '../../../../core/combat/combat-client';
import { fakeCombatClient } from '../../../../core/combat/combat-client-testing';
import { CombatState } from '../../../../core/combat/combat-state';
import { encounter, reactionWindow } from '../../../../core/combat/combat-testing';
import { ResourceClient } from '../../../../core/resources/resources-client';
import { ReactionSheet, type ReactionSheetData } from './reaction-sheet';

const plain = (text: string | null | undefined) => (text ?? '').replace(/\s+/g, ' ').trim();

const needsRoll = () =>
  new ConnectError('x', Code.FailedPrecondition, undefined, [
    {
      desc: EncounterBlockedSchema,
      value: create(EncounterBlockedSchema, { reason: EncounterBlockedReason.REACTION_NEEDS_ROLL }),
    },
  ]);

const counter = () =>
  reactionWindow({
    id: 'w1',
    reactorId: 'me',
    reactorLabel: 'Brisa',
    reactorIsPlayer: true,
    kind: ReactionKind.COUNTERSPELL,
    prompt: {
      case: 'counterspell',
      value: {
        casterLabel: 'Mago 1',
        distanceFt: 30,
        slotOptions: [{ level: 3, pact: false, free: 2 }],
      },
    } as never,
  } as never);

describe('ReactionSheet: the Counterspell check follows the campaign’s dice mode (RN-18)', () => {
  const api = {
    answerReaction: vi.fn(),
    resolveConcentrationSave: vi.fn(),
    markReactionRolling: vi.fn(),
    get: vi.fn(),
  };

  function setup(diceMode: DiceMode) {
    const window = counter();
    const state = new CombatState();
    state.apply(encounter({ reactionWindows: [window] } as never));
    state.noteReactionOpened(window.id);
    const data: ReactionSheetData = {
      campaignId: 'camp',
      encounterId: 'enc',
      window,
      round: 2,
      usage: [{ level: 3, total: 2, used: 0 }],
      pact: null,
      state,
      armorClass: 15,
      diceMode,
      preference: DicePreference.APP,
    };
    TestBed.configureTestingModule({
      providers: [
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close: vi.fn(), disableClose: true } },
        { provide: CombatClient, useValue: fakeCombatClient(api) },
        { provide: ResourceClient, useValue: { answerHeldRoll: vi.fn() } },
      ],
    });
    const fixture = TestBed.createComponent(ReactionSheet);
    fixture.detectChanges();
    return fixture;
  }

  const button = (el: HTMLElement, label: string) =>
    Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) =>
      plain(b.textContent).includes(label),
    );
  const flush = async (fixture: { detectChanges(): void; whenStable(): Promise<unknown> }) => {
    await fixture.whenStable();
    fixture.detectChanges();
  };

  beforeEach(() => {
    for (const fn of Object.values(api)) {
      fn.mockReset();
    }
    // The server answers with the combat, the window still open (and now marked).
    api.markReactionRolling.mockImplementation(async () =>
      encounter({ reactionWindows: [{ ...counter(), rolling: true }] } as never),
    );
  });

  async function toRollStep(diceMode: DiceMode) {
    api.answerReaction.mockRejectedValueOnce(needsRoll());
    const fixture = setup(diceMode);
    const el = fixture.nativeElement as HTMLElement;
    button(el, 'Usar Contramágica')!.click();
    await flush(fixture);
    expect(plain(el.textContent)).toContain('faça o teste de habilidade de conjuração');
    return { fixture, el };
  }

  it('follows the campaign’s dice mode: forced app offers only "Rolar no app" and sends no typed value', async () => {
    const { fixture, el } = await toRollStep(DiceMode.APP);
    expect(button(el, 'Digitar o resultado')).toBeUndefined();
    expect(el.querySelector('input[type="text"]')).toBeNull();
    api.answerReaction.mockRejectedValueOnce(new Error('stop'));
    button(el, 'Rolar no app')!.click();
    await flush(fixture);
    expect(api.answerReaction.mock.calls.at(-1)![3]).toEqual({
      use: true,
      slot: { level: 3, pact: false },
      die: { inApp: true },
    });
  });

  it('follows the campaign’s dice mode: forced physical offers only the typed entry and sends the face', async () => {
    const { fixture, el } = await toRollStep(DiceMode.PHYSICAL);
    expect(button(el, 'Rolar no app')).toBeUndefined();
    api.answerReaction.mockRejectedValueOnce(new Error('stop'));
    const field = el.querySelector<HTMLInputElement>('input[type="text"]')!;
    field.value = '9';
    field.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    button(el, 'Confirmar')!.click();
    await flush(fixture);
    expect(api.answerReaction.mock.calls.at(-1)![3]).toEqual({
      use: true,
      slot: { level: 3, pact: false },
      die: { typed: 9 },
    });
  });
});
