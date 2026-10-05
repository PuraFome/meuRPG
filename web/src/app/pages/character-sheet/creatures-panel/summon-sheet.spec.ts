import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { Code, ConnectError } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';

import { EncounterBlockedReason, EncounterBlockedSchema } from '../../../../gen/meurpg/play/v1/combat_pb';
import { CreaturesClient } from '../../../core/creatures/creatures-client';
import { FakeCreaturesClient, raven, summary, vitalsWith, flat, isOff } from '../../../core/creatures/creatures-testing';
import type { SummonCastVm } from '../../../core/creatures/summon-access';
import { SummonSheet, type SummonSheetData } from './summon-sheet';

const FAMILIAR: SummonCastVm = { key: 'spell:find-familiar', name: 'Encontrar Familiar', level: 1, ritual: true, slot: true, chain: false };
const UNDEAD: SummonCastVm = { key: 'spell:animate-dead', name: 'Animar os Mortos', level: 3, ritual: false, slot: true, chain: false };
const BEASTS: SummonCastVm = { key: 'spell:conjure-animals', name: 'Conjurar Animais', level: 3, ritual: false, slot: true, chain: false };

describe('SummonSheet: casting a summon outside combat (E9-10, MR-037)', () => {
  let api: FakeCreaturesClient;
  let close: ReturnType<typeof vi.fn>;

  async function setup(cast: SummonCastVm, replaces = '') {
    api = new FakeCreaturesClient();
    for (const [k, n, hp, ft] of [['bat', 'Morcego', 1, 5], ['cat', 'Gato', 2, 40], ['raven', 'Corvo', 1, 10]] as const) {
      api.blocks.set(`monster:${k}`, raven({ summary: summary(`monster:${k}`, n), hitPoints: hp, speedWalkFt: ft, speedFlyFt: k === 'raven' ? 50 : 0 }));
    }
    api.blocks.set('monster:skeleton', raven({ summary: summary('monster:skeleton', 'Esqueleto', { typePt: 'morto-vivo' }) }));
    api.blocks.set('monster:zombie', raven({ summary: summary('monster:zombie', 'Zumbi') }));
    api.catalog = [summary('monster:wolf', 'Lobo', { challengeRating: '1/4', sizePt: 'Médio' }), summary('monster:bear', 'Urso-negro', { challengeRating: '1/2' })];
    api.vitals = vitalsWith('char-1', [{ level: 1, total: 4, used: 0 }, { level: 3, total: 2, used: 1 }, { level: 4, total: 1, used: 1 }]);
    // The fake forms only have three of the fifteen: the sheet asks for them all, and a missing block fails the read.
    api.statBlock.mockImplementation(async (_c: string, key: string) => api.blocks.get(key) ?? raven({ summary: summary(key, key.replace('monster:', '')) }));
    close = vi.fn();
    const data: SummonSheetData = { campaignId: 'camp-1', characterId: 'char-1', cast, replaces };
    TestBed.configureTestingModule({
      providers: [
        { provide: CreaturesClient, useValue: api },
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close } },
      ],
    });
    const fixture = TestBed.createComponent(SummonSheet);
    const settle = async () => {
      for (let i = 0; i < 5; i++) {
        fixture.detectChanges();
        await fixture.whenStable();
        await new Promise((r) => setTimeout(r));
      }
      fixture.detectChanges();
    };
    fixture.detectChanges();
    await settle();
    const el = fixture.nativeElement as HTMLElement;
    const button = (name: string) => Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) => flat(b)?.includes(name))!;
    const radio = (name: string) => Array.from(el.querySelectorAll<HTMLLabelElement>('label')).find((l) => flat(l)?.includes(name))!.querySelector<HTMLInputElement>('input[type=radio]')!;
    const pick = async (name: string) => {
      const r = radio(name);
      r.checked = true;
      r.dispatchEvent(new Event('change'));
      await settle();
    };
    const type = async (value: string) => {
      const field = el.querySelector<HTMLInputElement>('input[name=name]')!;
      field.value = value;
      field.dispatchEvent(new Event('input'));
      await settle();
    };
    return { fixture, el, flat, button, pick, type, settle };
  }

  it('Encontrar Familiar as a ritual: the title, "ritual · 1 hora", the name field, the 15 forms and no slot picker', async () => {
    const { el, flat } = await setup(FAMILIAR);
    expect(flat(el.querySelector('.frame__title'))).toBe('Encontrar Familiar');
    expect(flat(el.querySelector('.frame__sub'))).toBe('Magia de 1º círculo · ritual · 1 hora');
    expect(el.querySelector('app-slot-picker')).toBeNull();
    expect(flat(el.querySelector('.cap'))).toBe('Forma · 15 do livro');
    expect(el.querySelectorAll('app-creature-choice-list input[type=radio]')).toHaveLength(15);
    expect(flat(el.querySelector('mat-hint'))).toBe('0 de 40');
  });

  it('says what is missing, one thing at a time, and the filled button stays dashed until it is ready', async () => {
    const { el, flat, button, type, pick } = await setup(FAMILIAR);
    expect(flat(el.querySelector('.line'))).toBe('Falta dar um nome ao familiar.');
    expect(isOff(button('Convocar'))).toBe(true);
    await type('Nanquim');
    expect(flat(el.querySelector('.line'))).toBe('Falta escolher a forma.');
    await pick('Corvo');
    expect(flat(el.querySelector('.line'))).toBe('Conjurar como ritual · 1 hora · sem gastar espaço');
    expect(isOff(button('Convocar Nanquim'))).toBe(false);
    expect(button('Convocar Nanquim').classList).not.toContain('go--off');
  });

  it('lists the forms as "Miúdo · 1 PV · 3 m, voo 15 m" and the search narrows them by name, ignoring accents', async () => {
    const { el, flat, settle } = await setup(FAMILIAR);
    expect(flat(Array.from(el.querySelectorAll('.row')).find((r) => flat(r)?.includes('Corvo')))).toBe('Corvo Miúdo · 1 PV · 3 m, voo 15 m');
    const search = el.querySelector<HTMLInputElement>('input[type=search]')!;
    search.value = 'cor';
    search.dispatchEvent(new Event('input'));
    await settle();
    expect(Array.from(el.querySelectorAll('.row__title')).map((t) => flat(t))).toContain('Corvo');
    expect(el.querySelectorAll('.row').length).toBeLessThan(15);
  });

  it('casts as a ritual with no slot, with the name and the form once, and closes with what to announce', async () => {
    const { button, pick, type, settle } = await setup(FAMILIAR);
    await type('Nanquim');
    await pick('Corvo');
    button('Convocar Nanquim').click();
    await settle();
    expect(api.casts).toHaveLength(1);
    expect(api.casts[0]).toMatchObject({
      campaignId: 'camp-1',
      characterId: 'char-1',
      spellKey: 'spell:find-familiar',
      ritual: true,
      slot: undefined,
      summon: { option: 0, creatureKeys: ['monster:raven'], names: ['Nanquim'] },
    });
    expect(close).toHaveBeenCalledWith({ spellName: 'Encontrar Familiar', ritual: true, names: ['Nanquim'], replaced: false });
  });

  it('a refusal stays in the sheet, in words by its typed reason, and a second tap is a new cast with a new key only after a change', async () => {
    const { el, flat, button, pick, type, settle } = await setup(FAMILIAR);
    await type('Nanquim');
    await pick('Corvo');
    api.failWith = new ConnectError('x', Code.FailedPrecondition, undefined, [{ desc: EncounterBlockedSchema, value: create(EncounterBlockedSchema, { reason: EncounterBlockedReason.SUMMON_IN_COMBAT }) }]);
    button('Convocar Nanquim').click();
    await settle();
    expect(close).not.toHaveBeenCalled();
    expect(flat(el.querySelector('[role=alert]'))).toBe('Há um combate em andamento. Conjure pela sua vez, na tela do combate.');
    const first = api.casts[0].idempotencyKey;
    button('Convocar Nanquim').click();
    await settle();
    expect(api.casts[1].idempotencyKey).toBe(first);
    await pick('Gato');
    button('Convocar Nanquim').click();
    await settle();
    expect(api.casts[2].idempotencyKey).not.toBe(first);
  });

  it('a character that already has a familiar is told it leaves, and the name starts as the old one', async () => {
    const { el, flat } = await setup(FAMILIAR, 'Nanquim');
    expect(flat(el.querySelector('.mr-notice--warning'))).toBe('Nanquim sai da ficha: o novo familiar toma o lugar dele.');
    expect(el.querySelector<HTMLInputElement>('input[name=name]')!.value).toBe('Nanquim');
  });

  it('Animar os Mortos: the slot picker lists only circles from the 3rd with slots, the count follows the slot', async () => {
    const { el, flat, button, pick } = await setup(UNDEAD);
    const rows = Array.from(el.querySelectorAll('app-slot-picker .row')).map((r) => flat(r));
    expect(rows).toHaveLength(2);
    expect(rows[0]).toContain('3º círculo');
    expect(rows[0]).toContain('1 livre de 2');
    expect(rows[1]).toContain('4º círculo');
    expect(rows[1]).toContain('Sem espaço livre');
    // The only free circle is chosen already: the 3rd, so one undead.
    expect(flat(el.querySelector('.forms .cap'))).toBe('Morto-vivo · um');
    await pick('Esqueleto');
    expect(flat(el.querySelector('.line'))).toBe('1 criatura · 1 minuto · gasta um espaço de 3º círculo');
    button('Animar 1 morto-vivo').click();
  });

  it('Animar os Mortos casts with the chosen slot and as many creatures as the server will check', async () => {
    const { button, pick, settle } = await setup(UNDEAD);
    await pick('Zumbi');
    button('Animar 1 morto-vivo').click();
    await settle();
    expect(api.casts[0]).toMatchObject({ spellKey: 'spell:animate-dead', ritual: false, slot: { level: 3, pact: false }, summon: { option: 0, creatureKeys: ['monster:zombie'], names: [] } });
  });

  it('Conjurar Animais: four options with the count the slot makes, the beasts of the option\'s ND, one kind for all', async () => {
    const { el, flat, button, pick, settle } = await setup(BEASTS);
    const options = Array.from(el.querySelectorAll('.opt')).map((o) => flat(o));
    expect(options).toEqual(['1 animal de ND 2 ou menos', '2 animais de ND 1 ou menos', '4 animais de ND 1/2 ou menos', '8 animais de ND 1/4 ou menos']);
    expect(api.searches.at(-1)).toMatchObject({ type: 'beast', maxCr: '2' });
    await pick('4 animais');
    expect(api.searches.at(-1)).toMatchObject({ type: 'beast', maxCr: '1/2' });
    await pick('Urso-negro');
    button('Conjurar os animais').click();
    await settle();
    expect(api.casts[0].summon.option).toBe(2);
    expect(api.casts[0].summon.creatureKeys).toEqual(['monster:bear', 'monster:bear', 'monster:bear', 'monster:bear']);
  });

  it('Cancelar and the X close without casting', async () => {
    const { button, el } = await setup(FAMILIAR);
    button('Cancelar').click();
    expect(close).toHaveBeenCalledWith(undefined);
    expect(api.casts).toHaveLength(0);
    expect(el.querySelector('.frame__close')?.getAttribute('aria-label')).toBe('Fechar');
  });
});
