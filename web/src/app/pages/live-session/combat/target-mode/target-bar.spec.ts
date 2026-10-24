import { TestBed } from '@angular/core/testing';
import { vi } from 'vitest';

import { settle } from '../../../../../test-settle';
import { TargetBar } from './target-bar';
import { TargetMode } from './target-mode';

function mount(variant: 'bar' | 'card', darts = false) {
  const open = vi.fn();
  const mode = new TargetMode((run) => run());
  mode.begin({
    key: 'spell:fire-bolt',
    name: darts ? 'Mísseis Mágicos' : 'Raio de Fogo',
    rows: [
      { id: 'g1', label: 'Goblin 1', reason: '', note: '' },
      { id: 'g2', label: 'Goblin 2', reason: '', note: '' },
    ],
    rule: darts ? { kind: 'darts', max: 3, min: 1 } : { kind: 'single', max: 1, min: 1 },
    detail: '+5 para acertar · 1d10 de fogo',
    foe: true,
    origin: null,
    open,
  });
  const fixture = TestBed.createComponent(TargetBar);
  fixture.componentRef.setInput('mode', mode);
  fixture.componentRef.setInput('variant', variant);
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  const button = (name: string) =>
    [...el.querySelectorAll('button')].find((b) => b.textContent?.trim().endsWith(name));
  return { fixture, el, mode, open, button };
}

describe('TargetBar on a phone', () => {
  it('is a region that asks, offers the list, and has an equal pair with Confirmar off', async () => {
    const { fixture, el, button } = mount('bar');
    await settle(fixture);
    expect(el.querySelector('section')?.getAttribute('aria-label')).toBe('Escolha do alvo');
    expect(el.textContent).toContain('Escolha o alvo · Raio de Fogo');
    expect(el.textContent).toContain('Toque num inimigo com anel.');
    expect(button('Escolher na lista')).toBeDefined();
    expect(button('Cancelar')).toBeDefined();
    const confirm = button('Confirmar')!;
    expect(confirm.getAttribute('aria-disabled')).toBe('true');
    expect(button('Desfazer')).toBeUndefined();
  });

  it('moves the focus to the title when the choice starts', async () => {
    const { fixture, el } = mount('bar');
    await settle(fixture);
    expect(document.activeElement).toBe(el.querySelector('.head__title'));
  });

  it('turns Confirmar into the filled button once a target is chosen, and confirms', async () => {
    const { fixture, mode, open, button } = mount('bar');
    mode.tap('g1');
    fixture.detectChanges();
    const confirm = button('Confirmar')!;
    expect(confirm.className).toContain('mat-mdc-unelevated-button');
    confirm.click();
    expect(open).toHaveBeenCalledWith(['g1']);
  });

  it('counts the darts, says it politely and undoes the last one', async () => {
    const { fixture, el, mode, button } = mount('bar', true);
    mode.tap('g1');
    mode.tap('g1');
    fixture.detectChanges();
    expect(el.textContent).toContain('Mísseis Mágicos · 2 de 3 dardos');
    expect(el.querySelector('[role="status"]')?.textContent).toBe('2 de 3 dardos');
    button('Desfazer')!.click();
    fixture.detectChanges();
    expect(mode.picks()).toEqual(['g1']);
  });

  it('cancels', () => {
    const { fixture, mode, button } = mount('bar');
    button('Cancelar')!.click();
    fixture.detectChanges();
    expect(mode.active()).toBe(false);
  });
});

describe('TargetBar in the rail', () => {
  it('asks on the map, with Cancelar and the list, and Confirmar only once a target is named', () => {
    const { fixture, el, mode, button } = mount('card');
    expect(el.textContent).toContain('Escolha o alvo no mapa');
    expect(el.textContent).toContain('Raio de Fogo · toque num inimigo com anel.');
    expect(button('Confirmar')).toBeUndefined();
    mode.tap('g1');
    fixture.detectChanges();
    expect(el.textContent).toContain('Raio de Fogo em Goblin 1');
    expect(button('Confirmar')).toBeDefined();
    expect(button('Escolher na lista')).toBeDefined();
  });
});
