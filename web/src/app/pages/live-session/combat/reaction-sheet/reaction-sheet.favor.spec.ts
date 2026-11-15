import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CombatantKind, ReactionKind } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { CombatClient } from '../../../../core/combat/combat-client';
import { fakeCombatClient } from '../../../../core/combat/combat-client-testing';
import { CombatState } from '../../../../core/combat/combat-state';
import { combatant, encounter, reactionWindow } from '../../../../core/combat/combat-testing';
import { FavorClient } from '../../../../core/play/favor-client';
import { FavorState } from '../../../../core/play/favor-state';
import { FakeFavorClient, openFavor } from '../../../../core/play/favor-testing';
import { ResourceClient } from '../../../../core/resources/resources-client';
import { RollAnimator } from '../../../../shared/roll-overlay/roll-animator';
import { settle } from '../../../../../test-settle';
import { ReactionSheet, type ReactionSheetData } from './reaction-sheet';

const plain = (text: string | null | undefined) => (text ?? '').replace(/\s+/g, ' ').trim();

function concentration(prompt: object = {}) {
  return reactionWindow({
    id: 'w1',
    kind: ReactionKind.CONCENTRATION_SAVE,
    reactorId: 'me',
    reactorLabel: 'Brisa',
    reactorIsPlayer: true,
    prompt: {
      case: 'concentrationSave',
      value: {
        spellNamePt: 'Bênção',
        damageTaken: 8,
        dc: 10,
        sourceLabel: 'Cimitarra do Goblin',
        saveBonus: 2,
        bonusKnown: true,
        mode: 'normal',
        modeSourcesPt: [],
        ...prompt,
      },
    } as never,
  } as never);
}

describe('ReactionSheet: the Inspiração on the concentration save', () => {
  const api = {
    answerReaction: vi.fn(),
    resolveConcentrationSave: vi.fn(),
    markReactionRolling: vi.fn(),
    get: vi.fn(),
  };

  function setup(prompt: object = {}, holders: Record<string, string> = { 'char-me': 'Brisa' }) {
    const window = concentration(prompt);
    const state = new CombatState();
    state.apply(
      encounter({
        reactionWindows: [window],
        combatants: [combatant({ id: 'me', label: 'Brisa', kind: CombatantKind.PLAYER })],
      } as never),
    );
    state.noteReactionOpened(window.id);
    const data: ReactionSheetData = {
      campaignId: 'camp',
      encounterId: 'enc',
      window,
      round: 2,
      usage: [],
      pact: null,
      state,
      armorClass: 15,
      diceMode: DiceMode.PLAYERS_CHOOSE,
      preference: DicePreference.APP,
    };
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({
      providers: [
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close: vi.fn(), disableClose: true } },
        { provide: CombatClient, useValue: fakeCombatClient(api) },
        { provide: ResourceClient, useValue: { answerHeldRoll: vi.fn() } },
        { provide: RollAnimator, useValue: { play: vi.fn() } },
        { provide: FavorClient, useValue: new FakeFavorClient().asClient() },
      ],
    });
    openFavor(TestBed.inject(FavorState), holders);
    const fixture = TestBed.createComponent(ReactionSheet);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const button = (label: string) =>
      Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) =>
        plain(b.textContent).includes(label),
      )!;
    const box = () => el.querySelector<HTMLInputElement>('app-favor-spend input[type=checkbox]');
    return { fixture, el, button, box };
  }

  beforeEach(() => {
    for (const fn of Object.values(api)) {
      fn.mockReset();
    }
    api.markReactionRolling.mockImplementation(async () => encounter({} as never));
    api.resolveConcentrationSave.mockResolvedValue({
      encounter: encounter({} as never),
      result: undefined,
    });
  });

  it('shows no box when the character has none', () => {
    expect(setup({}, {}).box()).toBeNull();
  });

  it('sends spend_favor when ticked and nothing about it when not', async () => {
    const off = setup();
    off.button('Rolar no app').click();
    await settle(off.fixture);
    expect(api.resolveConcentrationSave.mock.calls[0][3]).toEqual({
      kind: 'app',
      maneuverKey: undefined,
    });

    api.resolveConcentrationSave.mockClear();
    const on = setup();
    on.box()!.click();
    on.fixture.detectChanges();
    expect(plain(on.el.textContent)).toContain('Desmarque a caixa');
    on.button('Rolar no app').click();
    await settle(on.fixture);
    expect(api.resolveConcentrationSave.mock.calls[0][3]).toMatchObject({
      kind: 'app',
      spendFavor: true,
    });
  });

  it('is dashed with the reason when the save already has advantage', () => {
    const s = setup({ mode: 'advantage', modeSourcesPt: ['Ajuda de Tor: vantagem'] });
    expect(s.box()!.getAttribute('aria-disabled')).toBe('true');
    expect(plain(s.el.textContent)).toContain('ela já tem vantagem (Ajuda de Tor)');
  });

  it('unticks the box after the spend and reads who has the Inspiração again', async () => {
    const s = setup();
    const refresh = vi.spyOn(TestBed.inject(FavorState), 'refresh').mockResolvedValue();
    s.box()!.click();
    s.fixture.detectChanges();
    s.button('Rolar no app').click();
    await settle(s.fixture);
    expect(refresh).toHaveBeenCalledTimes(1);
  });
});
