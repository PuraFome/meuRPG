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

describe('MapHead: "Imprimir com a grade" (MR-033, E8-12)', () => {
  function render(gridColumns: number): HTMLElement {
    TestBed.configureTestingModule({ providers: [provideRouter([])] });
    const fixture = TestBed.createComponent(MapHead);
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.componentRef.setInput('map', mapMessage('map-1', 'Estrada do Vale', { gridColumns, gridRows: gridColumns > 0 ? 20 : 0 }));
    fixture.componentRef.setInput('saveName', () => Promise.resolve());
    fixture.componentRef.setInput('deleteMap', () => Promise.resolve());
    fixture.detectChanges();
    return fixture.nativeElement;
  }

  it('com grade é um link para a tela de impressão', () => {
    const link = render(30).querySelector('a.print__button')!;
    expect(link.textContent).toContain('Imprimir com a grade');
    expect(link.getAttribute('href')).toBe('/campaigns/camp-1/maps/map-1/print');
  });

  it('sem grade é um botão desabilitado, com o motivo escrito ao lado e ligado a ele', () => {
    const el = render(0);
    expect(el.querySelector('a.print__button')).toBeNull();
    const button = el.querySelector('button.print__button')!;
    expect(button.getAttribute('aria-disabled')).toBe('true');
    const reason = el.querySelector('#print-reason')!;
    expect(button.getAttribute('aria-describedby')).toBe('print-reason');
    expect(reason.textContent).toContain('Defina a grade do mapa para imprimir em escala');
  });
});

describe('MapHead: "Trocar imagem" asks before it erases (E9-01 4)', () => {
  let fixture: ComponentFixture<MapHead>;
  let el: HTMLElement;
  let changes: number;

  function render(inputs: { imageErases?: boolean; combatRunning?: boolean; showImage?: boolean } = {}) {
    changes = 0;
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({ providers: [provideRouter([])] });
    fixture = TestBed.createComponent(MapHead);
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.componentRef.setInput('map', mapMessage('map-1', 'A caverna do Vale Seco', { gridColumns: 24, gridRows: 16 }));
    fixture.componentRef.setInput('saveName', () => Promise.resolve());
    fixture.componentRef.setInput('deleteMap', () => Promise.resolve());
    fixture.componentRef.setInput('imageErases', inputs.imageErases ?? false);
    fixture.componentRef.setInput('combatRunning', inputs.combatRunning ?? false);
    fixture.componentRef.setInput('showImage', inputs.showImage ?? true);
    fixture.componentInstance.changeImage.subscribe(() => changes++);
    fixture.detectChanges();
    el = fixture.nativeElement;
  }
  const button = (text: string) => Array.from(el.querySelectorAll('button')).find((b) => b.textContent?.trim().endsWith(text))!;
  const settle = async () => {
    fixture.detectChanges();
    await new Promise((r) => setTimeout(r));
    await fixture.whenStable();
    fixture.detectChanges();
  };

  it('says on a computer that what is drawn on the image is what the players see', () => {
    render();
    expect(el.textContent).toContain('O que está desenhado na imagem, os jogadores veem.');
  });

  it('goes straight to the picker when nothing is painted or seen', () => {
    render({ imageErases: false });
    button('Trocar imagem').click();
    expect(changes).toBe(1);
    expect(el.querySelector('[role="group"] h3')).toBeNull();
  });

  it('asks in place when something would be erased: the focus on the title, "Voltar" first, nothing before the second click', async () => {
    render({ imageErases: true });
    button('Trocar imagem').click();
    await settle();
    const ask = el.querySelector('app-map-ask')!;
    expect(ask.querySelector('h3')?.textContent).toContain('Trocar a imagem?');
    expect(document.activeElement).toBe(ask.querySelector('h3'));
    expect(ask.textContent).toContain('Imagem agora: Imagem de A caverna do Vale Seco');
    expect(ask.textContent).toContain('apaga o terreno, as paredes, a cobertura e a luz pintados, e o que os jogadores já viram. Os pontos e os tokens ficam.');
    expect(Array.from(ask.querySelectorAll('button'), (b) => b.textContent?.trim())).toEqual(['Voltar', 'Apagar e trocar a imagem']);
    expect(changes).toBe(0);
    ask.querySelectorAll('button')[1].click();
    await settle();
    expect(changes).toBe(1);
    expect(el.querySelector('app-map-ask')).toBeNull();
  });

  it('"Voltar" asks nothing of the server and gives the focus back to "Trocar imagem"', async () => {
    render({ imageErases: true });
    button('Trocar imagem').click();
    await settle();
    el.querySelector('app-map-ask button')!.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    await settle();
    expect(changes).toBe(0);
    expect(document.activeElement?.textContent?.trim()).toBe('Trocar imagem');
  });

  it('while a combat runs "Trocar imagem" cannot act, and points at the reason', async () => {
    render({ combatRunning: true, imageErases: true });
    const swap = button('Trocar imagem');
    expect(swap.getAttribute('aria-disabled')).toBe('true');
    expect(swap.getAttribute('aria-describedby')).toBe('combat-why');
    swap.click();
    await settle();
    expect(changes).toBe(0);
    expect(el.querySelector('app-map-ask')).toBeNull();
  });

  it('on a phone the grid is only words, with no way to change it', () => {
    render({ showImage: false });
    expect(el.textContent).toContain('24 × 16 quadrados');
    expect(el.textContent).not.toContain('Mudar a grade');
    expect(el.textContent).not.toContain('O que está desenhado na imagem');
  });
});
