// A spell attack of a halfling whose d20 shows a 1: the d20 is kept, the sheet asks "Tirou 1: rolar de novo?" instead of
// the result, and the answer brings the cast, or the Bardic Inspiration question on the new roll.
import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  AttackOutcome,
  DiceRollSchema,
  LuckyOfferSchema,
  OutsideInspirationOfferSchema,
  SpellCastSchema,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import { ActionEconomy } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { CombatClient } from '../../../../core/combat/combat-client';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { LUCKY_TRAIT } from '../../../../core/combat/lucky';
import { SpellCatalog } from '../../../../core/combat/spell-catalog';
import { ResourceClient } from '../../../../core/resources/resources-client';
import { CastSheet, type CastSheetData } from './cast-sheet';

const flat = (n: Element | null | undefined) =>
  n?.textContent?.replace(/\s+/g, ' ').trim().replace(/ /g, ' ') ?? '';

/** The spell attack's d20 (+5), and what Lucky rolled again. */
const d20 = (face: number, again?: number) =>
  create(DiceRollSchema, {
    diceCount: 1,
    diceSides: 20,
    faces: [face],
    modifier: 5,
    total: face + 5,
    rerolled: again ? [{ index: 0, from: 1, to: again, sourceKey: LUCKY_TRAIT }] : [],
  });

const lucky = create(LuckyOfferSchema, { holdId: 'h1', d20: d20(1), luckyIndex: 0 });

/** The cast the answer brings: Raio Guiador on the Goblin, with the d20 after the answer. */
const cast = (face: number, again?: number) =>
  create(SpellCastSchema, {
    spellKey: 'spell:guiding-bolt',
    targets: [
      {
        combatantId: 't',
        outcome: face + 5 >= 12 ? AttackOutcome.HIT : AttackOutcome.MISS,
        attackRoll: d20(face, again),
      },
    ],
  });

describe('the cast sheet and the halfling’s Lucky question', () => {
  function setup() {
    TestBed.resetTestingModule();
    const apply = vi.fn();
    const resources = { answerOutsideLucky: vi.fn(), answerOutsideInspiration: vi.fn() };
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
      luckyOffer: { set(v: unknown): void; (): unknown };
      offer: () => unknown;
      cast: () => unknown;
      answerLucky(reroll: boolean, die: { inApp: true } | { face: number } | null): Promise<void>;
    };
    const el = fixture.nativeElement as HTMLElement;
    const ask = () => {
      sheet.luckyOffer.set(lucky);
      fixture.detectChanges();
    };
    return { fixture, el, sheet, resources, apply, ask };
  }

  beforeEach(() => {
    // The sheet scrolls its error into view; the test DOM has no scroll.
    Element.prototype.scrollTo = vi.fn();
  });

  it('shows the question in place of the form while the d20 waits: the 1 is there, no result', () => {
    const { el, ask } = setup();
    ask();
    const text = flat(el);
    expect(text).toContain('Tirou 1: rolar de novo?');
    expect(text).toContain(
      'Sortudo: você pode rolar esse d20 de novo e fica com o novo resultado.',
    );
    expect(text).toContain(
      'Seu d20: 1d20 (1) + 5 = 6. O resultado só aparece depois da sua escolha.',
    );
    expect(text).not.toMatch(/Acertou|Errou|Voltar à sua vez|Usar a Inspiração de Bardo/);
    expect(text).toContain('Ficar com o 1');
  });

  it('"Rolar de novo" casts the spell with the new d20, and the result says "Sortudo: 1 → 14"', async () => {
    const { fixture, el, sheet, resources, apply, ask } = setup();
    ask();
    const enc = encounter({ combatants: [combatant({ id: 't', label: 'Goblin' })] });
    resources.answerOutsideLucky.mockResolvedValue({
      result: { case: 'spellCast', value: { encounter: enc, cast: cast(14, 14) } },
    });
    await sheet.answerLucky(true, { inApp: true });
    fixture.detectChanges();
    expect(resources.answerOutsideLucky.mock.calls[0].slice(0, 4)).toEqual([
      'c',
      'h1',
      true,
      { inApp: true },
    ]);
    expect(apply).toHaveBeenCalledWith(enc);
    expect(sheet.cast()).not.toBeNull();
    expect(sheet.luckyOffer()).toBeNull();
    const text = flat(el);
    expect(text).not.toContain('Tirou 1: rolar de novo?');
    expect(text).toContain('Sortudo: 1 → 14');
  });

  it('"Ficar com o 1" sends no new die, and the result says nothing of Lucky', async () => {
    const { fixture, el, sheet, resources, ask } = setup();
    ask();
    resources.answerOutsideLucky.mockResolvedValue({
      result: { case: 'spellCast', value: { encounter: encounter({}), cast: cast(1) } },
    });
    await sheet.answerLucky(false, null);
    fixture.detectChanges();
    expect(resources.answerOutsideLucky.mock.calls[0].slice(0, 4)).toEqual([
      'c',
      'h1',
      false,
      null,
    ]);
    expect(flat(el)).not.toContain('Sortudo:');
  });

  it('with real dice sends the typed face of the new d20', async () => {
    const { sheet, resources, ask } = setup();
    ask();
    resources.answerOutsideLucky.mockResolvedValue({
      result: { case: 'spellCast', value: { encounter: encounter({}), cast: cast(15, 15) } },
    });
    await sheet.answerLucky(true, { face: 15 });
    expect(resources.answerOutsideLucky.mock.calls[0][3]).toEqual({ face: 15 });
  });

  it('after the new d20, a Bardic Inspiration die the caster holds is asked about, on the new roll', async () => {
    const { fixture, el, sheet, resources, ask } = setup();
    ask();
    const next = create(OutsideInspirationOfferSchema, {
      holdId: 'h1',
      sides: 8,
      fromName: 'Orla',
      d20: d20(6, 6),
    });
    resources.answerOutsideLucky.mockResolvedValue({
      result: { case: 'spellCast', value: { encounter: encounter({}), inspirationOffer: next } },
    });
    await sheet.answerLucky(true, { inApp: true });
    fixture.detectChanges();
    expect(sheet.offer()).toBe(next);
    expect(sheet.cast()).toBeNull();
    const text = flat(el);
    expect(text).not.toContain('Tirou 1: rolar de novo?');
    expect(text).toContain('Usar a Inspiração de Bardo (d8)?');
    expect(text).not.toMatch(/Acertou|Errou/);
  });

  it('keeps the key of an answer that failed, so a retry never rolls the d20 twice', async () => {
    const { fixture, el, sheet, resources, ask } = setup();
    ask();
    resources.answerOutsideLucky.mockRejectedValue(new ConnectError('down', Code.Unavailable));
    await sheet.answerLucky(true, { inApp: true });
    await sheet.answerLucky(true, { inApp: true });
    fixture.detectChanges();
    const keys = resources.answerOutsideLucky.mock.calls.map((c) => c[4] as string);
    expect(keys[0]).toBe(keys[1]);
    expect(flat(el.querySelector('[role=alert] p'))).toContain('o servidor não respondeu');
    expect(flat(el)).toContain('Tirou 1: rolar de novo?');
  });
});
