import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';

import type { ConsumableTargetVm } from '../../../core/characters/consumables';
import {
  type UseConsumableData,
  UseConsumableSheet,
  type UseConsumableRequest,
} from './use-consumable-sheet';

/** The text of a node as read, without the ligature names of its icons. */
const flat = (n: Element | null | undefined) => {
  if (!n) {
    return undefined;
  }
  const copy = n.cloneNode(true) as Element;
  copy.querySelectorAll('mat-icon').forEach((i) => i.remove());
  return copy.textContent?.replace(/\s+/g, ' ').trim();
};

const BRISA: ConsumableTargetVm = {
  id: 'c2',
  name: 'Brisa',
  distanceFt: 5,
  down: true,
  line: 'A 1,5 m · Caída',
};
const JABUTI: ConsumableTargetVm = {
  id: 'c3',
  name: 'Jabuti',
  distanceFt: 5,
  down: false,
  line: 'A 1,5 m · Ileso',
};

describe('UseConsumableSheet', () => {
  const use = vi.fn();
  const loadTargets = vi.fn();
  const close = vi.fn();

  beforeEach(() => {
    use.mockReset().mockResolvedValue('');
    loadTargets.mockReset().mockResolvedValue({ targets: [BRISA, JABUTI], error: '' });
    close.mockReset();
  });

  function render(over: Partial<UseConsumableData> = {}) {
    const data: UseConsumableData = {
      itemName: 'Poção de cura',
      itemKey: 'item:potion-of-healing',
      kind: 'potion',
      effectPt: 'Cura 2d4 + 2',
      left: 2,
      actingFor: null,
      inCombat: true,
      canApp: true,
      canType: true,
      preferApp: true,
      loadTargets,
      use,
      ...over,
    };
    TestBed.configureTestingModule({
      providers: [
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close, disableClose: false } },
      ],
    });
    const fixture = TestBed.createComponent(UseConsumableSheet);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const settle = async () => {
      await fixture.whenStable();
      fixture.detectChanges();
    };
    return { el, settle, fixture };
  }

  /** The text of a button without the ligature of its icon. */
  const label = (b: HTMLButtonElement | undefined) =>
    Array.from(b?.childNodes ?? [])
      .filter((n) => n.nodeType === Node.TEXT_NODE || (n as Element).tagName !== 'MAT-ICON')
      .map((n) => n.textContent)
      .join('')
      .replace(/\s+/g, ' ')
      .trim();

  it('names the potion, how many are left and what a combat costs, then its effect', async () => {
    const { el, settle } = render();
    await settle();
    expect(flat(el.querySelector('h2'))).toBe('Usar · Poção de cura');
    expect(flat(el.querySelector('.frame__sub'))).toBe('Restam 2 · em combate, gasta a ação');
    expect(flat(el.querySelector('.effect'))).toContain('Cura 2d4 + 2');
  });

  it('is "Usar por Tor" for the master, with the item in the line under it', async () => {
    const { el, settle } = render({
      actingFor: 'Tor',
      itemName: 'Poção de resistência',
      itemKey: 'item:potion-of-resistance-fire',
      effectPt: 'Resistência a fogo por 1 hora',
      left: 1,
      inCombat: false,
    });
    await settle();
    expect(flat(el.querySelector('h2'))).toBe('Usar por Tor');
    expect(flat(el.querySelector('.frame__sub'))).toBe('Poção de resistência · resta 1');
  });

  it('opens with "Dar a…" outlined and "Beber" filled', async () => {
    const { el, settle } = render();
    await settle();
    const give = Array.from(el.querySelectorAll('button')).find((b) => label(b) === 'Dar a…')!;
    const drink = Array.from(el.querySelectorAll('button')).find((b) => label(b) === 'Beber')!;
    expect(give.classList).toContain('mat-mdc-outlined-button');
    expect(drink.classList).toContain('mat-mdc-unelevated-button');
    expect(el.querySelector('input[type=radio]')).toBeNull();
  });

  it('drinks at once with "Beber": no receiver, the app rolls, one key, and closes with true', async () => {
    const { el, settle } = render();
    await settle();
    Array.from(el.querySelectorAll('button'))
      .find((b) => label(b) === 'Beber')!
      .click();
    await settle();
    expect(use).toHaveBeenCalledWith(
      { target: null, roll: { inApp: true } } satisfies UseConsumableRequest,
      expect.any(String),
    );
    expect(close).toHaveBeenCalledWith(true);
  });

  it('shows no dice for a potion that heals nothing', async () => {
    const { el, settle } = render({ effectPt: 'Resistência a fogo por 1 hora' });
    await settle();
    expect(flat(el)).not.toContain('O app rola');
    expect(flat(el)).not.toContain('Digitar os dados');
  });

  it('lists who may receive it as radio rows, the first one chosen, and says what the dice are', async () => {
    const { el, settle } = render();
    await settle();
    Array.from(el.querySelectorAll('button'))
      .find((b) => label(b) === 'Dar a…')!
      .click();
    await settle();
    expect(flat(el.querySelector('legend'))).toBe('Dar a quem está ao seu lado');
    const rows = Array.from(el.querySelectorAll('.who__row'));
    expect(rows.map(flat)).toEqual(['B Brisa A 1,5 m · Caída', 'J Jabuti A 1,5 m · Ileso']);
    const radios = Array.from(el.querySelectorAll<HTMLInputElement>('input[type=radio]'));
    expect(radios.map((r) => r.checked)).toEqual([true, false]);
    expect(flat(el.querySelector('.dice'))).toContain('O app rola 2d4 + 2 para Brisa.');
    expect(flat(el.querySelector('.cost'))).toBe('Gasta a sua ação. A poção age em Brisa.');
    const buttons = Array.from(el.querySelectorAll('button')).map(label);
    expect(buttons).toContain('Dar a Brisa');
    expect(buttons).toContain('Beber eu mesmo');
  });

  it('follows the choice of another receiver in the button, the dice line and the cost line', async () => {
    const { el, settle, fixture } = render();
    await settle();
    Array.from(el.querySelectorAll('button'))
      .find((b) => label(b) === 'Dar a…')!
      .click();
    await settle();
    el.querySelectorAll<HTMLInputElement>('input[type=radio]')[1].click();
    fixture.detectChanges();
    expect(flat(el.querySelector('.dice'))).toContain('O app rola 2d4 + 2 para Jabuti.');
    expect(flat(el.querySelector('.cost'))).toContain('A poção age em Jabuti.');
    expect(Array.from(el.querySelectorAll('button')).map(label)).toContain('Dar a Jabuti');
  });

  it('gives to the chosen character and closes with true', async () => {
    const { el, settle } = render();
    await settle();
    Array.from(el.querySelectorAll('button'))
      .find((b) => label(b) === 'Dar a…')!
      .click();
    await settle();
    Array.from(el.querySelectorAll('button'))
      .find((b) => label(b) === 'Dar a Brisa')!
      .click();
    await settle();
    expect(use).toHaveBeenCalledWith(
      { target: BRISA, roll: { inApp: true } } satisfies UseConsumableRequest,
      expect.any(String),
    );
    expect(close).toHaveBeenCalledWith(true);
  });

  it('drinks it themselves from the list with "Beber eu mesmo"', async () => {
    const { el, settle } = render();
    await settle();
    Array.from(el.querySelectorAll('button'))
      .find((b) => label(b) === 'Dar a…')!
      .click();
    await settle();
    Array.from(el.querySelectorAll('button'))
      .find((b) => label(b) === 'Beber eu mesmo')!
      .click();
    await settle();
    expect(use).toHaveBeenCalledWith(
      { target: null, roll: { inApp: true } } satisfies UseConsumableRequest,
      expect.any(String),
    );
  });

  it('turns "Dar a…" into a dashed line and says nobody is by, with no list', async () => {
    loadTargets.mockResolvedValue({ targets: [], error: '' });
    const { el, settle } = render();
    await settle();
    const give = Array.from(el.querySelectorAll('button')).find((b) => label(b) === 'Dar a…')!;
    expect(give.getAttribute('aria-disabled')).toBe('true');
    expect(give.classList).toContain('give--off');
    give.click();
    await settle();
    expect(el.querySelector('input[type=radio]')).toBeNull();
    expect(flat(el.querySelector('.none'))).toBe('Ninguém a 1,5 m de você para receber a poção.');
  });

  it('says out of a combat that there is nobody in the party to give it to', async () => {
    loadTargets.mockResolvedValue({ targets: [], error: '' });
    const { el, settle } = render({ inCombat: false });
    await settle();
    expect(flat(el.querySelector('.none'))).toBe(
      'Não há mais ninguém no grupo para receber a poção.',
    );
  });

  it('says a potion the app applies nothing of costs the action as well, in a combat', async () => {
    const { el, settle } = render({
      effectPt: '',
      itemName: 'Poção de escalada',
      itemKey: 'item:potion-of-climbing',
      left: 1,
    });
    await settle();
    expect(flat(el.querySelector('.frame__sub'))).toBe('Resta 1 · em combate, gasta a ação');
    expect(flat(el.querySelector('.cost'))).toBe('Gasta a sua ação.');
  });

  it('says an oil costs no action: the master judges the time', async () => {
    const { el, settle } = render({
      effectPt: '',
      itemName: 'Óleo de afiar',
      itemKey: 'item:oil-of-sharpness',
      left: 1,
    });
    await settle();
    expect(flat(el.querySelector('.frame__sub'))).toBe('Resta 1');
    expect(flat(el.querySelector('.cost'))).toBe(
      'Não gasta a ação: o mestre decide o tempo que isso leva.',
    );
    Array.from(el.querySelectorAll('button'))
      .find((b) => label(b) === 'Dar a…')!
      .click();
    await settle();
    expect(flat(el.querySelector('.cost'))).toBe(
      'Não gasta a ação: o mestre decide o tempo que isso leva. A poção age em Brisa.',
    );
  });

  it('uses a scroll with one "Usar" button and no "Dar a…"', async () => {
    const { el, settle } = render({
      kind: 'scroll',
      effectPt: '',
      itemName: 'Pergaminho de magia',
      itemKey: 'item:spell-scroll-1st',
    });
    await settle();
    const labels = Array.from(el.querySelectorAll('button')).map(label);
    expect(labels).toContain('Usar');
    expect(labels).not.toContain('Dar a…');
  });

  it('says out of a combat that it costs no turn, and asks for the party', async () => {
    const { el, settle } = render({ inCombat: false, actingFor: 'Tor' });
    await settle();
    expect(flat(el.querySelector('.cost'))).toBe(
      'Fora de combate: não gasta a vez. “Dar a…” lista o grupo todo.',
    );
    Array.from(el.querySelectorAll('button'))
      .find((b) => label(b) === 'Dar a…')!
      .click();
    await settle();
    expect(flat(el.querySelector('legend'))).toBe('Dar a alguém do grupo');
    expect(flat(el.querySelector('.cost'))).toBe(
      'Fora de combate, não gasta a vez. A poção age em Brisa.',
    );
  });

  it('keeps "Beber" working and says why when the list could not be read', async () => {
    loadTargets.mockResolvedValue({
      targets: [],
      error: 'Esse item não está mais na ficha: alguém já o usou ou o tirou. Nada foi gasto.',
    });
    const { el, settle } = render();
    await settle();
    expect(flat(el.querySelector('[role=alert]'))).toBe(
      'Esse item não está mais na ficha: alguém já o usou ou o tirou. Nada foi gasto.',
    );
    expect(
      Array.from(el.querySelectorAll('button')).find((b) => label(b) === 'Beber'),
    ).toBeTruthy();
  });

  describe('the dice (RN-18)', () => {
    it('writes one field for each die when the player types them, and sends the faces', async () => {
      const { el, settle, fixture } = render();
      await settle();
      Array.from(el.querySelectorAll('button'))
        .find((b) => label(b) === 'Dar a…')!
        .click();
      await settle();
      Array.from(el.querySelectorAll('button'))
        .find((b) => label(b) === 'Digitar os dados')!
        .click();
      fixture.detectChanges();
      const fields = Array.from(el.querySelectorAll<HTMLInputElement>('.dice input'));
      expect(fields).toHaveLength(2);
      expect(Array.from(el.querySelectorAll('.dice label')).map(flat)).toEqual([
        'Dado 1 (d4)',
        'Dado 2 (d4)',
      ]);
      expect(flat(el.querySelector('.dice'))).toContain('O + 2 o app soma.');
      for (const [i, v] of ['3', '2'].entries()) {
        fields[i].value = v;
        fields[i].dispatchEvent(new Event('input'));
      }
      fixture.detectChanges();
      Array.from(el.querySelectorAll('button'))
        .find((b) => label(b) === 'Dar a Brisa')!
        .click();
      await settle();
      expect(use).toHaveBeenCalledWith(
        { target: BRISA, roll: { faces: [3, 2] } } satisfies UseConsumableRequest,
        expect.any(String),
      );
    });

    it('keeps the button off until every face is of its die, and says what is wrong', async () => {
      const { el, settle, fixture } = render();
      await settle();
      Array.from(el.querySelectorAll('button'))
        .find((b) => label(b) === 'Digitar os dados')!
        .click();
      fixture.detectChanges();
      const fields = Array.from(el.querySelectorAll<HTMLInputElement>('.dice input'));
      fields[0].value = '5';
      fields[0].dispatchEvent(new Event('input'));
      fields[1].value = '2';
      fields[1].dispatchEvent(new Event('input'));
      fixture.detectChanges();
      const drink = Array.from(el.querySelectorAll('button')).find((b) => label(b) === 'Beber')!;
      expect(drink.getAttribute('aria-disabled')).toBe('true');
      expect(flat(el.querySelector('.dice'))).toContain('Digite um número de 1 a 4');
      drink.click();
      expect(use).not.toHaveBeenCalled();
    });

    it('goes back to the app with "Rolar no app"', async () => {
      const { el, settle, fixture } = render();
      await settle();
      Array.from(el.querySelectorAll('button'))
        .find((b) => label(b) === 'Digitar os dados')!
        .click();
      fixture.detectChanges();
      Array.from(el.querySelectorAll('button'))
        .find((b) => label(b) === 'Rolar no app')!
        .click();
      fixture.detectChanges();
      expect(flat(el.querySelector('.dice'))).toContain('O app rola 2d4 + 2.');
    });

    it('opens on the fields when the table plays with real dice only, with no way back to the app', async () => {
      const { el, settle } = render({ canApp: false, preferApp: false });
      await settle();
      expect(el.querySelectorAll('.dice input')).toHaveLength(2);
      expect(
        Array.from(el.querySelectorAll('button')).find((b) => label(b) === 'Rolar no app'),
      ).toBeUndefined();
    });

    it('opens on the fields for a player who prefers their own dice, with "Rolar no app" beside', async () => {
      const { el, settle } = render({ preferApp: false });
      await settle();
      expect(el.querySelectorAll('.dice input')).toHaveLength(2);
      expect(
        Array.from(el.querySelectorAll('button')).find((b) => label(b) === 'Rolar no app'),
      ).toBeTruthy();
    });

    it('offers no typing when the table forces the app', async () => {
      const { el, settle } = render({ canType: false });
      await settle();
      expect(flat(el.querySelector('.dice'))).toContain('O app rola 2d4 + 2.');
      expect(
        Array.from(el.querySelectorAll('button')).find((b) => label(b) === 'Digitar os dados'),
      ).toBeUndefined();
    });
  });

  it('keeps the dialog open and says why when the server refuses, and retries under the same key', async () => {
    use.mockResolvedValueOnce(
      'Quem recebe o item precisa estar a até 1,5 m de quem o dá. Nada foi gasto.',
    );
    const { el, settle } = render();
    await settle();
    Array.from(el.querySelectorAll('button'))
      .find((b) => label(b) === 'Dar a…')!
      .click();
    await settle();
    Array.from(el.querySelectorAll('button'))
      .find((b) => label(b) === 'Dar a Brisa')!
      .click();
    await settle();
    expect(flat(el.querySelector('[role=alert]'))).toBe(
      'Quem recebe o item precisa estar a até 1,5 m de quem o dá. Nada foi gasto.',
    );
    expect(close).not.toHaveBeenCalled();
    Array.from(el.querySelectorAll('button'))
      .find((b) => label(b) === 'Dar a Brisa')!
      .click();
    await settle();
    expect(use.mock.calls[1][1]).toBe(use.mock.calls[0][1]);
    expect(close).toHaveBeenCalledWith(true);
  });

  it('uses another key for another receiver', async () => {
    use.mockResolvedValueOnce('Nada mudou.');
    const { el, settle, fixture } = render();
    await settle();
    Array.from(el.querySelectorAll('button'))
      .find((b) => label(b) === 'Dar a…')!
      .click();
    await settle();
    Array.from(el.querySelectorAll('button'))
      .find((b) => label(b) === 'Dar a Brisa')!
      .click();
    await settle();
    el.querySelectorAll<HTMLInputElement>('input[type=radio]')[1].click();
    fixture.detectChanges();
    Array.from(el.querySelectorAll('button'))
      .find((b) => label(b) === 'Dar a Jabuti')!
      .click();
    await settle();
    expect(use.mock.calls[1][1]).not.toBe(use.mock.calls[0][1]);
  });

  it('says "Usando..." while the call is on its way and does not send twice', async () => {
    let done: (v: string) => void = () => undefined;
    use.mockReturnValue(new Promise<string>((r) => (done = r)));
    const { el, settle } = render();
    await settle();
    Array.from(el.querySelectorAll('button'))
      .find((b) => label(b) === 'Beber')!
      .click();
    await settle();
    const busy = Array.from(el.querySelectorAll('button')).find((b) => label(b) === 'Usando...')!;
    expect(busy).toBeTruthy();
    busy.click();
    expect(use).toHaveBeenCalledTimes(1);
    done('');
    await settle();
    expect(close).toHaveBeenCalledWith(true);
  });

  it('closes without using anything with the close button', async () => {
    const { el, settle } = render();
    await settle();
    el.querySelector<HTMLButtonElement>('button[aria-label="Fechar"]')!.click();
    expect(use).not.toHaveBeenCalled();
    expect(close).toHaveBeenCalled();
  });
});
