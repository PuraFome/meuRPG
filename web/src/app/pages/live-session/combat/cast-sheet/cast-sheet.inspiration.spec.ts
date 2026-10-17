// A spell attack of a character that holds a Bardic Inspiration die: the d20 is kept, the sheet asks about the die
// instead of the result, and the answer brings the cast.
import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { SpellCastSchema } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { ActionEconomy } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { CombatClient } from '../../../../core/combat/combat-client';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { SpellCatalog } from '../../../../core/combat/spell-catalog';
import { inspirationOffer } from '../../../../core/play/scene-testing';
import { ResourceClient } from '../../../../core/resources/resources-client';
import { CastSheet, type CastSheetData } from './cast-sheet';

describe('the cast sheet and the Bardic Inspiration question', () => {
  function setup() {
    TestBed.resetTestingModule();
    const apply = vi.fn();
    const resources = { answerOutsideInspiration: vi.fn() };
    const data = {
      campaignId: 'c',
      encounterId: 'enc',
      casterId: 's',
      round: 1,
      spellKey: 'spell:guiding-bolt',
      name: 'Raio Guiador',
      level: 1,
      concentration: false,
      cantripDice: '',
      economy: ActionEconomy.ACTION,
      slots: [],
      usage: [],
      pact: null,
      targets: undefined,
      shieldFree: null,
      shieldName: '',
      attackBonus: 5,
      diceMode: DiceMode.PLAYERS_CHOOSE,
      preference: DicePreference.APP,
      state: {
        encounter: signal(encounter({ combatants: [combatant({ id: 't', label: 'Goblin' })] })),
        apply,
      },
    } as unknown as CastSheetData;
    TestBed.configureTestingModule({
      providers: [
        { provide: CombatClient, useValue: {} },
        { provide: ResourceClient, useValue: resources },
        { provide: SpellCatalog, useValue: { details: () => Promise.resolve(null) } },
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close: vi.fn() } },
      ],
    });
    const fixture = TestBed.createComponent(CastSheet);
    fixture.detectChanges();
    const sheet = fixture.componentInstance as unknown as {
      offer: { set(v: unknown): void };
      cast: () => unknown;
      answerDie(use: boolean, die: { inApp: true } | null): Promise<void>;
    };
    return { fixture, el: fixture.nativeElement as HTMLElement, sheet, resources, apply };
  }

  it('shows the question in place of the form while the d20 waits, with no result', async () => {
    const { fixture, el, sheet } = setup();
    sheet.offer.set(inspirationOffer(4, 5));
    fixture.detectChanges();
    expect(el.textContent).toContain('Usar a Inspiração de Bardo (d8)?');
    expect(el.textContent).not.toContain('Voltar à sua vez');
  });

  it('casts the spell with the answer, applying the combat it brings', async () => {
    const { fixture, sheet, resources, apply } = setup();
    sheet.offer.set(inspirationOffer(4, 5));
    fixture.detectChanges();
    const enc = encounter({});
    resources.answerOutsideInspiration.mockResolvedValue({
      result: {
        case: 'spellCast',
        value: {
          encounter: enc,
          cast: create(SpellCastSchema, { spellKey: 'spell:guiding-bolt' }),
        },
      },
    });
    await sheet.answerDie(true, { inApp: true });
    expect(resources.answerOutsideInspiration.mock.calls[0].slice(0, 4)).toEqual([
      'c',
      'h1',
      true,
      { inApp: true },
    ]);
    expect(apply).toHaveBeenCalledWith(enc);
    expect(sheet.cast()).not.toBeNull();
  });

  it('keeps the die: the answer sends no roll, and the cast is told without it', async () => {
    const { fixture, el, sheet, resources } = setup();
    sheet.offer.set(inspirationOffer(4, 5));
    fixture.detectChanges();
    resources.answerOutsideInspiration.mockResolvedValue({
      result: {
        case: 'spellCast',
        value: {
          encounter: encounter({}),
          cast: create(SpellCastSchema, { spellKey: 'spell:guiding-bolt' }),
        },
      },
    });
    Array.from(el.querySelectorAll('button'))
      .find((b) => b.textContent?.includes('Guardar o dado'))!
      .click();
    await fixture.whenStable();
    fixture.detectChanges();
    expect(resources.answerOutsideInspiration.mock.calls[0].slice(0, 4)).toEqual([
      'c',
      'h1',
      false,
      null,
    ]);
    expect(sheet.cast()).not.toBeNull();
    expect(el.textContent).not.toContain('Usar a Inspiração de Bardo (d8)?');
  });
});
