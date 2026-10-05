import { ComponentFixture, TestBed } from '@angular/core/testing';

import { type MapLayers } from '../../../core/maps/layers';
import type { PaintSaveStatus } from '../../../core/maps/paint-queue';
import { type LayerVisibility, LayersPanel } from './layers-panel';

const layers: MapLayers = {
  columns: 24,
  rows: 16,
  walls: Array.from({ length: 242 }, (_, i) => ({ col: i % 24, row: Math.floor(i / 24) })),
  terrain: [1, 2, 3, 4].map((col) => ({ col, row: 9 })),
  half: [{ col: 19, row: 7 }, { col: 19, row: 8 }],
  threeQuarters: [{ col: 20, row: 4 }],
};
const all: LayerVisibility = { terrain: true, wall: true, cover: true, light: true };

describe('LayersPanel', () => {
  let fixture: ComponentFixture<LayersPanel>;
  let el: HTMLElement;
  let changes: LayerVisibility[];
  let retries: number;

  function setup(status: PaintSaveStatus = 'saved', visible = all, extra: Record<string, unknown> = {}) {
    changes = [];
    retries = 0;
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({});
    fixture = TestBed.createComponent(LayersPanel);
    fixture.componentRef.setInput('layers', layers);
    fixture.componentRef.setInput('visible', visible);
    fixture.componentRef.setInput('status', status);
    for (const [key, value] of Object.entries(extra)) {
      fixture.componentRef.setInput(key, value);
    }
    fixture.componentInstance.visibleChange.subscribe((v) => changes.push(v));
    fixture.componentInstance.retry.subscribe(() => retries++);
    fixture.detectChanges();
    el = fixture.nativeElement;
  }
  const text = () => (el.textContent ?? '').replace(/ /g, ' ').replace(/\s+/g, ' ');

  it('lists the four layers with what each holds, in the artboard\'s words', () => {
    setup();
    for (const name of ['Terreno difícil', 'Parede', 'Cobertura', 'Luz']) {
      expect(text()).toContain(name);
    }
    expect(text()).toContain('4 quadrados · custa +1,5 m por quadrado');
    expect(text()).toContain('242 quadrados · bloqueia movimento, visão e luz');
    expect(text()).toContain('2 quadrados de meia cobertura e 1 de três quartos');
    expect(text()).toContain('nada pintado');
    expect(text()).toContain('Claro');
    expect(text()).toContain('Penumbra');
    expect(text()).toContain('Escuro');
  });

  it('a switch shows or hides a layer on his own map', () => {
    setup();
    const wall = el.querySelector<HTMLElement>('button[role="switch"][aria-labelledby$="-label"]')!;
    expect(el.querySelectorAll('[role="switch"]')).toHaveLength(4);
    expect(wall.getAttribute('aria-checked')).toBe('true');
    wall.click();
    expect(changes).toEqual([{ ...all, terrain: false }]);
  });

  it('says "Tudo salvo" with a check, "Salvando", or "Não salvou" with a way to try again when the server did not answer', () => {
    setup('saved');
    expect(text()).toContain('Tudo salvo');
    setup('saving');
    expect(text()).toContain('Salvando');
    setup('error', all, { problem: 'Não foi possível falar com o servidor agora.', retryable: true });
    expect(text()).toContain('Não salvou');
    expect(el.querySelector('[role="alert"]')?.textContent).toContain('Não foi possível falar com o servidor');
    Array.from(el.querySelectorAll('button')).find((b) => b.textContent?.includes('Tentar de novo'))!.click();
    expect(retries).toBe(1);
  });

  it('a refusal no retry fixes says what happened and offers no "Tentar de novo"', () => {
    setup('error', all, { problem: 'Defina a grade para pintar e ligar a névoa.', retryable: false });
    expect(text()).toContain('Defina a grade para pintar e ligar a névoa.');
    expect(Array.from(el.querySelectorAll('button')).some((b) => b.textContent?.includes('Tentar de novo'))).toBe(false);
  });

  it('never says "Tudo salvo" while painting is off (no grid, or the layers not read yet)', () => {
    setup('saved', all, { active: false });
    expect(text()).not.toContain('Tudo salvo');
    expect(text()).not.toContain('Salvando');
  });

  it('announces "Tudo salvo" once, politely, after a run of strokes, and nothing for each batch', async () => {
    setup('saved');
    const live = () => el.querySelector('[role="status"]')?.textContent?.trim();
    expect(live()).toBe('');
    fixture.componentRef.setInput('status', 'saving');
    fixture.detectChanges();
    await fixture.whenStable();
    expect(live()).toBe('');
    fixture.componentRef.setInput('status', 'saved');
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    expect(live()).toBe('Tudo salvo');
  });
});
