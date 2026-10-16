// An effect's saving throw of a halfling whose d20 shows a 1: the d20 is kept and the window stays open, the sheet asks
// "Tirou 1: rolar de novo?" before the result, and the answer settles the save, or asks about the Bardic Inspiration die
// on the new roll.
import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  LuckyOfferSchema,
  OutsideInspirationOfferSchema,
  ReactionKind,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  EffectPhase,
  EffectSaveResultSchema,
} from '../../../../../gen/meurpg/play/v1/lasting_effects_pb';
import { CombatState } from '../../../../core/combat/combat-state';
import { encounter, reactionWindow } from '../../../../core/combat/combat-testing';
import { LUCKY_TRAIT } from '../../../../core/combat/lucky';
import { EffectsClient } from '../../../../core/effects/effects-client';
import { ResourceClient } from '../../../../core/resources/resources-client';
import { EffectSaveSheet, type EffectSaveSheetData } from './effect-save-sheet';

const plain = (text: string | null | undefined) => (text ?? '').replace(/\s+/g, ' ').trim();

const save = () =>
  reactionWindow({
    id: 'w1',
    kind: ReactionKind.EFFECT_SAVE,
    reactorId: 'me',
    reactorLabel: 'Pipo',
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
        textPt: 'Teste de resistência de Sabedoria. Se passar, o efeito acaba sobre você.',
      },
    } as never,
  } as never);

const lucky = create(LuckyOfferSchema, {
  holdId: 'h1',
  luckyIndex: 0,
  d20: { diceCount: 1, diceSides: 20, faces: [1], modifier: 1, total: 2 },
});

/** What the answer settles: the save with the new d20 (or the 1 kept). */
const settled = (d20: number, again?: number) =>
  create(EffectSaveResultSchema, {
    d20,
    modifier: 1,
    total: d20 + 1,
    saved: d20 + 1 >= 13,
    d20Faces: [d20],
    rerolled: again ? [{ index: 0, from: 1, to: again, sourceKey: LUCKY_TRAIT }] : [],
  });

describe('EffectSaveSheet: the halfling’s Lucky question, "Tirou 1: rolar de novo?"', () => {
  const api = { rollEffectSave: vi.fn() };
  const resources = { answerOutsideLucky: vi.fn(), answerOutsideInspiration: vi.fn() };

  function setup(diceMode = DiceMode.PLAYERS_CHOOSE) {
    const window = save();
    const state = new CombatState();
    state.apply(encounter({ reactionWindows: [window] } as never));
    const data: EffectSaveSheetData = {
      campaignId: 'camp',
      encounterId: 'enc',
      window,
      round: 3,
      state,
      diceMode,
      preference: DicePreference.APP,
    };
    TestBed.configureTestingModule({
      providers: [
        { provide: EffectsClient, useValue: api },
        { provide: ResourceClient, useValue: resources },
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close: vi.fn(), disableClose: false } },
      ],
    });
    const fixture = TestBed.createComponent(EffectSaveSheet);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const button = (label: string) =>
      Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) =>
        plain(b.textContent).includes(label),
      );
    const flush = async () => {
      await fixture.whenStable();
      fixture.detectChanges();
    };
    return { fixture, el, button, flush };
  }

  /** Rolls in the app and gets the question back: the window is still open. */
  async function asked(s: ReturnType<typeof setup>) {
    api.rollEffectSave.mockResolvedValue({
      encounter: encounter({ reactionWindows: [save()] } as never),
      result: undefined,
      lucky,
    });
    s.button('Rolar no app')!.click();
    await s.flush();
  }

  beforeEach(() => {
    api.rollEffectSave.mockReset();
    resources.answerOutsideLucky.mockReset();
    resources.answerOutsideInspiration.mockReset();
  });

  it('asks after a d20 of 1 and before the result, with the d20 and no "Passou" or "Falhou"', async () => {
    const s = setup();
    await asked(s);
    const text = plain(s.el.textContent);
    expect(text).toContain('Tirou 1: rolar de novo?');
    expect(text).toContain(
      'Seu d20: 1d20 (1) + 1 = 2. O resultado só aparece depois da sua escolha.',
    );
    expect(text).not.toMatch(/Passou|Falhou|Deixar o mestre rolar por mim/);
    expect(s.button('Rolar de novo')).toBeTruthy();
    expect(s.button('Ficar com o 1')).toBeTruthy();
    expect(s.button('Fechar')).toBeTruthy();
  });

  it('"Rolar de novo" settles the save with the new d20 and says "Sortudo: 1 → 14"', async () => {
    const s = setup();
    await asked(s);
    resources.answerOutsideLucky.mockResolvedValue({
      result: {
        case: 'effectSave',
        value: { encounter: encounter({ reactionWindows: [] } as never), result: settled(14, 14) },
      },
    });
    s.button('Rolar de novo')!.click();
    await s.flush();
    expect(resources.answerOutsideLucky.mock.calls[0].slice(0, 4)).toEqual([
      'camp',
      'h1',
      true,
      { inApp: true },
    ]);
    const text = plain(s.el.textContent);
    expect(text).not.toContain('Tirou 1: rolar de novo?');
    expect(text).toContain('Passou');
    expect(text).toContain('Sortudo: 1 → 14');
  });

  it('"Ficar com o 1" keeps it: the save fails, and nothing says Lucky rolled', async () => {
    const s = setup();
    await asked(s);
    resources.answerOutsideLucky.mockResolvedValue({
      result: {
        case: 'effectSave',
        value: { encounter: encounter({ reactionWindows: [] } as never), result: settled(1) },
      },
    });
    s.button('Ficar com o 1')!.click();
    await s.flush();
    expect(resources.answerOutsideLucky.mock.calls[0].slice(0, 4)).toEqual([
      'camp',
      'h1',
      false,
      null,
    ]);
    const text = plain(s.el.textContent);
    expect(text).toContain('Falhou');
    expect(text).not.toContain('Sortudo:');
  });

  it('after the new d20, a Bardic Inspiration die the character holds is asked about, on the new roll', async () => {
    const s = setup();
    await asked(s);
    resources.answerOutsideLucky.mockResolvedValue({
      result: {
        case: 'effectSave',
        value: {
          encounter: encounter({ reactionWindows: [save()] } as never),
          inspirationOffer: create(OutsideInspirationOfferSchema, {
            holdId: 'h1',
            sides: 8,
            fromName: 'Orla',
            d20: { diceCount: 1, diceSides: 20, faces: [9], modifier: 1, total: 10 },
          }),
        },
      },
    });
    s.button('Rolar de novo')!.click();
    await s.flush();
    const text = plain(s.el.textContent);
    expect(text).not.toContain('Tirou 1: rolar de novo?');
    expect(text).toContain('Usar a Inspiração de Bardo (d8)?');
    expect(text).not.toMatch(/Passou|Falhou/);
  });

  it('keeps the key of an answer that failed, so a retry never rolls the d20 twice', async () => {
    const s = setup();
    await asked(s);
    resources.answerOutsideLucky.mockRejectedValue(new ConnectError('down', Code.Unavailable));
    s.button('Rolar de novo')!.click();
    await s.flush();
    s.button('Rolar de novo')!.click();
    await s.flush();
    const keys = resources.answerOutsideLucky.mock.calls.map((c) => c[4] as string);
    expect(keys).toHaveLength(2);
    expect(keys[0]).toBe(keys[1]);
    expect(plain(s.el.querySelector('[role=alert] p')?.textContent)).toContain(
      'o servidor não respondeu',
    );
    expect(plain(s.el.textContent)).toContain('Tirou 1: rolar de novo?');
  });
});
