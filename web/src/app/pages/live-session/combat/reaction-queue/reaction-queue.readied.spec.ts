import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';

import {
  CombatantKind,
  ReactionKind,
  ReactionWaitSchema,
  ReadiedActionSchema,
  ReadiedKind,
  ReadiedStatus,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import { CombatClient } from '../../../../core/combat/combat-client';
import { fakeCombatClient } from '../../../../core/combat/combat-client-testing';
import { CombatState } from '../../../../core/combat/combat-state';
import { combatant, encounter, reactionWindow } from '../../../../core/combat/combat-testing';
import { EffectsClient } from '../../../../core/effects/effects-client';
import { ReactionQueue } from './reaction-queue';

const plain = (text: string | null | undefined) =>
  (text ?? '').replace(/ /g, ' ').replace(/\s+/g, ' ').trim();

const held = (over: Record<string, unknown> = {}) =>
  create(ReadiedActionSchema, {
    id: 'r1',
    triggerText: 'Quando o zumbi passar pela porta',
    kind: ReadiedKind.ATTACK,
    actionKey: 'equipment:longsword',
    actionNamePt: 'Espada longa',
    targetCombatantId: 'z1',
    status: ReadiedStatus.WAITING,
    ...over,
  } as never);

describe("ReactionQueue, a readied action's window (#582, board 582-05)", () => {
  const api = { answerReaction: vi.fn() };

  function setup(
    row: Record<string, unknown> = {},
    window: Record<string, unknown> = {},
    wait = false,
  ) {
    api.answerReaction.mockReset().mockImplementation(async () => ({
      encounter: encounter({ revision: 9 } as never),
      result: undefined,
    }));
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({
      providers: [
        { provide: CombatClient, useValue: fakeCombatClient(api) },
        { provide: EffectsClient, useValue: { rollEffectSave: vi.fn() } },
      ],
    });
    const fixture = TestBed.createComponent(ReactionQueue);
    fixture.componentRef.setInput(
      'encounter',
      encounter({
        combatants: [
          combatant({ id: 'tor', label: 'Tor', kind: CombatantKind.PLAYER, readied: held(row) }),
          combatant({ id: 'z1', label: 'Zumbi 1' }),
        ],
        reactionWindows: [
          reactionWindow({
            id: 'w1',
            kind: ReactionKind.READIED,
            groupId: 'fire-1',
            reactorId: 'tor',
            reactorLabel: 'Tor',
            reactorIsPlayer: true,
            answerNow: true,
            ...window,
          } as never),
        ],
        reactionWait: wait
          ? create(ReactionWaitSchema, {
              titlePt: 'Esperando o mestre',
              detailPt: 'Um ataque preparado espera outras reações',
            })
          : undefined,
      } as never),
    );
    fixture.componentRef.setInput('campaignId', 'camp');
    fixture.componentRef.setInput('state', new CombatState());
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }
  const buttons = (el: HTMLElement) =>
    Array.from(el.querySelectorAll<HTMLButtonElement>('.btns button'));

  it('reads what it holds and the trigger, who it waits for, and "Deixar passar" first', () => {
    const { el } = setup();
    const row = el.querySelector('.row') as HTMLElement;
    expect(plain(row.querySelector('.row__desc')?.textContent)).toBe(
      'Ação preparada: Espada longa no Zumbi 1 · “Quando o zumbi passar pela porta”',
    );
    expect(plain(row.querySelector('.row__status')?.textContent)).toMatch(/^Esperando Tor/);
    expect(buttons(el).map((b) => plain(b.textContent))).toEqual([
      'Deixar passar',
      'Atacar por Tor',
    ]);
    expect(buttons(el).every((b) => !b.classList.contains('btn--off'))).toBe(true);
  });

  it('answers a readied attack with the dice of the app, and the pass with nothing', async () => {
    const { fixture, el } = setup();
    buttons(el)[1].click();
    await fixture.whenStable();
    expect(api.answerReaction).toHaveBeenCalledWith(
      'camp',
      'enc',
      'w1',
      { use: true, die: { inApp: true } },
      expect.any(String),
    );
    api.answerReaction.mockClear();
    buttons(el)[0].click();
    await fixture.whenStable();
    expect(api.answerReaction).toHaveBeenCalledWith(
      'camp',
      'enc',
      'w1',
      { use: false },
      expect.any(String),
    );
  });

  it('says the wait the server writes while another window holds the roll, with both buttons off', () => {
    const { el } = setup({}, { answerNow: false }, true);
    expect(plain(el.querySelector('.row')?.textContent)).toContain(
      'Esperando o mestre · Um ataque preparado espera outras reações',
    );
    expect(buttons(el).every((b) => b.classList.contains('btn--off'))).toBe(true);
  });

  it('does not use a move for the player from the queue, and says why', async () => {
    const { fixture, el } = setup({ kind: ReadiedKind.MOVE, moveFt: 25, actionNamePt: '' });
    expect(plain(el.querySelector('.row')?.textContent)).toContain(
      'Mover é pelo mapa: mova Tor com a janela aberta.',
    );
    expect(buttons(el)[1].classList.contains('btn--off')).toBe(true);
    buttons(el)[1].click();
    await fixture.whenStable();
    expect(api.answerReaction).not.toHaveBeenCalled();
  });
});
