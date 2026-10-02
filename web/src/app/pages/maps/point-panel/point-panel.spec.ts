import { ComponentFixture, TestBed } from '@angular/core/testing';

import { MapPointKind } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { mapPoint } from '../../../core/maps/maps-testing';
import { PointPanel } from './point-panel';

describe('PointPanel', () => {
  let fixture: ComponentFixture<PointPanel>;
  let el: HTMLElement;
  let dirty: boolean[];

  beforeEach(() => {
    fixture = TestBed.createComponent(PointPanel);
    fixture.componentRef.setInput(
      'point',
      mapPoint('p1', 'Taverna do Javali', { description: 'Onde a Velha Odra conta o que sabe.', revealed: false }),
    );
    fixture.componentRef.setInput('maps', [{ id: 'm2', name: 'Torre de Mirathel' }]);
    dirty = [];
    fixture.componentInstance.dirtyChange.subscribe((d) => dirty.push(d));
    fixture.detectChanges();
    el = fixture.nativeElement;
  });

  function type(input: HTMLInputElement | HTMLTextAreaElement, value: string): void {
    input.value = value;
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
  }

  it('shows the selected point: its name, kind, text and state', () => {
    expect(el.querySelector('.pp__title')?.textContent).toBe('Taverna do Javali');
    expect((el.querySelector('input') as HTMLInputElement).value).toBe('Taverna do Javali');
    expect(el.querySelector('textarea')?.value).toBe('Onde a Velha Odra conta o que sabe.');
    const kinds = Array.from(el.querySelectorAll('[role="radio"]'));
    expect(kinds.map((k) => k.getAttribute('aria-checked'))).toEqual(['false', 'false', 'true']);
    expect(el.querySelector('[role="switch"]')?.getAttribute('aria-checked')).toBe('false');
  });

  it('keeps "Salvar ponto" off until something changes, then sends only that', () => {
    const save = Array.from(el.querySelectorAll('button')).find((b) => b.textContent?.includes('Salvar ponto')) as HTMLButtonElement;
    expect(save.disabled).toBe(true);
    expect(fixture.componentInstance.changes()).toBeNull();
    type(el.querySelector('input') as HTMLInputElement, 'Taverna nova');
    expect(save.disabled).toBe(false);
    expect(dirty.at(-1)).toBe(true);
    expect(fixture.componentInstance.changes()).toEqual({ name: 'Taverna nova' });
  });

  it('shows "Leva para" only for a Submapa, and offers the other maps', () => {
    expect(el.querySelector('select')).toBeNull();
    (el.querySelectorAll('[role="radio"]')[1] as HTMLElement).click();
    fixture.detectChanges();
    const select = el.querySelector('select') as HTMLSelectElement;
    expect(Array.from(select.options).map((o) => o.textContent?.trim())).toEqual(['Nenhum mapa', 'Torre de Mirathel']);
    expect(fixture.componentInstance.changes()).toEqual({ kind: MapPointKind.SUBMAP });
  });

  it('asks for a name when it is empty, and sends nothing', () => {
    type(el.querySelector('input') as HTMLInputElement, '  ');
    expect(fixture.componentInstance.changes()).toBeNull();
    fixture.detectChanges();
    expect(el.querySelector('mat-error')?.textContent).toContain('Dê um nome ao ponto.');
  });

  it('confirms a delete in place, naming the point', () => {
    (Array.from(el.querySelectorAll('button')).find((b) => b.textContent?.includes('Apagar ponto')) as HTMLButtonElement).click();
    fixture.detectChanges();
    expect(el.querySelector('.pp__confirm')?.textContent).toContain('Apagar Taverna do Javali? Não dá para desfazer.');
    const removed: number[] = [];
    fixture.componentInstance.removeConfirmed.subscribe(() => removed.push(1));
    (el.querySelector('.pp__confirm button') as HTMLButtonElement).click();
    expect(removed).toHaveLength(1);
  });

  it('discards the draft', () => {
    type(el.querySelector('input') as HTMLInputElement, 'Outra');
    fixture.componentInstance.discard();
    fixture.detectChanges();
    expect((el.querySelector('input') as HTMLInputElement).value).toBe('Taverna do Javali');
  });
});
