import { TestBed } from '@angular/core/testing';

import { RollMode, RollModeRequestStatus } from '../../../../../gen/meurpg/play/v1/combat_rolls_pb';
import { RollModePicker } from './roll-mode-picker';

const sources = [
  { effect: RollMode.ADVANTAGE, textPt: 'Alvo Derrubado a 1,5 m: vantagem' },
  { effect: RollMode.DISADVANTAGE, textPt: 'Você está Envenenado: desvantagem' },
];

function setup(inputs: Record<string, unknown> = {}) {
  const fixture = TestBed.createComponent(RollModePicker);
  for (const [k, v] of Object.entries(inputs)) {
    fixture.componentRef.setInput(k, v);
  }
  fixture.componentRef.setInput('mode', inputs['suggested'] ?? RollMode.NORMAL);
  const asked: number[] = [];
  const canceled: number[] = [];
  fixture.componentInstance.ask.subscribe(() => asked.push(1));
  fixture.componentInstance.cancelAsk.subscribe(() => canceled.push(1));
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  const radio = (word: string) =>
    Array.from(el.querySelectorAll<HTMLLabelElement>('.radio')).find((l) =>
      l.textContent!.includes(word),
    )!;
  const pick = (word: string) => {
    radio(word).querySelector('input')!.click();
    fixture.detectChanges();
  };
  const button = (name: string) =>
    Array.from(el.querySelectorAll('button')).find((b) => b.textContent?.includes(name));
  const write = (value: string) => {
    const input = el.querySelector<HTMLInputElement>('.reason__field')!;
    input.value = value;
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
  };
  return { fixture, el, radio, pick, button, write, asked, canceled };
}

describe('RollModePicker', () => {
  it('lists the sources with their word and preselects the suggestion in a radio group', () => {
    const { el, radio } = setup({ suggested: RollMode.ADVANTAGE, sources });
    const tags = Array.from(el.querySelectorAll('.source')).map((s) =>
      s.textContent!.replace(/\s+/g, ' ').trim(),
    );
    expect(tags[0]).toContain('Vantagem');
    expect(tags[0]).toContain('Alvo Derrubado a 1,5 m: vantagem');
    expect(tags[1]).toContain('Desvantagem');
    expect(el.querySelector('[role="radiogroup"]')).not.toBeNull();
    expect(radio('Vantagem').querySelector('input')!.checked).toBe(true);
    expect(radio('Vantagem').textContent).toContain('sugerido');
    expect(el.querySelector('.reason')).toBeNull();
  });

  it('asks for a reason when a player picks disadvantage, with the fiction hint', () => {
    const { el, pick } = setup({ suggested: RollMode.NORMAL });
    pick('Desvantagem');
    expect(el.querySelector('.reason__field')).not.toBeNull();
    expect(el.textContent).toContain('É ficção; não escreva dados reais de pessoas.');
    expect(
      Array.from(el.querySelectorAll('button')).some((b) =>
        b.textContent!.includes('Pedir ao mestre'),
      ),
    ).toBe(false);
  });

  it('offers "Pedir ao mestre" for a better mode, off until the reason is written', () => {
    const { el, pick, button, write, asked } = setup({ suggested: RollMode.NORMAL });
    pick('Vantagem');
    expect(button('Pedir ao mestre')!.getAttribute('aria-disabled')).toBe('true');
    write('estou escondido atrás da pedra');
    expect(button('Pedir ao mestre')!.getAttribute('aria-disabled')).toBeNull();
    button('Pedir ao mestre')!.click();
    expect(asked).toHaveLength(1);
    expect(el.querySelector('.reason__field')).not.toBeNull();
  });

  it('shows "Aguardando o mestre…" in a live region with a cancel, and locks the modes', () => {
    const { fixture, el, button, radio, canceled } = setup({
      suggested: RollMode.NORMAL,
      request: {
        id: 'r1',
        status: RollModeRequestStatus.PENDING,
        requestedMode: RollMode.ADVANTAGE,
      },
    });
    fixture.componentRef.setInput('mode', RollMode.ADVANTAGE);
    fixture.detectChanges();
    const live = el.querySelector('[aria-live="polite"]')!;
    expect(live.textContent).toBe('Aguardando o mestre…');
    expect(radio('Normal').querySelector('input')!.disabled).toBe(true);
    button('Cancelar pedido')!.click();
    expect(canceled).toHaveLength(1);
  });

  it("shows the master's decision once the request is answered", () => {
    const { el, radio } = setup({
      suggested: RollMode.NORMAL,
      request: {
        id: 'r1',
        status: RollModeRequestStatus.ANSWERED,
        decidedMode: RollMode.ADVANTAGE,
      },
    });
    expect(el.textContent).toContain('O mestre decidiu: Vantagem.');
    expect(radio('Vantagem').querySelector('input')!.checked).toBe(true);
    expect(el.querySelector('.reason')).toBeNull();
  });

  it('says only the master gives a better mode where a player cannot ask', () => {
    const { el, pick } = setup({ suggested: RollMode.NORMAL, approval: 'blocked' });
    pick('Vantagem');
    expect(el.textContent).toContain('Só o mestre dá Vantagem aqui.');
    expect(
      Array.from(el.querySelectorAll('button')).some((b) =>
        b.textContent!.includes('Pedir ao mestre'),
      ),
    ).toBe(false);
  });

  it('lets the master pick any mode with just a reason, no request', () => {
    const { el, pick, write } = setup({ suggested: RollMode.NORMAL, approval: 'free' });
    pick('Vantagem');
    write('o goblin está distraído');
    expect(
      Array.from(el.querySelectorAll('button')).some((b) =>
        b.textContent!.includes('Pedir ao mestre'),
      ),
    ).toBe(false);
  });
});
