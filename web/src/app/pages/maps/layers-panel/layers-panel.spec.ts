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

  function setup(status: PaintSaveStatus = 'saved', visible = all) {
    changes = [];
    retries = 0;
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({});
    fixture = TestBed.createComponent(LayersPanel);
    fixture.componentRef.setInput('layers', layers);
    fixture.componentRef.setInput('visible', visible);
    fixture.componentRef.setInput('status', status);
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

  it('says "Tudo salvo" with a check, "Salvando", or "Não salvou" with a way to try again', () => {
    setup('saved');
    expect(text()).toContain('Tudo salvo');
    setup('saving');
    expect(text()).toContain('Salvando');
    setup('error');
    expect(text()).toContain('Não salvou');
    Array.from(el.querySelectorAll('button')).find((b) => b.textContent?.includes('Tentar de novo'))!.click();
    expect(retries).toBe(1);
  });
});
