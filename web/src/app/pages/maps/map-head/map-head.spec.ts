import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { mapMessage } from '../../../core/maps/maps-testing';
import { MapHead } from './map-head';

describe('MapHead: renomear e apagar (E6-27)', () => {
  let fixture: ComponentFixture<MapHead>;
  let el: HTMLElement;
  let saved: string[];
  let deleted: number;
  let saveResult: () => Promise<void>;
  let deleteResult: () => Promise<void>;

  beforeEach(async () => {
    saved = [];
    deleted = 0;
    saveResult = () => Promise.resolve();
    deleteResult = () => Promise.resolve();
    TestBed.configureTestingModule({ providers: [provideRouter([])] });
    fixture = TestBed.createComponent(MapHead);
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.componentRef.setInput(
      'map',
      mapMessage('map-1', 'Mirathel e arredores', { revealed: true }),
    );
    fixture.componentRef.setInput('pointCount', 5);
    fixture.componentRef.setInput('tokenCount', 4);
    fixture.componentRef.setInput('saveName', (name: string) => {
      saved.push(name);
      return saveResult();
    });
    fixture.componentRef.setInput('deleteMap', () => {
      deleted++;
      return deleteResult();
    });
    fixture.detectChanges();
    el = fixture.nativeElement;
  });

  function button(text: string): HTMLButtonElement {
    const found = Array.from(el.querySelectorAll('button')).find((b) =>
      b.textContent?.trim().endsWith(text),
    );
    if (!found) {
      throw new Error(`no button "${text}"`);
    }
    return found;
  }

  // The calls are plain promises: let them settle, then render.
  async function settle(): Promise<void> {
    fixture.detectChanges();
    await new Promise((resolve) => setTimeout(resolve));
    await fixture.whenStable();
    fixture.detectChanges();
  }

  function type(value: string): void {
    const input = el.querySelector('input') as HTMLInputElement;
    input.value = value;
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
  }

  it('turns the title into the "Nome do mapa" field, filled in and focused', async () => {
    button('Renomear').click();
    await settle();
    const input = el.querySelector('input') as HTMLInputElement;
    expect(el.querySelector('h1')).toBeNull();
    expect(input.value).toBe('Mirathel e arredores');
    expect(document.activeElement).toBe(input);
    expect(el.textContent).toContain('Até 80 caracteres');
  });

  it('saves the trimmed name and goes back to the title, focus on "Renomear"', async () => {
    button('Renomear').click();
    await settle();
    type('  Arredores de Mirathel  ');
    button('Salvar nome').click();
    await settle();
    expect(saved).toEqual(['Arredores de Mirathel']);
    expect(el.querySelector('input')).toBeNull();
    expect(document.activeElement?.textContent).toContain('Renomear');
  });

  it('does not call the server for an empty or unchanged name', async () => {
    button('Renomear').click();
    await settle();
    type('   ');
    button('Salvar nome').click();
    await settle();
    expect(saved).toEqual([]);
    expect(el.querySelector('input')).not.toBeNull();

    type('Mirathel e arredores');
    button('Salvar nome').click();
    await settle();
    expect(saved).toEqual([]);
    expect(el.querySelector('h1')?.textContent).toContain('Mirathel e arredores');
  });

  it('keeps the field and shows why when the server refuses', async () => {
    saveResult = () =>
      Promise.reject(new Error('O mapa mudou em outra aba. Recarregue a página e tente de novo.'));
    button('Renomear').click();
    await settle();
    type('Outro nome');
    button('Salvar nome').click();
    await settle();
    expect(el.querySelector('[role="alert"]')?.textContent).toContain('O mapa mudou em outra aba');
    expect((el.querySelector('input') as HTMLInputElement).value).toBe('Outro nome');
  });

  it('"Cancelar" leaves the name as it was', async () => {
    button('Renomear').click();
    await settle();
    type('Outro nome');
    button('Cancelar').click();
    await settle();
    expect(saved).toEqual([]);
    expect(el.querySelector('h1')?.textContent).toContain('Mirathel e arredores');
  });

  it('asks in place of the state line before deleting, focus on "Cancelar"', async () => {
    fixture.componentRef.setInput('currentSession', 5);
    button('Apagar mapa').click();
    await settle();
    const group = el.querySelector('[role="group"]') as HTMLElement;
    expect(group.textContent).toContain('Apagar Mirathel e arredores?');
    expect(group.textContent).toContain('Os 5 pontos e os 4 tokens dele vão junto');
    expect(group.textContent).toContain('É o mapa atual da Sessão 5');
    expect(el.querySelector('.state')).toBeNull();
    expect(document.activeElement?.textContent?.trim()).toBe('Cancelar');
    expect(deleted).toBe(0);
  });

  it('deletes only after the confirmation, and "Cancelar" brings the line back', async () => {
    button('Apagar mapa').click();
    await settle();
    button('Cancelar').click();
    await settle();
    expect(deleted).toBe(0);
    expect(el.querySelector('.state')).not.toBeNull();
    expect(document.activeElement?.textContent).toContain('Apagar mapa');

    button('Apagar mapa').click();
    await settle();
    (el.querySelector('[role="group"] .danger') as HTMLButtonElement).click();
    await settle();
    expect(deleted).toBe(1);
  });

  it('shows why a delete failed and keeps the question open', async () => {
    deleteResult = () => Promise.reject(new Error('Só o mestre da campanha muda os mapas.'));
    button('Apagar mapa').click();
    await settle();
    (el.querySelector('[role="group"] .danger') as HTMLButtonElement).click();
    await settle();
    expect(el.querySelector('[role="group"] [role="alert"]')?.textContent).toContain('Só o mestre');
  });
});
