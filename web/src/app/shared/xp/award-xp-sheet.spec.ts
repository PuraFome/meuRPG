import { ComponentFixture, TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import { XpMode } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  AwardXPResponseSchema,
  XPAwardSchema,
  XPBlockedReason,
  XPBlockedSchema,
} from '../../../gen/meurpg/progression/v1/progression_pb';
import type { ExperienceRow } from '../../core/progression/experience-store';
import { ProgressionClient } from '../../core/progression/progression-client';
import { type AwardXpData, AwardXpSheet } from './award-xp-sheet';

const nbsp = ' ';

function row(id: string, name: string, sub: string, xp = 2600): ExperienceRow {
  return { id, name, sub, level: 3, xp, nextLevelXp: 2700, canLevelUp: false, levelUpReason: 0 };
}

const PARTY = [
  row('p1', 'Pensantus', 'Mago 3 · de Vinicius'),
  row('t1', 'Toren', 'Guerreiro 3 · de Caio', 2250),
  row('b1', 'Brisa', 'Ladina 3 · de Lia', 1950),
];

describe('AwardXpSheet (E7-07)', () => {
  const award = vi.fn();
  const close = vi.fn();

  beforeEach(() => {
    award.mockReset().mockResolvedValue(
      create(AwardXPResponseSchema, { award: create(XPAwardSchema, { id: 'a1', totalXp: 150 }), xpEach: 50, lostXp: 0 }),
    );
    close.mockReset();
  });

  function setup(over: Partial<AwardXpData> = {}) {
    const data: AwardXpData = { campaignId: 'camp-1', xpMode: XpMode.ENEMIES, rows: PARTY, ...over };
    TestBed.configureTestingModule({
      imports: [AwardXpSheet],
      providers: [
        { provide: ProgressionClient, useValue: { award } },
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close } },
      ],
    });
    const fixture = TestBed.createComponent(AwardXpSheet);
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  const reasonInput = (el: HTMLElement) => el.querySelector<HTMLInputElement>('app-xp-reason input')!;
  const amountInput = (el: HTMLElement) => el.querySelector<HTMLInputElement>('input[aria-describedby*="amount-hint"]')!;
  const boxes = (el: HTMLElement) => Array.from(el.querySelectorAll<HTMLInputElement>('app-xp-recipients input[type="checkbox"]'));
  const give = (el: HTMLElement) => el.querySelector<HTMLButtonElement>('app-xp-actions .primary')!;
  const text = (el: HTMLElement, selector: string) => el.querySelector(selector)?.textContent?.replace(/[ \t\r\n]+/g, ' ').trim();
  const errors = (el: HTMLElement) => Array.from(el.querySelectorAll('mat-error')).map((e) => e.textContent?.trim());

  function type(fixture: ComponentFixture<AwardXpSheet>, input: HTMLInputElement, value: string) {
    input.value = value;
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
  }

  function leave(fixture: ComponentFixture<AwardXpSheet>, input: HTMLInputElement) {
    input.dispatchEvent(new Event('blur'));
    fixture.detectChanges();
  }

  async function fill(fixture: ComponentFixture<AwardXpSheet>, el: HTMLElement, reason = 'Pelo resgate do mercador', amount = '150') {
    type(fixture, reasonInput(el), reason);
    type(fixture, amountInput(el), amount);
  }

  async function press(fixture: ComponentFixture<AwardXpSheet>, el: HTMLElement) {
    give(el).click();
    await fixture.whenStable();
    fixture.detectChanges();
  }

  it('asks for the reason first, and says what the campaign counts', () => {
    const { el } = setup();
    expect(text(el, 'h2')).toBe('Dar XP');
    expect(el.textContent).toContain('Campanha por inimigos. O XP vale a qualquer hora');
    // The sheet opens on the reason (openSheet's `focus` selector).
    expect(reasonInput(el).hasAttribute('data-initial-focus')).toBe(true);
    expect(el.textContent).toContain('XP para o grupo');
  });

  it('is by gold in a gold campaign, with the sum written', () => {
    const { fixture, el } = setup({ xpMode: XpMode.GOLD });
    expect(el.textContent).toContain('Ouro encontrado (PO)');
    expect(el.textContent).toContain('Campanha por ouro: 1 XP por peça de ouro (PO).');
    type(fixture, amountInput(el), '120');
    expect(text(el, '.amount__hint')).toBe(`Vale 1${nbsp}XP por PO: 120${nbsp}PO são 120${nbsp}XP.`);
    expect(text(el, 'app-xp-split .split__sum')).toBe(`120${nbsp}PO = 120${nbsp}XP ÷ 3 = 40`);
  });

  it('counts the reason as it is typed', () => {
    const { fixture, el } = setup();
    type(fixture, reasonInput(el), 'Pelo resgate do mercador');
    expect(el.textContent).toContain('24 de 120');
    expect(reasonInput(el).getAttribute('maxlength')).toBe('120');
  });

  it('shows a field error when the person leaves the field, not on each key', () => {
    const { fixture, el } = setup();
    // Typing a wrong amount shows nothing yet.
    type(fixture, amountInput(el), '0');
    expect(errors(el)).toEqual([]);
    leave(fixture, amountInput(el));
    expect(errors(el)).toEqual([`Digite um valor de${nbsp}1${nbsp}a${nbsp}1.000.000.`]);

    // The reason says it when left empty, and goes away when it is filled.
    leave(fixture, reasonInput(el));
    expect(errors(el)).toContain('Escreva o motivo do XP.');
    type(fixture, reasonInput(el), 'Pelo resgate');
    expect(errors(el)).not.toContain('Escreva o motivo do XP.');
  });

  it('asks for the gold in the gold campaign\'s words', () => {
    const { fixture, el } = setup({ xpMode: XpMode.GOLD });
    leave(fixture, amountInput(el));
    expect(errors(el)[0]).toContain('Digite as PO');
  });

  it('shows the live division and moves it when someone is unchecked', () => {
    const { fixture, el } = setup();
    type(fixture, reasonInput(el), 'Pelo resgate');
    type(fixture, amountInput(el), '150');
    expect(text(el, 'app-xp-split .split__big')).toBe(`50${nbsp}XP para cada`);
    expect(text(el, 'app-xp-split .split__sum')).toBe(`150${nbsp}XP ÷ 3 = 50`);
    expect(el.textContent).toContain(`+50${nbsp}XP`);

    boxes(el)[2].click(); // Brisa
    fixture.detectChanges();
    expect(text(el, 'app-xp-split .split__big')).toBe(`75${nbsp}XP para cada`);
    expect(el.textContent).toContain('Não recebe');
    expect(give(el).textContent?.trim()).toBe(`Dar 75${nbsp}XP a cada um`);
    // A screen reader hears the number, in its own polite status.
    expect(text(el, 'app-xp-split [role="status"]')).toBe(`75${nbsp}XP para cada.`);
  });

  it('writes what is lost when it does not divide', () => {
    const { fixture, el } = setup();
    type(fixture, amountInput(el), '350');
    expect(text(el, 'app-xp-split .split__sum')).toBe(`350${nbsp}XP ÷ 3 = 116. 2${nbsp}XP se perdem na divisão.`);
  });

  it('waits with nobody checked, and says why, linked to the button', () => {
    const { fixture, el } = setup();
    type(fixture, reasonInput(el), 'Pelo resgate');
    type(fixture, amountInput(el), '150');
    boxes(el).forEach((b) => b.click());
    fixture.detectChanges();

    expect(text(el, 'app-xp-split .split__big')).toBe('Ninguém marcado');
    const reason = el.querySelector('app-xp-actions .reason')!;
    expect(reason.textContent?.trim()).toBe('Marque pelo menos um personagem');
    expect(give(el).getAttribute('aria-describedby')).toBe(reason.id);
    expect(give(el).getAttribute('aria-disabled')).toBe('true');
    expect(give(el).textContent?.trim()).toBe('Dar XP');
  });

  it('never calls the server while it waits', async () => {
    const { fixture, el } = setup();
    boxes(el).forEach((b) => b.click());
    fixture.detectChanges();
    await press(fixture, el);
    expect(award).not.toHaveBeenCalled();
  });

  it('goes to the first wrong field instead of calling, with every error showing', async () => {
    const { fixture, el } = setup();
    await press(fixture, el);
    expect(award).not.toHaveBeenCalled();
    expect(errors(el)).toHaveLength(2);
    expect(document.activeElement).toBe(reasonInput(el));

    type(fixture, reasonInput(el), 'Pelo resgate');
    await press(fixture, el);
    expect(document.activeElement).toBe(amountInput(el));
  });

  it('gives an avulso award to the ones checked, and closes with what the server answered', async () => {
    const { fixture, el } = setup();
    await fill(fixture, el);
    boxes(el)[2].click();
    fixture.detectChanges();
    await press(fixture, el);

    expect(award).toHaveBeenCalledTimes(1);
    const [campaignId, input, reason, ids, key] = award.mock.calls[0];
    expect([campaignId, input, reason, ids]).toEqual(['camp-1', { mode: 'manual', amount: 150 }, 'Pelo resgate do mercador', ['p1', 't1']]);
    expect(key).toMatch(/^[0-9a-f-]{36}$/);
    expect(close).toHaveBeenCalledWith(expect.objectContaining({ xpEach: 50, lostXp: 0 }));
  });

  it('gives gold as gold', async () => {
    const { fixture, el } = setup({ xpMode: XpMode.GOLD });
    await fill(fixture, el, 'O baú do Capitão', '120');
    await press(fixture, el);
    expect(award.mock.calls[0][1]).toEqual({ mode: 'gold', gold: 120 });
  });

  describe('opened from the end of a combat', () => {
    const FROM_COMBAT = { reason: 'Combate: Emboscada na estrada', amount: 350, encounterId: 'enc-1' };

    it('comes with the reason and the total filled', () => {
      const { el } = setup(FROM_COMBAT);
      expect(reasonInput(el).value).toBe('Combate: Emboscada na estrada');
      expect(amountInput(el).value).toBe('350');
      expect(give(el).textContent?.trim()).toBe(`Dar 116${nbsp}XP a cada um`);
    });

    it('is the combat\'s award while the total is the combat\'s', async () => {
      const { fixture, el } = setup(FROM_COMBAT);
      await press(fixture, el);
      expect(award.mock.calls[0][1]).toEqual({ mode: 'enemies', encounterId: 'enc-1' });
    });

    it('is an avulso award once the master changed the total', async () => {
      const { fixture, el } = setup(FROM_COMBAT);
      type(fixture, amountInput(el), '300');
      await press(fixture, el);
      expect(award.mock.calls[0][1]).toEqual({ mode: 'manual', amount: 300 });
    });
  });

  describe('when the server refuses', () => {
    const refused = () =>
      new ConnectError('x', Code.FailedPrecondition, undefined, [
        { desc: XPBlockedSchema, value: { reason: XPBlockedReason.XP_BLOCKED_REASON_CHARACTER_NOT_ELIGIBLE } },
      ]);

    it('says why in words, by the typed reason, and keeps the sheet open', async () => {
      award.mockRejectedValue(refused());
      const { fixture, el } = setup();
      await fill(fixture, el);
      await press(fixture, el);

      expect(el.querySelector('[role="alert"]')?.textContent).toContain('morreu ou saiu da campanha');
      expect(close).not.toHaveBeenCalled();
    });

    it('repeats the same key on a retry, and makes a new one when a value changed', async () => {
      award.mockRejectedValueOnce(new ConnectError('x', Code.Unavailable)).mockRejectedValueOnce(new ConnectError('x', Code.Unavailable));
      const { fixture, el } = setup();
      await fill(fixture, el);
      await press(fixture, el);
      await press(fixture, el);
      expect(award.mock.calls[1][4]).toBe(award.mock.calls[0][4]);

      type(fixture, amountInput(el), '300');
      await press(fixture, el);
      expect(award.mock.calls[2][4]).not.toBe(award.mock.calls[0][4]);
    });

    it('sends one call per press, even when pressed twice at once', async () => {
      let finish!: (v: unknown) => void;
      award.mockReturnValue(new Promise((resolve) => (finish = resolve)));
      const { fixture, el } = setup();
      await fill(fixture, el);
      give(el).click();
      give(el).click();
      fixture.detectChanges();
      expect(award).toHaveBeenCalledTimes(1);
      finish(create(AwardXPResponseSchema, { award: create(XPAwardSchema, { id: 'a1' }), xpEach: 50 }));
      await fixture.whenStable();
    });
  });

  it('closes with nothing on "Cancelar" and on the close button', () => {
    const { el } = setup();
    Array.from(el.querySelectorAll<HTMLButtonElement>('app-xp-actions .secondary'))[0].click();
    el.querySelector<HTMLButtonElement>('button[aria-label="Fechar"]')!.click();
    expect(close).toHaveBeenCalledTimes(2);
    expect(close).toHaveBeenCalledWith(undefined);
  });

  it('says so when the campaign has no living character', () => {
    const { el } = setup({ rows: [] });
    expect(el.textContent).toContain('Nenhum personagem de jogador vivo');
    expect(give(el).getAttribute('aria-disabled')).toBe('true');
  });
});
