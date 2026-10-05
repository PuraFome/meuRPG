import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';

import { MapPointKind, MapPointSchema } from '../../../../../gen/meurpg/maps/v1/maps_pb';
import { type FoundChoice, TreasureCard } from './treasure-card';

const PEOPLE = [
  { id: 'p', name: 'Pensantus', sub: 'Mago 4, de Vinicius' },
  { id: 'b', name: 'Brisa', sub: 'Ladina 4, de Lia' },
];
const hidden = () => create(MapPointSchema, { id: 't', kind: MapPointKind.TREASURE, name: 'Baú de moedas', treasureValuePo: 250, description: '250 PO e uma adaga de prata.' });
const found = (extra = {}) =>
  create(MapPointSchema, { id: 't', kind: MapPointKind.TREASURE, name: 'Baú de moedas', treasureValuePo: 250, description: '250 PO e uma adaga de prata.', treasureFoundAt: timestampFromDate(new Date(2026, 9, 4, 21, 40)), treasureFoundBy: [{ characterId: 'b', characterName: 'Brisa' }], ...extra });

function setup(point = hidden()) {
  const fixture = TestBed.createComponent(TreasureCard);
  fixture.componentRef.setInput('point', point);
  fixture.componentRef.setInput('people', PEOPLE);
  const marks: FoundChoice[] = [];
  const unmarks: string[] = [];
  fixture.componentInstance.mark.subscribe((m) => marks.push(m));
  fixture.componentInstance.unmark.subscribe((p) => unmarks.push(p.id));
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  const button = (t: string) => Array.from(el.querySelectorAll('button')).find((b) => b.textContent?.includes(t))!;
  return { fixture, el, marks, unmarks, button };
}

describe('TreasureCard', () => {
  it('a hidden treasure says it is the master\'s alone until found', () => {
    const { el } = setup();
    expect(el.textContent).toContain('Escondido');
    expect(el.textContent).toContain('O que tem dentro · só você vê até achar');
    expect(el.textContent?.replace(/\u00a0/g, ' ')).toContain('Tesouro · valor 250');
  });

  it('opens the form in place with nobody checked, and needs at least one person', () => {
    const { fixture, el, button, marks } = setup();
    button('Marcar como encontrado').click();
    fixture.detectChanges();
    expect(el.querySelector('[role=dialog]')).toBeNull();
    expect(el.querySelectorAll('input[type=checkbox]:checked')).toHaveLength(0);
    expect(el.textContent).toContain('Quem encontrou o Baú de moedas?');
    expect(el.querySelector('.pf__off')?.getAttribute('aria-disabled')).toBe('true');
    el.querySelectorAll<HTMLInputElement>('input[type=checkbox]')[1].click();
    fixture.detectChanges();
    expect(el.textContent).toContain('No resumo da sessão: Brisa');
    (el.querySelector('.pf__go') as HTMLButtonElement).click();
    expect(marks).toEqual([{ point: expect.anything(), characterIds: ['b'] }]);
  });

  it('"Voltar" closes the form and gives the focus back to the button', async () => {
    const { fixture, el, button } = setup();
    button('Marcar como encontrado').click();
    fixture.detectChanges();
    button('Voltar').click();
    await fixture.whenStable();
    fixture.detectChanges();
    expect(el.querySelector('.tr__form')).toBeNull();
  });

  it('a found treasure says who and when, and "Desmarcar" asks in place', () => {
    const { fixture, el, button, unmarks } = setup(found());
    expect(el.textContent?.replace(/\s+/g, ' ')).toContain('Encontrado por Brisa às 21:40');
    button('Desmarcar').click();
    fixture.detectChanges();
    expect(el.querySelector('[role=alertdialog]')?.textContent).toContain('Desmarcar o Baú de moedas?');
    const buttons = Array.from(el.querySelectorAll('.tr__pair button')) as HTMLButtonElement[];
    buttons[1].click();
    expect(unmarks).toEqual(['t']);
  });

  it('a converted treasure has the lock and the way to undo, and no "Desmarcar"', () => {
    const { el } = setup(found({ treasureConverted: true }));
    expect(el.textContent).toContain('Convertido em XP');
    expect(el.textContent).toContain('desfaça esse XP na página da campanha');
    expect(Array.from(el.querySelectorAll('button')).some((b) => b.textContent?.includes('Desmarcar'))).toBe(false);
  });
});
