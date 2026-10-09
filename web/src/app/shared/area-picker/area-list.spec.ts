import { TestBed } from '@angular/core/testing';

import type { AreaRow } from '../../core/combat/area-text';
import { AreaList } from './area-list';

const plain = (t: string | null | undefined) =>
  (t ?? '').replace(/ /g, ' ').replace(/\s+/g, ' ').trim();

const row = (over: Partial<AreaRow>): AreaRow => ({
  id: 'x',
  label: 'Goblin 1',
  state: 'Ileso',
  ally: false,
  self: false,
  hidden: false,
  cover: 'Sem cobertura',
  mark: null,
  distance: '',
  ...over,
});

function setup(inputs: Record<string, unknown>) {
  const fixture = TestBed.createComponent(AreaList);
  for (const [k, v] of Object.entries(inputs)) {
    fixture.componentRef.setInput(k, v);
  }
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

describe('AreaList: who is in the area', () => {
  it('lists each creature with its state, its cover as text and the ally tag, under the ally warning', () => {
    const el = setup({
      rows: [
        row({ id: 't', label: 'Toren', ally: true, distance: 'a 3,0 m do ponto' }),
        row({
          id: 'h',
          label: 'Hobgoblin',
          state: 'Ferido',
          cover: 'Três quartos (do mapa): +5 no teste de Destreza',
          mark: 'three',
        }),
      ],
      count: '2 criaturas',
      warning: {
        strong: 'Toren está na área.',
        rest: 'A Bola de Fogo atinge aliados também. Mude o local se não quiser atingi-lo.',
      },
    });
    const alert = el.querySelector('[role="alert"]');
    expect(plain(alert?.textContent)).toContain('Toren está na área.');
    expect(plain(alert?.textContent)).toContain('A Bola de Fogo atinge aliados também.');
    const items = Array.from(el.querySelectorAll('li.row')).map((li) => plain(li.textContent));
    expect(items[0]).toContain('Toren');
    expect(items[0]).toContain('Aliado');
    expect(items[0]).toContain('Sem cobertura · a 3,0 m do ponto');
    expect(items[1]).toContain('Ferido');
    expect(items[1]).toContain('Três quartos (do mapa): +5 no teste de Destreza');
    expect(el.querySelector('li.row .mr-swatch--three')).not.toBeNull();
    expect(plain(el.querySelector('.head__title')?.textContent)).toBe('Quem você vê na área');
    expect(plain(el.querySelector('.head__count')?.textContent)).toBe('2 criaturas');
    expect(plain(el.querySelector('p[role="status"]')?.textContent)).toBe(
      '2 criaturas na área; Toren é aliado',
    );
  });

  it('says a spell whose save is not Dexterity gives no cover bonus', () => {
    const el = setup({
      rows: [row({ cover: 'Sem bônus de cobertura: o teste é de Constituição' })],
      count: '1 criatura',
    });
    expect(plain(el.textContent)).toContain('Sem bônus de cobertura: o teste é de Constituição');
  });

  it('gives way to the confirmation in place when nobody the caster sees is inside', () => {
    const el = setup({
      rows: [],
      nobody: {
        strong: 'Ninguém que você vê está na área.',
        rest: 'A magia gasta o espaço de 3º nível e a sua ação mesmo assim.',
      },
    });
    expect(plain(el.querySelector('[role="alert"]')?.textContent)).toContain(
      'Ninguém que você vê está na área. A magia gasta o espaço de 3º nível e a sua ação mesmo assim.',
    );
    expect(el.querySelector('ul')).toBeNull();
  });

  it("marks the master's hidden creatures, with the count of them", () => {
    const el = setup({
      rows: [row({ id: 'g3', label: 'Goblin 3', hidden: true })],
      count: '1 criatura · 1 escondida',
      heading: 'Na área',
    });
    expect(plain(el.querySelector('li.row')?.textContent)).toContain('Escondido');
    expect(plain(el.querySelector('.head__count')?.textContent)).toBe('1 criatura · 1 escondida');
  });
});
