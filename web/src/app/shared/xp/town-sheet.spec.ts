import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { Code, ConnectError } from '@connectrpc/connect';

import { XpMode } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  AwardXPResponseSchema,
  ListTreasuresToConvertResponseSchema,
  TreasureToConvertSchema,
  XPAwardSchema,
  XPBlockedReason,
  XPBlockedSchema,
} from '../../../gen/meurpg/progression/v1/progression_pb';
import type { ExperienceRow } from '../../core/progression/experience-store';
import { ProgressionClient } from '../../core/progression/progression-client';
import { type TownData, TownSheet } from './town-sheet';

const nbsp = ' ';

function row(id: string, name: string, sub: string): ExperienceRow {
  return { id, name, playerUserId: '', sub, level: 4, xp: 3000, nextLevelXp: 6500, canLevelUp: false, levelUpReason: 0 };
}
const PARTY = [row('p', 'Pensantus', 'Mago 4'), row('t', 'Toren', 'Guerreiro 4'), row('b', 'Brisa', 'Ladina 4'), row('s', 'Sálvia', 'Druida 5')];

function treasure(id: string, name: string, valuePo: number, finders: string[], inSession = true) {
  return create(TreasureToConvertSchema, {
    pointId: id,
    name,
    valuePo,
    mapName: 'A caverna do Vale Seco',
    foundAt: timestampFromDate(new Date(2026, 9, 4, 21, 40)),
    foundBy: finders.map((f) => ({ characterId: f, characterName: f })),
    foundInSession: inSession,
  });
}
const CHEST = treasure('c', 'Baú de moedas', 250, ['Brisa']);
const PURSE = treasure('b', 'Bolsa do capitão', 120, ['Toren']);
const IDOL = treasure('i', 'Ídolo de prata', 50, ['Pensantus', 'Sálvia']);

describe('TownSheet: "Voltar à cidade" (E9-09, MR-041)', () => {
  const award = vi.fn();
  const listTreasures = vi.fn();
  const close = vi.fn();

  beforeEach(() => {
    // The same day as the finds, whatever day the test runs: "às 21:40", never "em 04/10".
    vi.useFakeTimers({ toFake: ['Date'] });
    vi.setSystemTime(new Date(2026, 9, 4, 23, 0));
    close.mockReset();
    listTreasures.mockReset().mockResolvedValue(create(ListTreasuresToConvertResponseSchema, { treasures: [CHEST, PURSE, IDOL], total: 3 }));
    award.mockReset().mockResolvedValue(
      create(AwardXPResponseSchema, {
        award: create(XPAwardSchema, { id: 'a1', gold: 420, treasureCount: 3 }),
        xpEach: 105,
        lostXp: 0,
      }),
    );
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  async function setup(over: Partial<TownData> = {}) {
    const data: TownData = { campaignId: 'camp-1', xpMode: XpMode.GOLD, rows: PARTY, treasures: [CHEST, PURSE, IDOL], total: 3, ...over };
    TestBed.configureTestingModule({
      imports: [TownSheet],
      providers: [
        { provide: ProgressionClient, useValue: { award, listTreasures } },
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close } },
      ],
    });
    const fixture = TestBed.createComponent(TownSheet);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  const text = (el: HTMLElement, selector: string) => el.querySelector(selector)?.textContent?.replace(/[ \t\r\n]+/g, ' ').trim();
  const treasureBoxes = (el: HTMLElement) => Array.from(el.querySelectorAll<HTMLInputElement>('.trow input[type="checkbox"]'));
  const personBoxes = (el: HTMLElement) => Array.from(el.querySelectorAll<HTMLInputElement>('.person input[type="checkbox"]'));
  const primary = (el: HTMLElement) => el.querySelector<HTMLButtonElement>('app-xp-actions .primary')!;
  const settle = async (fixture: { whenStable(): Promise<unknown>; detectChanges(): void }) => {
    await fixture.whenStable();
    fixture.detectChanges();
  };

  it('lists the treasures with who found them and when, the value on the right, all checked', async () => {
    const { el } = await setup();
    expect(text(el, 'h2')).toBe('Voltar à cidade');
    expect(el.textContent).toContain('Converte o ouro encontrado em XP, 1 XP por PO, num prêmio só. Dá para desfazer.');
    // The one subtitle, at every size.
    const rows = Array.from(el.querySelectorAll('.trow'));
    expect(rows.map((r) => text(r as HTMLElement, '.trow__name'))).toEqual(['Baú de moedas', 'Bolsa do capitão', 'Ídolo de prata']);
    expect(text(rows[0] as HTMLElement, '.trow__sub')).toBe(`Encontrado por Brisa às${nbsp}21:40`);
    expect(text(rows[2] as HTMLElement, '.trow__sub')).toContain('por Pensantus e Sálvia');
    expect(rows.map((r) => text(r as HTMLElement, '.trow__po'))).toEqual([`250${nbsp}PO`, `120${nbsp}PO`, `50${nbsp}PO`]);
    expect(treasureBoxes(el).every((b) => b.checked)).toBe(true);
    // Every living character receives, all checked.
    expect(personBoxes(el)).toHaveLength(4);
    expect(personBoxes(el).every((b) => b.checked)).toBe(true);
  });

  it('writes the calculation: the sum, 420 XP ÷ 4 = 105 XP para cada, and nothing left', async () => {
    const { el } = await setup();
    expect(text(el, '.calc__sum')).toBe(`420${nbsp}PO em 3\u00a0tesouros = 420${nbsp}XP`);
    expect(text(el, '.calc__big')).toBe(`420${nbsp}XP ÷ 4 = 105${nbsp}XP para cada`);
    expect(text(el, '.calc__left')).toBe(`Sobra 0${nbsp}XP.`);
    expect(el.querySelector('[role="status"]')?.textContent).toBe(`105${nbsp}XP para cada.`);
    // The one filled button says the number.
    expect(primary(el).textContent).toContain(`Dar 105${nbsp}XP para cada`);
  });

  it('moves the calculation with the choice and names the leftover (370 PO between 3)', async () => {
    const { fixture, el } = await setup();
    treasureBoxes(el)[2].click(); // the idol: 50 PO
    personBoxes(el)[1].click(); // Toren
    fixture.detectChanges();
    expect(text(el, '.calc__sum')).toBe(`370${nbsp}PO em 2\u00a0tesouros = 370${nbsp}XP`);
    expect(text(el, '.calc__big')).toBe(`370${nbsp}XP ÷ 3 = 123${nbsp}XP para cada`);
    expect(text(el, '.calc__left')).toBe(`Sobra 1${nbsp}XP, que não vai para ninguém.`);
    expect(primary(el).textContent).toContain(`Dar 123${nbsp}XP para cada`);
  });

  it('waits, in words, with nothing checked: the button turns dashed and does not call the server', async () => {
    const { fixture, el } = await setup();
    treasureBoxes(el).forEach((b) => b.click());
    fixture.detectChanges();
    expect(text(el, '.calc')).toBe('Marque pelo menos um tesouro e um personagem.');
    expect(primary(el).textContent).toContain('Dar XP');
    expect(primary(el).textContent).not.toContain('para cada');
    expect(primary(el).classList.contains('primary--off')).toBe(true);
    expect(primary(el).getAttribute('aria-disabled')).toBe('true');
    primary(el).click();
    await settle(fixture);
    expect(award).not.toHaveBeenCalled();
  });

  it('waits too when the total is too small for everyone to get 1 XP (3 PO between 4)', async () => {
    const coin = treasure('x', 'Moeda', 3, ['Brisa']);
    listTreasures.mockResolvedValue(create(ListTreasuresToConvertResponseSchema, { treasures: [coin], total: 1 }));
    const { el } = await setup({ treasures: [coin], total: 1 });
    expect(text(el, '.calc')).toBe('O total é pequeno demais: cada um precisa receber pelo menos 1 XP.');
    expect(primary(el).classList.contains('primary--off')).toBe(true);
  });

  it('gives one award: GOLD, the checked treasures by id, no gold typed, the checked characters, a key', async () => {
    const { fixture, el } = await setup();
    treasureBoxes(el)[1].click(); // not the purse
    fixture.detectChanges();
    primary(el).click();
    await settle(fixture);

    expect(award).toHaveBeenCalledTimes(1);
    const [campaignId, input, reason, ids, key] = award.mock.calls[0];
    expect(campaignId).toBe('camp-1');
    expect(input).toEqual({ mode: 'town', treasurePointIds: ['c', 'i'] });
    expect(reason).toBe('Voltar à cidade');
    expect(ids).toEqual(['p', 't', 'b', 's']);
    expect(key).toMatch(/[0-9a-f-]{36}/);
    expect(close).toHaveBeenCalledWith({ award: expect.objectContaining({ id: 'a1' }), xpEach: 105, lostXp: 0 });
  });

  it('retries with the same key and takes a new one when the choice changes', async () => {
    award.mockRejectedValueOnce(new ConnectError('down', Code.Unavailable));
    const { fixture, el } = await setup();
    primary(el).click();
    await settle(fixture);
    expect(el.querySelector('[role="alert"]')?.textContent).toContain('o servidor não respondeu');
    primary(el).click();
    await settle(fixture);
    expect(award.mock.calls[1][4]).toBe(award.mock.calls[0][4]);

    award.mockRejectedValueOnce(new ConnectError('down', Code.Unavailable));
    TestBed.resetTestingModule();
    const again = await setup();
    treasureBoxes(again.el)[0].click();
    again.fixture.detectChanges();
    primary(again.el).click();
    await settle(again.fixture);
    expect(award.mock.calls.at(-1)![4]).not.toBe(award.mock.calls[0][4]);
  });

  describe('errors in place', () => {
    function blocked(reason: XPBlockedReason, treasurePointId = '') {
      return new ConnectError('x', Code.FailedPrecondition, undefined, [{ desc: XPBlockedSchema, value: { reason, treasurePointId } }]);
    }

    it('a treasure converted meanwhile: says so, reads the list again, drops it and keeps the rest of the choice', async () => {
      award.mockRejectedValueOnce(blocked(XPBlockedReason.XP_BLOCKED_REASON_TREASURE_ALREADY_CONVERTED, 'c'));
      const { fixture, el } = await setup();
      treasureBoxes(el)[1].click(); // the purse is unchecked and stays so
      listTreasures.mockResolvedValue(create(ListTreasuresToConvertResponseSchema, { treasures: [PURSE, IDOL], total: 2 }));
      fixture.detectChanges();
      primary(el).click();
      await settle(fixture);
      await settle(fixture);

      expect(el.querySelector('[role="alert"]')?.textContent).toContain('já virou XP em outro prêmio');
      expect(Array.from(el.querySelectorAll('.trow__name'), (n) => n.textContent)).toEqual(['Bolsa do capitão', 'Ídolo de prata']);
      expect(treasureBoxes(el).map((b) => b.checked)).toEqual([false, true]);
      expect(close).not.toHaveBeenCalled();
    });

    it('over the limit: says the limit and keeps the sheet open', async () => {
      award.mockRejectedValueOnce(blocked(XPBlockedReason.XP_BLOCKED_REASON_TREASURES_OVER_LIMIT));
      const { fixture, el } = await setup();
      primary(el).click();
      await settle(fixture);
      expect(el.querySelector('[role="alert"]')?.textContent).toContain('mais de 1.000.000 PO');
      expect(close).not.toHaveBeenCalled();
    });

    it('a campaign that does not convert: says why, in the treasures\' words', async () => {
      award.mockRejectedValueOnce(
        new ConnectError('x', Code.FailedPrecondition, undefined, [
          { desc: XPBlockedSchema, value: { reason: XPBlockedReason.XP_BLOCKED_REASON_MODE_NOT_ALLOWED, xpMode: XpMode.ENEMIES } },
        ]),
      );
      const { fixture, el } = await setup();
      primary(el).click();
      await settle(fixture);
      expect(el.querySelector('[role="alert"]')?.textContent).toContain('então o tesouro não vira XP');
    });
  });

  it('says a find outside a session counts in no session summary', async () => {
    listTreasures.mockResolvedValue(create(ListTreasuresToConvertResponseSchema, { treasures: [CHEST, treasure('o', 'Anel', 40, ['Toren'], false)], total: 2 }));
    const { el } = await setup({ treasures: [CHEST, treasure('o', 'Anel', 40, ['Toren'], false)], total: 2 });
    const rows = Array.from(el.querySelectorAll('.trow'));
    // One line above the list says what it means; each find made outside carries a short tag.
    expect(text(el, '.note--top')).toBe('Um tesouro achado fora de uma sessão não conta em nenhum resumo de sessão.');
    expect(rows[0].textContent).not.toContain('fora de uma sessão');
    expect(rows[1].textContent).toContain('Encontrado por Toren');
    expect(rows[1].textContent).toContain('fora de uma sessão');
  });

  it('says there are more when the server holds more than the 100 it sent', async () => {
    listTreasures.mockResolvedValue(create(ListTreasuresToConvertResponseSchema, { treasures: [CHEST], total: 130 }));
    const { el } = await setup({ treasures: [CHEST], total: 130 });
    expect(text(el, '.note')).toBe('Há mais 129\u00a0tesouros encontrados, que ficam para a próxima vez.');
  });

  it('explains an empty list, with no filled button, and only "Fechar"', async () => {
    listTreasures.mockResolvedValue(create(ListTreasuresToConvertResponseSchema, { treasures: [], total: 0 }));
    const { el } = await setup({ treasures: [], total: 0 });
    expect(text(el, '.none')).toContain('Nenhum tesouro encontrado para converter.');
    expect(el.querySelector('app-xp-actions .primary')).toBeNull();
    expect(Array.from(el.querySelectorAll('app-xp-actions button'), (b) => b.textContent?.trim())).toEqual(['Fechar']);
  });

  it('reads the list again as it opens: what is new comes checked, what left goes', async () => {
    listTreasures.mockResolvedValue(create(ListTreasuresToConvertResponseSchema, { treasures: [PURSE, IDOL, treasure('n', 'Colar', 80, ['Brisa'])], total: 3 }));
    const { el } = await setup({ treasures: [CHEST, PURSE, IDOL], total: 3 });
    expect(Array.from(el.querySelectorAll('.trow__name'), (n) => n.textContent)).toEqual(['Bolsa do capitão', 'Ídolo de prata', 'Colar']);
    expect(treasureBoxes(el).every((b) => b.checked)).toBe(true);
  });

  it('closes without giving on "Cancelar" and on the ✕', async () => {
    const { fixture, el } = await setup();
    const cancel = Array.from(el.querySelectorAll<HTMLButtonElement>('app-xp-actions button')).find((b) => b.textContent?.trim() === 'Cancelar')!;
    cancel.click();
    expect(close).toHaveBeenCalledWith(undefined);
    el.querySelector<HTMLButtonElement>('.frame__close')!.click();
    await settle(fixture);
    expect(close).toHaveBeenCalledTimes(2);
    expect(award).not.toHaveBeenCalled();
  });

  describe('what is known of the list', () => {
    it('says "lendo" while the host has not read the list, never "nenhum"; then lists what it read', async () => {
      let answer!: (v: unknown) => void;
      listTreasures.mockReset().mockReturnValue(new Promise((r) => (answer = r)));
      const { fixture, el } = await setup({ treasures: undefined, total: undefined });
      expect(text(el, '.none')).toBe('Lendo os tesouros encontrados...');
      expect(el.textContent).not.toContain('Nenhum tesouro');
      expect(el.querySelector('app-xp-actions .primary')).toBeNull();
      answer(create(ListTreasuresToConvertResponseSchema, { treasures: [CHEST], total: 1 }));
      await settle(fixture);
      expect(Array.from(el.querySelectorAll('.trow__name'), (n) => n.textContent)).toEqual(['Baú de moedas']);
    });

    it('says it could not read, with "Tentar de novo", and never "nenhum"; the retry reads again', async () => {
      listTreasures.mockReset().mockRejectedValueOnce(new Error('down'));
      const { fixture, el } = await setup({ treasures: undefined, total: undefined });
      expect(el.querySelector('[role="alert"]')?.textContent).toContain('Não foi possível ler os tesouros encontrados.');
      expect(el.textContent).not.toContain('Nenhum tesouro');
      listTreasures.mockResolvedValue(create(ListTreasuresToConvertResponseSchema, { treasures: [CHEST, PURSE], total: 2 }));
      Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) => b.textContent?.includes('Tentar de novo'))!.click();
      await settle(fixture);
      expect(el.querySelectorAll('.trow')).toHaveLength(2);
    });

    it('says "nenhum" only after a read that worked and found none', async () => {
      listTreasures.mockResolvedValue(create(ListTreasuresToConvertResponseSchema, { treasures: [], total: 0 }));
      const { el } = await setup({ treasures: undefined, total: undefined });
      expect(text(el, '.none')).toContain('Nenhum tesouro encontrado para converter.');
      expect(text(el, '.none')).toContain('marque no mapa');
    });
  });

  it('keeps who receives in the footer, always in sight, and moves with the choice', async () => {
    const { fixture, el } = await setup();
    expect(text(el, '.calc__who')).toBe('Para 4: Pensantus, Toren, Brisa e Sálvia');
    personBoxes(el)[0].click();
    fixture.detectChanges();
    expect(text(el, '.calc__who')).toBe('Para 3: Toren, Brisa e Sálvia');
  });

  it('agrees in number when one treasure is beyond the list', async () => {
    listTreasures.mockResolvedValue(create(ListTreasuresToConvertResponseSchema, { treasures: [CHEST], total: 2 }));
    const { el } = await setup({ treasures: [CHEST], total: 2 });
    expect(text(el, '.note')).toBe(`Há mais 1${nbsp}tesouro encontrado, que fica para a próxima vez.`);
  });

  describe('refusals that leave the list stale', () => {
    it('a treasure that is gone (not_found): says so, reads again, and does not talk about the campaign', async () => {
      award.mockRejectedValueOnce(new ConnectError('x', Code.NotFound));
      const { fixture, el } = await setup();
      listTreasures.mockResolvedValue(create(ListTreasuresToConvertResponseSchema, { treasures: [PURSE, IDOL], total: 2 }));
      primary(el).click();
      await settle(fixture);
      await settle(fixture);
      expect(el.querySelector('[role="alert"]')?.textContent).toContain('não está mais no mapa');
      expect(el.querySelector('[role="alert"]')?.textContent).not.toContain('campanha');
      expect(Array.from(el.querySelectorAll('.trow__name'), (n) => n.textContent)).toEqual(['Bolsa do capitão', 'Ídolo de prata']);
    });

    it('a character that cannot receive leaves "Quem recebe", so the retry can go', async () => {
      award.mockRejectedValueOnce(
        new ConnectError('x', Code.FailedPrecondition, undefined, [
          { desc: XPBlockedSchema, value: { reason: XPBlockedReason.XP_BLOCKED_REASON_CHARACTER_NOT_ELIGIBLE, characterId: 's' } },
        ]),
      );
      const { fixture, el } = await setup();
      primary(el).click();
      await settle(fixture);
      await settle(fixture);
      expect(personBoxes(el)).toHaveLength(3);
      expect(el.querySelector('[role="alert"]')?.textContent).toContain('morreu ou saiu');
      expect(text(el, '.calc__who')).toBe('Para 3: Pensantus, Toren e Brisa');
      primary(el).click();
      await settle(fixture);
      expect(award.mock.calls[1][3]).toEqual(['p', 't', 'b']);
      expect(close).toHaveBeenCalled();
    });
  });

  it('is one subtitle at every size, and "Fechar" is outlined when it is the only button', async () => {
    listTreasures.mockResolvedValue(create(ListTreasuresToConvertResponseSchema, { treasures: [], total: 0 }));
    const { el } = await setup({ treasures: [], total: 0 });
    const only = el.querySelector<HTMLButtonElement>('app-xp-actions button')!;
    expect(only.textContent?.trim()).toBe('Fechar');
    expect(only.classList.contains('mat-mdc-outlined-button')).toBe(true);
  });
});
