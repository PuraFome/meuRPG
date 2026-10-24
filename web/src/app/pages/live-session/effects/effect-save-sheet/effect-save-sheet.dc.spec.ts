import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { ReactionKind } from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  EffectPhase,
  EffectSaveResultSchema,
} from '../../../../../gen/meurpg/play/v1/lasting_effects_pb';
import { CombatState } from '../../../../core/combat/combat-state';
import { encounter, reactionWindow } from '../../../../core/combat/combat-testing';
import { EffectsClient } from '../../../../core/effects/effects-client';
import { ResourceClient } from '../../../../core/resources/resources-client';
import { EffectSaveSheet, type EffectSaveSheetData } from './effect-save-sheet';

const plain = (text: string | null | undefined) => (text ?? '').replace(/\s+/g, ' ').trim();

function window(dc?: number) {
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
        textPt: 'Teste de resistência de Sabedoria. Se passar, o efeito acaba sobre você.',
        dc,
      },
    } as never,
  } as never);
}

describe('EffectSaveSheet, the save DC', () => {
  const api = { rollEffectSave: vi.fn() };

  function setup(w: ReturnType<typeof window>) {
    const state = new CombatState();
    state.apply(encounter({ reactionWindows: [w] } as never));
    const data: EffectSaveSheetData = {
      campaignId: 'camp',
      encounterId: 'enc',
      window: w,
      round: 3,
      state,
      diceMode: DiceMode.PLAYERS_CHOOSE,
      preference: DicePreference.APP,
    };
    TestBed.configureTestingModule({
      providers: [
        { provide: EffectsClient, useValue: api },
        { provide: ResourceClient, useValue: {} },
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close: vi.fn(), disableClose: false } },
      ],
    });
    const fixture = TestBed.createComponent(EffectSaveSheet);
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  const roll = async (
    fixture: { detectChanges(): void; whenStable(): Promise<unknown> },
    el: HTMLElement,
  ) => {
    Array.from(el.querySelectorAll('button'))
      .find((b) => plain(b.textContent).includes('Rolar no app'))!
      .click();
    await fixture.whenStable();
    fixture.detectChanges();
  };

  const result = (dc?: number) =>
    create(EffectSaveResultSchema, {
      d20: 12,
      modifier: 1,
      total: 13,
      saved: true,
      effectEnded: true,
      textPt: 'Imobilizar Pessoa acabou.',
      dc,
    });

  beforeEach(() => api.rollEffectSave.mockReset());

  it('says "CD 14" in the question when the prompt has the DC', () => {
    const { el } = setup(window(14));
    expect(plain(el.textContent)).toContain(
      'Teste de resistência de Sabedoria, CD 14. Se passar, o efeito acaba sobre você.',
    );
  });

  it('says nothing of a DC in the question when the prompt has none, or 0', () => {
    for (const dc of [undefined, 0]) {
      TestBed.resetTestingModule();
      const { el } = setup(window(dc));
      expect(plain(el.textContent)).toContain(
        'Teste de resistência de Sabedoria. Se passar, o efeito acaba sobre você.',
      );
      expect(plain(el.textContent)).not.toMatch(/\bCD\b/);
    }
  });

  it('says the DC the total was against in the result when the result has it', async () => {
    api.rollEffectSave.mockResolvedValue({
      encounter: encounter({ reactionWindows: [] } as never),
      result: result(14),
    });
    const { fixture, el } = setup(window(14));
    await roll(fixture, el);
    expect(plain(el.querySelector('.res')?.textContent)).toContain('Passou');
    expect(plain(el.querySelector('.res')?.textContent)).toContain('contra CD 14');
  });

  it('says no DC in the result when the result has none', async () => {
    api.rollEffectSave.mockResolvedValue({
      encounter: encounter({ reactionWindows: [] } as never),
      result: result(),
    });
    const { fixture, el } = setup(window());
    await roll(fixture, el);
    expect(plain(el.querySelector('.res')?.textContent)).toContain('Passou');
    expect(plain(el.querySelector('.res')?.textContent)).not.toMatch(/\bCD\b/);
  });
});
