import { signal } from '@angular/core';
import { type ComponentFixture, TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { Code, ConnectError } from '@connectrpc/connect';

import { nextTurn } from '../../../../test-settle';
import { FakeFreeDiceClient, freeRoll } from '../../../core/free-dice/free-dice-testing';
import { FreeDiceState } from '../../../core/free-dice/free-dice-state';
import { FreeDiceSheet, type FreeDiceSheetData } from './free-dice-sheet';

const text = (el: Element | null | undefined) =>
  (el?.textContent ?? '').replace(/\s+/g, ' ').trim();

interface Harness {
  readonly fixture: ComponentFixture<FreeDiceSheet>;
  readonly el: HTMLElement;
  readonly api: FakeFreeDiceClient;
  readonly state: FreeDiceState;
  readonly online: ReturnType<typeof signal<boolean>>;
  readonly ref: { close: ReturnType<typeof vi.fn>; disableClose: boolean };
}

async function settle(h: Pick<Harness, 'fixture'>): Promise<void> {
  h.fixture.detectChanges();
  await h.fixture.whenStable();
  await nextTurn();
  h.fixture.detectChanges();
}

async function mount(
  over: { master?: boolean; physical?: boolean; setup?: (api: FakeFreeDiceClient) => void } = {},
): Promise<Harness> {
  const api = new FakeFreeDiceClient();
  over.setup?.(api);
  const master = over.master ?? false;
  const state = new FreeDiceState(api as never, {
    campaignId: () => 'c1',
    isMaster: () => master,
    show: () => undefined,
    now: () => new Date(2026, 9, 18, 20, 30),
    makeKey: (() => {
      let n = 0;
      return () => `k${++n}`;
    })(),
  });
  const online = signal(true);
  const data: FreeDiceSheetData = {
    state,
    isMaster: master,
    online: online.asReadonly(),
    physical: over.physical ?? false,
  };
  const ref = { close: vi.fn(), disableClose: false };
  TestBed.configureTestingModule({
    providers: [
      { provide: MAT_DIALOG_DATA, useValue: data },
      { provide: MatDialogRef, useValue: ref },
    ],
  });
  const fixture = TestBed.createComponent(FreeDiceSheet);
  const h = { fixture, el: fixture.nativeElement as HTMLElement, api, state, online, ref };
  await state.refresh();
  await settle(h);
  return h;
}

const click = (el: Element | null | undefined) => (el as HTMLElement).click();
const named = (root: HTMLElement, selector: string, name: string) =>
  Array.from(root.querySelectorAll<HTMLElement>(selector)).find((b) => text(b).includes(name));
const bigButton = (el: HTMLElement) => el.querySelector<HTMLButtonElement>('.roll');
/** The words of the big button, without the icon's name. */
const bigText = (el: HTMLElement) => text(el.querySelector('.roll .roll__text'));

function type(input: HTMLInputElement, value: string): void {
  input.value = value;
  input.dispatchEvent(new Event('input', { bubbles: true }));
  input.dispatchEvent(new Event('change', { bubbles: true }));
}

describe('FreeDiceSheet: the form', () => {
  it('has the title, the shortcuts, the dice and the two steppers', async () => {
    const { el } = await mount();
    expect(text(el.querySelector('h2'))).toBe('Rolar dados');
    expect(text(el)).toContain('Atalhos (preenchem o formulário)');
    for (const label of ['1d20', '1d6', '2d6', '1d100']) {
      expect(named(el, 'button.shortcut', label), label).toBeDefined();
    }
    const dice = Array.from(
      el.querySelectorAll('[role="radiogroup"][aria-label="Dado"] .fc__text'),
    ).map(text);
    expect(dice).toEqual(['d4', 'd6', 'd8', 'd10', 'd12', 'd20', 'd100']);
    expect(text(el)).toContain('Quantidade (1 a 20)');
    expect(text(el)).toContain('Modificador (−20 a +20)');
    expect(text(el)).toContain('Digite o número ou use os botões.');
  });

  it('fills the form from a shortcut and the big button says the formula', async () => {
    const { fixture, el } = await mount();
    expect(bigText(el)).toBe('Rolar 1d20');
    click(named(el, 'button.shortcut', '2d6'));
    fixture.detectChanges();
    expect(bigText(el)).toBe('Rolar 2d6');
    expect(el.querySelector<HTMLInputElement>('#free-count')?.value).toBe('2');
  });

  it('moves the number with the buttons and shows the modifier with its sign', async () => {
    const { fixture, el } = await mount();
    const plus = el.querySelector<HTMLElement>('[aria-label="Aumentar o modificador"]');
    click(plus);
    click(plus);
    click(plus);
    fixture.detectChanges();
    expect(el.querySelector<HTMLInputElement>('#free-mod')?.value).toBe('+3');
    expect(bigText(el)).toBe('Rolar 1d20 + 3');
    click(el.querySelector('[aria-label="Mais um dado"]'));
    fixture.detectChanges();
    expect(bigText(el)).toBe('Rolar 2d20 + 3');
  });

  it('takes a typed number and keeps it in range', async () => {
    const { fixture, el } = await mount();
    type(el.querySelector<HTMLInputElement>('#free-count')!, '30');
    fixture.detectChanges();
    expect(el.querySelector<HTMLInputElement>('#free-count')?.value).toBe('20');
    type(el.querySelector<HTMLInputElement>('#free-mod')!, '-50');
    fixture.detectChanges();
    expect(el.querySelector<HTMLInputElement>('#free-mod')?.value).toBe('−20');
    expect(bigText(el)).toBe('Rolar 20d20 - 20');
  });

  it('puts the choice of how to roll the d20 only when one d20 is selected', async () => {
    const { fixture, el } = await mount();
    const mode = () => el.querySelector('[role="radiogroup"][aria-label="Como rolar o d20"]');
    expect(mode()).not.toBeNull();
    expect(Array.from(mode()!.querySelectorAll('.fc__text')).map(text)).toEqual([
      'Normal',
      'Vantagem',
      'Desvantagem',
    ]);
    click(named(el, 'label', 'Vantagem')!.querySelector('input'));
    fixture.detectChanges();
    expect(bigText(el)).toBe('Rolar 1d20 com vantagem');
    click(named(el, 'button.shortcut', '2d6'));
    fixture.detectChanges();
    expect(mode()).toBeNull();
    expect(text(el)).toContain('Vantagem e desvantagem valem para um d20 só.');
    expect(bigText(el)).toBe('Rolar 2d6');
  });

  it('tells the player the roll goes to the whole table, and gives only the master the choice', async () => {
    const player = await mount();
    expect(text(player.el)).toContain('A sua rolagem vai para a mesa toda.');
    expect(player.el.querySelector('[aria-label="Quem vê"]')).toBeNull();
  });

  it('lets the master choose who sees it, and the big button follows', async () => {
    const { fixture, el } = await mount({ master: true });
    expect(text(el)).toContain('Para a mesa: todos veem a rolagem e o dado na tela.');
    expect(bigText(el)).toBe('Rolar 1d20 para a mesa');
    click(named(el, 'label', 'Só eu')!.querySelector('input'));
    fixture.detectChanges();
    expect(text(el)).toContain('Só eu: nenhum jogador recebe esta rolagem, nem no registro.');
    expect(bigText(el)).toBe('Rolar 1d20, só eu');
    expect(text(el.querySelector('.frame__head'))).toContain('Só eu');
  });

  it('puts the lock of "Só eu" straight in the big button, where Material lines it up with the words', async () => {
    const { fixture, el } = await mount({ master: true });
    expect(bigButton(el)!.querySelector('mat-icon')).toBeNull();
    click(named(el, 'label', 'Só eu')!.querySelector('input'));
    fixture.detectChanges();
    // Inside the label (a projection lost to nested @if blocks) the icon sits on the baseline, 3 px above the middle.
    const icon = bigButton(el)!.querySelector('mat-icon')!;
    expect(icon.parentElement).toBe(bigButton(el));
    expect(icon.parentElement!.classList.contains('mdc-button__label')).toBe(false);
  });

  it('has exactly one main action', async () => {
    const { el } = await mount({ master: true });
    expect(el.querySelectorAll('button.roll')).toHaveLength(1);
  });

  it('says "Rolar no app" at a table of physical dice, with the note', async () => {
    const { el } = await mount({ physical: true });
    expect(text(el)).toContain('Esta é a rolagem do app.');
    expect(bigText(el)).toBe('Rolar no app');
    expect(bigButton(el)?.getAttribute('aria-label')).toBe('Rolar no app: 1d20');
  });
});

describe('FreeDiceSheet: rolling', () => {
  it('rolls once and shows the result card, with the dice and who sees it', async () => {
    const { fixture, el, api } = await mount({
      setup: (a) => (a.answer = freeRoll('r1', { total: 11 })),
    });
    click(named(el, 'button.shortcut', '2d6'));
    click(bigButton(el));
    click(bigButton(el));
    await settle({ fixture });
    expect(api.rolls).toHaveLength(1);
    expect(api.rolls[0]).toMatchObject({ sides: 6, count: 2, modifier: 0, idempotencyKey: 'k1' });
    expect(text(el.querySelector('.card__total'))).toBe('11');
    expect(text(el.querySelector('.card__line'))).toBe('2d6 + 3 = 11');
    expect(Array.from(el.querySelectorAll('.die')).map(text)).toEqual(['4', '4']);
    expect(text(el.querySelector('.card__who'))).toContain('Rolagem livre · para a mesa');
    expect(text(el.querySelector('.foot'))).toContain('Mudar dados');
    expect(text(el.querySelector('.foot'))).toContain('Rolar 2d6 de novo');
  });

  it('shows "Rolando…" and a disabled button while the server rolls', async () => {
    let release!: () => void;
    const { fixture, el } = await mount({
      setup: (a) => {
        a.hold = new Promise<void>((r) => (release = r));
      },
    });
    click(bigButton(el));
    fixture.detectChanges();
    expect(bigText(el)).toBe('Rolando…');
    expect(bigButton(el)?.getAttribute('aria-disabled')).toBe('true');
    expect(text(el.querySelector('.card--rolling'))).toContain('Rolagem livre · 1d20');
    expect(el.querySelectorAll('.die--unknown').length).toBe(1);
    release();
    await settle({ fixture });
    expect(el.querySelector('.card--rolling')).toBeNull();
  });

  it('locks the sheet while the server rolls, so the backdrop and Esc do not close it', async () => {
    let release!: () => void;
    const { fixture, el, ref } = await mount({
      setup: (a) => {
        a.hold = new Promise<void>((r) => (release = r));
      },
    });
    expect(ref.disableClose).toBe(false);
    click(bigButton(el));
    await settle({ fixture });
    expect(ref.disableClose).toBe(true);
    release();
    await settle({ fixture });
    expect(ref.disableClose).toBe(false);
  });

  it('marks the discarded d20 in words', async () => {
    const { fixture, el } = await mount({
      setup: (a) =>
        (a.answer = freeRoll('r1', {
          sides: 20,
          count: 1,
          modifier: 5,
          mode: 'advantage',
          formula: '1d20 + 5 com vantagem',
          dice: [
            { face: 14, discarded: false },
            { face: 9, discarded: true },
          ],
          total: 19,
        })),
    });
    click(bigButton(el));
    await settle({ fixture });
    expect(text(el.querySelector('.card__line'))).toBe('1d20 + 5 com vantagem = 19');
    expect(text(el.querySelector('.die--discarded'))).toContain('9');
    expect(text(el.querySelector('.die--discarded'))).toContain('descartado');
  });

  it('says natural 20 and 1 in words', async () => {
    const one = (face: number) =>
      freeRoll('n', {
        sides: 20,
        count: 1,
        modifier: 5,
        formula: '1d20 + 5',
        dice: [{ face, discarded: false }],
        total: face + 5,
      });
    const high = await mount({ setup: (a) => (a.answer = one(20)) });
    click(bigButton(high.el));
    await settle(high);
    expect(text(high.el.querySelector('.card__line'))).toBe('1d20 + 5 = 25 · 20 natural');
    TestBed.resetTestingModule();
    const low = await mount({ setup: (a) => (a.answer = one(1)) });
    click(bigButton(low.el));
    await settle(low);
    expect(text(low.el.querySelector('.card__line'))).toBe('1d20 + 5 = 6 · 1 natural');
  });

  it('wraps 20 dice in lines with the sum on top', async () => {
    const dice = Array.from({ length: 20 }, (_, i) => ({ face: (i % 6) + 1, discarded: false }));
    const { fixture, el } = await mount({
      setup: (a) =>
        (a.answer = freeRoll('big', {
          count: 20,
          modifier: 3,
          formula: '20d6 + 3',
          dice,
          total: 78,
        })),
    });
    click(bigButton(el));
    await settle({ fixture });
    expect(el.querySelectorAll('.die')).toHaveLength(20);
    expect(el.querySelector('.card--many')).not.toBeNull();
  });

  it('goes back to the form with "Mudar dados" and rolls again with "de novo"', async () => {
    const { fixture, el, api } = await mount();
    click(bigButton(el));
    await settle({ fixture });
    click(named(el, 'button', 'Rolar 1d20 de novo'));
    await settle({ fixture });
    expect(api.rolls).toHaveLength(2);
    expect(api.rolls[1].idempotencyKey).toBe('k2');
    click(named(el, 'button', 'Mudar dados'));
    fixture.detectChanges();
    expect(el.querySelector('.card__total')).toBeNull();
    expect(el.querySelector('#free-count')).not.toBeNull();
  });

  it('says what the master alone saw, with the badge, in words', async () => {
    const { fixture, el } = await mount({
      master: true,
      setup: (a) =>
        (a.answer = freeRoll('s', {
          secret: true,
          byMaster: true,
          rollerName: 'Mestre',
          sides: 20,
          count: 1,
          modifier: 0,
          formula: '1d20',
          dice: [{ face: 14, discarded: false }],
          total: 14,
        })),
    });
    click(named(el, 'label', 'Só eu')!.querySelector('input'));
    click(bigButton(el));
    await settle({ fixture });
    expect(text(el.querySelector('.card__who'))).toContain('Só você');
    expect(text(el)).toContain('Só você viu esta rolagem. Nenhum jogador a recebeu.');
    expect(text(el.querySelector('.foot'))).toContain('Rolar 1d20 de novo, só eu');
  });

  it("shows the server's refusal with the wait, and the form still works", async () => {
    const { fixture, el, api } = await mount({
      setup: (a) =>
        (a.rollError = new ConnectError(
          'slow',
          Code.ResourceExhausted,
          new Headers({ 'Retry-After': '3' }),
        )),
    });
    click(bigButton(el));
    await settle({ fixture });
    const alert = el.querySelector('[role="alert"]');
    expect(text(alert)).toContain('Não deu para rolar.');
    expect(text(alert)).toContain('Tente de novo em 3 segundos.');
    expect(bigButton(el)?.getAttribute('aria-disabled')).not.toBe('true');
    api.rollError = null;
    click(bigButton(el));
    await settle({ fixture });
    expect(el.querySelector('[role="alert"]')).toBeNull();
    expect(el.querySelector('.card__total')).not.toBeNull();
  });

  it('turns the button off and says why when the connection is gone', async () => {
    const { fixture, el, api, online } = await mount();
    online.set(false);
    fixture.detectChanges();
    expect(bigButton(el)?.getAttribute('aria-disabled')).toBe('true');
    expect(text(el)).toContain('Reconectando… Não dá para rolar sem conexão.');
    click(bigButton(el));
    await settle({ fixture });
    expect(api.rolls).toHaveLength(0);
  });
});

describe('FreeDiceSheet: the last rolls', () => {
  it('says there is nothing yet', async () => {
    const { el } = await mount();
    expect(text(el)).toContain('Suas últimas rolagens nesta sessão');
    expect(text(el)).toContain('Você ainda não rolou nesta sessão. As rolagens aparecem aqui.');
  });

  it('says the list is loading, then lists with the time and "Repetir"', async () => {
    const api = new FakeFreeDiceClient();
    api.lists = {
      table: [],
      mine: [
        freeRoll('a', {}, 28),
        freeRoll(
          'b',
          {
            sides: 20,
            count: 1,
            modifier: 5,
            formula: '1d20 + 5',
            total: 17,
            dice: [{ face: 12, discarded: false }],
          },
          24,
        ),
      ],
    };
    const { el } = await mount({ setup: (a) => (a.lists = api.lists) });
    const rows = Array.from(el.querySelectorAll('.last__row'));
    expect(rows.map((r) => text(r.querySelector('.last__text')))).toEqual([
      '2d6 + 3 = 11 (4, 4)',
      '1d20 + 5 = 17 (12)',
    ]);
    expect(text(rows[0])).toContain('há 2 min');
    expect(text(rows[0])).toContain('Repetir 2d6 + 3');
  });

  it('marks a secret roll with "Só você"', async () => {
    const { el } = await mount({
      master: true,
      setup: (a) => (a.lists = { table: [], mine: [freeRoll('s', { secret: true })] }),
    });
    expect(text(el.querySelector('.last__row'))).toContain('Só você');
  });

  it('says it could not load, offers "Tentar de novo", and the form still rolls', async () => {
    const { fixture, el, api } = await mount({
      setup: (a) => (a.listError = new ConnectError('down', Code.Unavailable)),
    });
    expect(text(el)).toContain(
      'Não deu para carregar as suas últimas rolagens. Você ainda pode rolar.',
    );
    api.listError = null;
    api.lists = { table: [], mine: [freeRoll('a')] };
    click(named(el, 'button', 'Tentar de novo'));
    await settle({ fixture });
    expect(el.querySelector('.last__row')).not.toBeNull();
  });

  it('"Repetir" rolls the same dice again at once', async () => {
    const { fixture, el, api } = await mount({
      setup: (a) => (a.lists = { table: [], mine: [freeRoll('a', {}, 28)] }),
    });
    click(named(el, 'button', 'Repetir 2d6 + 3'));
    await settle({ fixture });
    expect(api.rolls[0]).toMatchObject({ sides: 6, count: 2, modifier: 3 });
  });
});
