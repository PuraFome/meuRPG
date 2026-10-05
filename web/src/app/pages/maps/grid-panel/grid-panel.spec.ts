import { ComponentFixture, TestBed } from '@angular/core/testing';
import { Code, ConnectError } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';

import { MapBlockedSchema, MapBlockedReason, type Map as MapMessage } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { FakeMapsClient, mapMessage } from '../../../core/maps/maps-testing';
import { MapsClient } from '../../../core/maps/maps-client';
import { GridPanel } from './grid-panel';

describe('GridPanel', () => {
  let fixture: ComponentFixture<GridPanel>;
  let el: HTMLElement;
  let api: FakeMapsClient;
  let changed: MapMessage[];

  async function setup(map: MapMessage, inputs: { erases?: boolean; combatRunning?: boolean } = {}) {
    api = new FakeMapsClient();
    changed = [];
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({ providers: [{ provide: MapsClient, useValue: api }] });
    fixture = TestBed.createComponent(GridPanel);
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.componentRef.setInput('map', map);
    fixture.componentRef.setInput('erases', inputs.erases ?? false);
    fixture.componentRef.setInput('combatRunning', inputs.combatRunning ?? false);
    fixture.componentInstance.changed.subscribe((m) => changed.push(m));
    fixture.detectChanges();
    el = fixture.nativeElement;
    await fixture.whenStable();
  }

  const button = (text: string) => Array.from(el.querySelectorAll('button')).find((b) => b.textContent?.trim().endsWith(text))!;
  const text = () => (el.textContent ?? '').replace(/ /g, ' ').replace(/\s+/g, ' ');
  const settle = async () => {
    fixture.detectChanges();
    await new Promise((r) => setTimeout(r));
    await fixture.whenStable();
    fixture.detectChanges();
  };
  function type(value: string): void {
    const input = el.querySelector('input')!;
    input.value = value;
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
  }

  const withGrid = mapMessage('map-1', 'A caverna do Vale Seco', { gridColumns: 24, gridRows: 16 });

  it('shows the grid in squares and the size of a square', async () => {
    await setup(withGrid);
    expect(text()).toContain('24 × 16 quadrados');
    expect(text()).toContain('1 quadrado = 1,5 m (5 pés)');
  });

  it('asks in place before it erases: the title has the focus, "Voltar" first, and nothing is sent yet', async () => {
    await setup(withGrid, { erases: true });
    button('Mudar a grade').click();
    await settle();
    expect(el.querySelector('h3')?.textContent).toContain('Mudar a grade?');
    expect(document.activeElement).toBe(el.querySelector('h3'));
    expect(text()).toContain('apaga o terreno, as paredes, a cobertura e a luz pintados, e o que os jogadores já viram');
    expect(text()).toContain('Voltar');
    expect(api.calls).toEqual([]);
    type('30');
    expect(text()).toContain('Linhas: 20, pela proporção da imagem. A grade ficaria com 30 × 20 quadrados.');
    expect(api.calls).toEqual([]);
    button('Apagar e mudar a grade').click();
    await settle();
    expect(api.calls).toEqual(['setGrid map-1 30']);
    expect(changed).toHaveLength(1);
    expect(el.querySelector('h3')).toBeNull();
  });

  it('"Voltar" closes the question, sends nothing and gives the focus back to "Mudar a grade"', async () => {
    await setup(withGrid, { erases: true });
    button('Mudar a grade').click();
    await settle();
    button('Voltar').click();
    await settle();
    expect(api.calls).toEqual([]);
    expect(document.activeElement).toBe(button('Mudar a grade'));
  });

  it('changes straight away, with no warning, when nothing is painted or seen', async () => {
    await setup(withGrid, { erases: false });
    button('Mudar a grade').click();
    await settle();
    expect(text()).not.toContain('apaga o terreno');
    expect(button('Mudar a grade').closest('section')).toBeTruthy();
    type('12');
    Array.from(el.querySelectorAll('button')).filter((b) => b.textContent?.trim() === 'Mudar a grade').at(-1)!.click();
    await settle();
    expect(api.calls).toEqual(['setGrid map-1 12']);
  });

  it('refuses a number out of 4 to 200 and the button cannot act', async () => {
    await setup(withGrid);
    button('Mudar a grade').click();
    await settle();
    type('300');
    expect(text()).toContain('Use um número inteiro de 4 a 200.');
    const go = Array.from(el.querySelectorAll('button')).filter((b) => b.textContent?.trim() === 'Mudar a grade').at(-1)!;
    expect(go.classList).toContain('mr-button--off');
    go.click();
    expect(api.calls).toEqual([]);
  });

  it('while a combat runs the change is off, with the reason in a line', async () => {
    await setup(withGrid, { combatRunning: true });
    const change = button('Mudar a grade');
    expect(change.getAttribute('aria-disabled')).toBe('true');
    expect(change.classList).toContain('mr-button--off');
    expect(change.getAttribute('aria-describedby')).toBe('gp-why');
    expect(text()).toContain('Desligado enquanto o combate dura.');
    change.click();
    await settle();
    expect(el.querySelector('h3')).toBeNull();
  });

  it('without a grid asks for the columns and "Definir a grade"', async () => {
    await setup(mapMessage('map-1', 'Sem grade', { gridColumns: 0, gridRows: 0 }));
    expect(text()).toContain('Este mapa ainda não tem grade.');
    expect(text()).toContain('de 4 a 200');
    type('24');
    expect(text()).toContain('24 × 16 quadrados');
    button('Definir a grade').click();
    await settle();
    expect(api.calls).toEqual(['setGrid map-1 24']);
  });

  it('says why when the server refuses (a combat began meanwhile), by the typed reason', async () => {
    await setup(withGrid);
    api.failWith = new ConnectError('x', Code.FailedPrecondition, undefined, [{ desc: MapBlockedSchema, value: create(MapBlockedSchema, { reason: MapBlockedReason.COMBAT_RUNNING }) }]);
    button('Mudar a grade').click();
    await settle();
    Array.from(el.querySelectorAll('button')).filter((b) => b.textContent?.trim() === 'Mudar a grade').at(-1)!.click();
    await settle();
    expect(text()).toContain('Há um combate neste mapa: a grade e a imagem só mudam depois dele.');
    expect(changed).toEqual([]);
  });
});
