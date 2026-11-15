import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CombatantKind, ReactionKind } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { FavorBlockedReason } from '../../../../../gen/meurpg/play/v1/favor_pb';
import {
  EffectPhase,
  EffectSaveResultSchema,
} from '../../../../../gen/meurpg/play/v1/lasting_effects_pb';
import { CombatState } from '../../../../core/combat/combat-state';
import { combatant, encounter, reactionWindow } from '../../../../core/combat/combat-testing';
import { EffectsClient } from '../../../../core/effects/effects-client';
import { FavorClient } from '../../../../core/play/favor-client';
import { FavorState } from '../../../../core/play/favor-state';
import { FakeFavorClient, favorRefused, openFavor } from '../../../../core/play/favor-testing';
import { ResourceClient } from '../../../../core/resources/resources-client';
import { EffectSaveSheet, type EffectSaveSheetData } from './effect-save-sheet';

const plain = (text: string | null | undefined) => (text ?? '').replace(/\s+/g, ' ').trim();

function save(over: object = {}) {
  return reactionWindow({
    id: 'w1',
    kind: ReactionKind.EFFECT_SAVE,
    reactorId: 'me',
    reactorLabel: 'Brisa',
    reactorIsPlayer: true,
    prompt: {
      case: 'effectSave',
      value: {
        effectId: 'e1',
        sourceNamePt: 'Imobilizar Pessoa',
        ability: 'wis',
        abilityNamePt: 'Sabedoria',
        phase: EffectPhase.END,
        modifier: 1,
        bonusKnown: true,
        mode: 'normal',
        modeSourcesPt: [],
        extraDice: [],
        autoFail: false,
        handedToMaster: false,
        textPt: 'Teste de resistência de Sabedoria.',
        ...over,
      },
    } as never,
  } as never);
}

const passed = create(EffectSaveResultSchema, {
  d20: 12,
  d20Faces: [4, 12],
  modifier: 1,
  total: 13,
  saved: true,
  effectEnded: true,
  textPt: 'Imobilizar Pessoa acabou.',
});

describe('EffectSaveSheet: the Inspiração (a save sheet, 581-07)', () => {
  const api = { rollEffectSave: vi.fn() };

  function setup(over: object = {}, holders: Record<string, string> = { 'char-me': 'Brisa' }) {
    TestBed.resetTestingModule();
    const window = save(over);
    const state = new CombatState();
    state.apply(
      encounter({
        reactionWindows: [window],
        combatants: [combatant({ id: 'me', label: 'Brisa', kind: CombatantKind.PLAYER })],
      } as never),
    );
    const data: EffectSaveSheetData = {
      campaignId: 'camp',
      encounterId: 'enc',
      window,
      round: 3,
      state,
      diceMode: DiceMode.PLAYERS_CHOOSE,
      preference: DicePreference.APP,
    };
    TestBed.configureTestingModule({
      providers: [
        { provide: EffectsClient, useValue: api },
        { provide: ResourceClient, useValue: { answerHeldRoll: vi.fn() } },
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close: vi.fn(), disableClose: false } },
        { provide: FavorClient, useValue: new FakeFavorClient().asClient() },
      ],
    });
    openFavor(TestBed.inject(FavorState), holders);
    const fixture = TestBed.createComponent(EffectSaveSheet);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const button = (label: string) =>
      Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) =>
        plain(b.textContent).includes(label),
      )!;
    const box = () => el.querySelector<HTMLInputElement>('app-favor-spend input[type=checkbox]')!;
    const tick = () => {
      box().click();
      fixture.detectChanges();
    };
    const settle = async () => {
      await fixture.whenStable();
      fixture.detectChanges();
    };
    return { fixture, el, button, box, tick, settle };
  }

  beforeEach(() => {
    api.rollEffectSave.mockReset();
    api.rollEffectSave.mockResolvedValue({
      encounter: encounter({} as never),
      result: passed,
    });
  });

  it('shows no box when the character has none', () => {
    expect(setup({}, {}).el.querySelector('app-favor-spend .favor')).toBeNull();
  });

  it('sends spend_favor when ticked, for the roll in the app, and not otherwise', async () => {
    const off = setup();
    off.button('Rolar no app').click();
    await off.settle();
    expect(api.rollEffectSave.mock.calls[0][3]).toEqual({ kind: 'app' });

    api.rollEffectSave.mockClear();
    const on = setup();
    on.tick();
    expect(plain(on.el.textContent)).toContain('Desmarque a caixa');
    on.button('Rolar no app').click();
    await on.settle();
    expect(api.rollEffectSave.mock.calls[0][3]).toEqual({ kind: 'app', spendFavor: true });
  });

  it('asks the second d20 of a typed save while ticked, and sends the spend with it', async () => {
    const s = setup();
    s.tick();
    s.button('Digitar o resultado').click();
    s.fixture.detectChanges();
    expect(s.el.querySelectorAll('app-multi-roll input')).toHaveLength(2);
    const [a, b] = Array.from(s.el.querySelectorAll<HTMLInputElement>('app-multi-roll input'));
    a.value = '4';
    a.dispatchEvent(new Event('input'));
    b.value = '12';
    b.dispatchEvent(new Event('input'));
    s.fixture.detectChanges();
    s.button('Confirmar').click();
    await s.settle();
    expect(api.rollEffectSave.mock.calls[0][3]).toMatchObject({
      kind: 'typed',
      face: 4,
      second: 12,
      spendFavor: true,
    });
  });

  it('is dashed when the save already has advantage', () => {
    const s = setup({ mode: 'advantage', modeSourcesPt: ['Ajuda de Tor: vantagem'] });
    expect(s.box().getAttribute('aria-disabled')).toBe('true');
    expect(plain(s.el.textContent)).toContain('ela já tem vantagem (Ajuda de Tor)');
  });

  it('does not offer the box on a save that fails by itself or one left to the master', () => {
    expect(setup({ autoFail: true }).el.querySelector('app-favor-spend .favor')).toBeNull();
    expect(setup({ handedToMaster: true }).el.querySelector('app-favor-spend .favor')).toBeNull();
  });

  it('says the refusal in the error area and unticks the box', async () => {
    api.rollEffectSave.mockRejectedValue(favorRefused(FavorBlockedReason.ADDS_NOTHING));
    const s = setup();
    s.tick();
    s.button('Rolar no app').click();
    await s.settle();
    expect(plain(s.el.querySelector('[role=alert]')?.textContent)).toContain(
      'A Inspiração não mudaria esta rolagem.',
    );
    expect(s.box().checked).toBe(false);
  });
});
