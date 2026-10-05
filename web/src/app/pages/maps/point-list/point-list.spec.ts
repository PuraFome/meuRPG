import { create } from '@bufbuild/protobuf';
import { ComponentFixture, TestBed } from '@angular/core/testing';

import { MapPointKind, MapPointSchema, TrapState } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { TrapTrigger } from '../../../../gen/meurpg/rules/v1/rules_pb';
import { PointList, pointSubLine } from './point-list';

const pit = create(MapPointSchema, { id: 'p1', kind: MapPointKind.TRAP, name: 'Fosso escondido', trap: { findDc: 15, noticeDc: 15, areaSize: 2, trigger: TrapTrigger.ENTER, state: TrapState.ARMED } });
const needle = create(MapPointSchema, { id: 'p2', kind: MapPointKind.TRAP, name: 'Agulha envenenada', trap: { findDc: 20, areaSize: 1, trigger: TrapTrigger.MANUAL, state: TrapState.ARMED } });
const chest = create(MapPointSchema, { id: 'p3', kind: MapPointKind.TREASURE, name: 'Baú de moedas', treasureValuePo: 250 });
const found = create(MapPointSchema, { id: 'p4', kind: MapPointKind.TREASURE, name: 'Baú achado', treasureValuePo: 1000, revealed: true, treasureFoundAt: { seconds: 1n, nanos: 0 } });
const torch = create(MapPointSchema, { id: 'p5', kind: MapPointKind.LIGHT, name: 'Tocha da guarita', light: { presetKey: 'light:torch', brightFt: 20, dimFt: 20 } });
const scene = create(MapPointSchema, { id: 'p6', kind: MapPointKind.SCENE, name: 'Taverna', revealed: true });

const plain = (s: string) => s.replace(/ /g, ' ');

describe('pointSubLine', () => {
  it('writes the numbers that matter, by kind', () => {
    expect(plain(pointSubLine(pit))).toBe('Armadilha · área 2×2 · notar CD 15 · achar CD 15');
    expect(plain(pointSubLine(needle))).toBe('Armadilha · manual · achar CD 20');
    expect(plain(pointSubLine(chest))).toBe('Tesouro · 250 PO');
    expect(plain(pointSubLine(torch, 'Tocha'))).toBe('Luz · Tocha · 6 m claro + 6 m de penumbra');
    expect(pointSubLine(scene)).toBe('Cena de RP');
  });
});

describe('PointList', () => {
  let fixture: ComponentFixture<PointList>;
  let el: HTMLElement;
  let picked: string[];

  function setup(points = [torch, pit, needle, chest, found, scene], selectedId: string | null = null) {
    picked = [];
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({});
    fixture = TestBed.createComponent(PointList);
    fixture.componentRef.setInput('points', points);
    fixture.componentRef.setInput('selectedId', selectedId);
    fixture.componentInstance.pick.subscribe((id) => picked.push(id));
    fixture.detectChanges();
    el = fixture.nativeElement;
  }
  const rows = () => Array.from(el.querySelectorAll<HTMLElement>('button.pl__row'));
  const row = (name: string) => rows().find((r) => r.textContent?.includes(name))!;

  it('lists every point with its state in words, never colour alone', () => {
    setup();
    expect(rows()).toHaveLength(6);
    expect(row('Fosso escondido').textContent).toContain('Armada');
    expect(row('Fosso escondido').textContent).toContain('Escondido');
    expect(row('Baú de moedas').textContent).toContain('Não encontrado');
    expect(row('Baú achado').textContent).toContain('Encontrado');
    expect(row('Baú achado').textContent).not.toContain('Escondido');
    expect(row('Taverna').textContent).not.toContain('Escondido');
  });

  it('picks a point by its row, and tells which is the chosen one', () => {
    setup(undefined, 'p2');
    row('Agulha envenenada').click();
    expect(picked).toEqual(['p2']);
    expect(row('Agulha envenenada').getAttribute('aria-pressed')).toBe('true');
    expect(row('Fosso escondido').getAttribute('aria-pressed')).toBe('false');
  });

  it('invites the first point when there is none', () => {
    setup([]);
    expect(el.textContent).toContain('Nenhum ponto ainda.');
  });
});
