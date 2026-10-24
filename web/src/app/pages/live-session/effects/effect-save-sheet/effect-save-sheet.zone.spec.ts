import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { ReactionKind } from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  EffectSavePromptSchema,
  EffectSaveResultSchema,
} from '../../../../../gen/meurpg/play/v1/lasting_effects_pb';
import { CombatState } from '../../../../core/combat/combat-state';
import { encounter, reactionWindow, spellZone } from '../../../../core/combat/combat-testing';
import { EffectsClient } from '../../../../core/effects/effects-client';
import { ResourceClient } from '../../../../core/resources/resources-client';
import { EffectSaveSheet, type EffectSaveSheetData } from './effect-save-sheet';

const plain = (text: string | null | undefined) => (text ?? '').replace(/\s+/g, ' ').trim();

function zoneWindow(over: object = {}) {
  return reactionWindow({
    id: 'w1',
    kind: ReactionKind.EFFECT_SAVE,
    reactorId: 'me',
    reactorLabel: 'Tor',
    reactorIsPlayer: true,
    prompt: {
      case: 'effectSave',
      value: create(EffectSavePromptSchema, {
        effectId: 'z1',
        zoneId: 'z1',
        zoneReason: 'zone_start',
        sourceNamePt: 'Raio Lunar',
        ability: 'con',
        abilityNamePt: 'Constituição',
        modifier: 3,
        bonusKnown: true,
        mode: 'normal',
        textPt: 'Teste de resistência de Constituição. Se falhar, você sofre dano de radiante.',
        damageTypeNamePt: 'radiante',
        halfOnSave: true,
        ...over,
      }),
    } as never,
  } as never);
}

function setup(over: object = {}, diceMode = DiceMode.PLAYERS_CHOOSE) {
  const window = zoneWindow(over);
  const state = new CombatState();
  state.apply(
    encounter({
      reactionWindows: [window],
      zones: [spellZone({ id: 'z1', ownerLabel: 'Bartolo' })],
    } as never),
  );
  const data: EffectSaveSheetData = {
    campaignId: 'camp',
    encounterId: 'enc',
    window,
    round: 3,
    state,
    diceMode,
    preference: DicePreference.APP,
  };
  const rollEffectSave = vi.fn();
  TestBed.configureTestingModule({
    providers: [
      { provide: EffectsClient, useValue: { rollEffectSave } },
      { provide: ResourceClient, useValue: {} },
      { provide: MAT_DIALOG_DATA, useValue: data },
      { provide: MatDialogRef, useValue: { close: vi.fn(), disableClose: false } },
    ],
  });
  const fixture = TestBed.createComponent(EffectSaveSheet);
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement, rollEffectSave };
}

describe('EffectSaveSheet for a spell zone', () => {
  beforeEach(() => TestBed.resetTestingModule());

  it('is titled with the zone and its caster and says why, with no DC for a target player', () => {
    const { el } = setup();
    expect(plain(el.querySelector('h2')?.textContent)).toBe('Raio Lunar de Bartolo');
    expect(plain(el.querySelector('.frame__sub')?.textContent)).toContain(
      'A zona atingiu você no começo do seu turno.',
    );
    const text = plain(el.textContent);
    expect(text).toContain('Teste de resistência de Constituição.');
    expect(text).toContain('Se falhar: sofre dano radiante. Se passar: metade do dano.');
    expect(text).toContain('Raio Lunar, de Bartolo');
    expect(text).toContain('Seu modificador: +3.');
    expect(text).not.toMatch(/\bCD\b|\d+ CD|CD \d/);
  });

  it('writes the DC when the server sent it', () => {
    const { el } = setup({ dc: 14, zoneReason: 'zone_enter' });
    expect(plain(el.textContent)).toContain('Constituição, CD 14.');
    expect(plain(el.querySelector('.frame__sub')?.textContent)).toContain('Você entrou na zona.');
  });

  it('rolls through the same effect save call, the app first', async () => {
    const { el, fixture, rollEffectSave } = setup();
    rollEffectSave.mockResolvedValue({
      encounter: encounter({ zones: [] } as never),
      result: create(EffectSaveResultSchema, {
        d20: 9,
        modifier: 3,
        total: 12,
        saved: false,
        dc: 14,
      }),
    });
    const app = Array.from(el.querySelectorAll('button')).find((b) =>
      plain(b.textContent).includes('Rolar no app'),
    )!;
    app.click();
    await fixture.whenStable();
    fixture.detectChanges();
    expect(rollEffectSave).toHaveBeenCalledWith('camp', 'enc', 'w1', { kind: 'app' }, expect.any(String));
    const text = plain(el.textContent);
    expect(text).toContain('Falhou');
    expect(text).toContain('contra CD 14');
  });

  it('types the d20 when the table rolls physical dice', () => {
    const { el } = setup({}, DiceMode.PHYSICAL);
    expect(plain(el.textContent)).not.toContain('Rolar no app');
    expect(el.querySelector('input')).not.toBeNull();
  });
});
