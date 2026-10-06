import { ComponentFixture, TestBed } from '@angular/core/testing';

import { DungeonDoorKind } from '../../../gen/meurpg/maps/v1/dungeons_pb';
import { previewResponse } from '../../core/maps/dungeons-testing';
import { DungeonPreview, type DungeonLayout } from './dungeon-preview';

function layoutOf(partial: Partial<DungeonLayout> = {}): DungeonLayout {
  const p = previewResponse();
  return { width: p.width, height: p.height, open: p.open, doors: p.doors, stairs: p.stairs, roomCount: p.rooms.length, ...partial };
}

describe('DungeonPreview (E10-05 1)', () => {
  let fixture: ComponentFixture<DungeonPreview>;
  let el: HTMLElement;

  function render(layout: DungeonLayout) {
    TestBed.configureTestingModule({});
    fixture = TestBed.createComponent(DungeonPreview);
    fixture.componentRef.setInput('layout', layout);
    fixture.detectChanges();
    el = fixture.nativeElement;
  }

  it('is a picture named by its counts, with the walls as one shape and the grid drawn under them', () => {
    render(layoutOf());
    const picture = el.querySelector('[role="img"]')!;
    expect(picture.getAttribute('aria-label')!.replace(/\u00a0/g, ' ')).toBe('Prévia da masmorra: 2 salas, 4 portas e 1 passagem, 2 escadas, 11 × 9 quadrados (16,5 × 13,5 m).');
    expect(el.querySelectorAll('svg path.dp__veil')).toHaveLength(1);
    expect(el.querySelector('svg path.dp__hatch')?.getAttribute('fill')).toMatch(/^url\(#dp-hatch-\d+\)$/);
    expect(el.querySelector('svg')?.getAttribute('viewBox')).toBe('0 0 11 9');
    // The picture keeps the dungeon's proportion.
    expect((picture as HTMLElement).style.aspectRatio).toContain('11 / 9');
  });

  it('draws each door by its kind with the one mark the map uses, and none for a passage', () => {
    render(layoutOf());
    // Four doors (locked, barred, closed, secret) and one passage: four marks.
    expect(el.querySelectorAll('app-map-layers app-door-mark')).toHaveLength(4);
    expect(el.querySelectorAll('app-map-layers .sq--door')).toHaveLength(4);
    // The secret door is drawn with its own mark (the frame and the keyhole), the locked one with the padlock.
    expect(el.querySelector('.dm__secret')).toBeTruthy();
    expect(el.querySelector('.dm__lock')).toBeTruthy();
  });

  it('places a door on its square and turns it with the wall', () => {
    render(layoutOf());
    const squares = Array.from(el.querySelectorAll<HTMLElement>('app-map-layers .sq--door'));
    // The locked door of the sample is at column 2, row 4 of an 11 × 9 grid.
    const locked = squares.find((s) => s.querySelector('.dm__lock'))!;
    expect(parseFloat(locked.style.left)).toBeCloseTo((2 / 11) * 100, 3);
    expect(parseFloat(locked.style.top)).toBeCloseTo((4 / 9) * 100, 3);
  });

  it('draws the stairs as badges with an arrow, up and down, centred on their squares', () => {
    render(layoutOf());
    const stairs = Array.from(el.querySelectorAll<HTMLElement>('.dp__stair'));
    expect(stairs).toHaveLength(2);
    expect(stairs.map((s) => s.querySelector('mat-icon')?.textContent)).toEqual(['arrow_upward', 'arrow_downward']);
    expect(parseFloat(stairs[0]!.style.left)).toBeCloseTo((1.5 / 11) * 100, 3);
    expect(parseFloat(stairs[1]!.style.top)).toBeCloseTo((3.5 / 9) * 100, 3);
  });

  it('names under it only what it draws, in the language\'s order, the master\'s marks with the crossed eye', () => {
    render(layoutOf());
    const legend = el.querySelector('ul.mr-legend')!;
    const names = Array.from(legend.querySelectorAll('li')).map((li) => li.textContent!.replace(/visibility_off|arrow_(?:up|down)ward/g, '').trim());
    expect(names).toEqual(['Parede', 'Porta fechada', 'Porta trancada (só você vê)', 'Grade', 'Porta secreta (só você vê)', 'Escada para cima', 'Escada para baixo']);
    expect(legend.querySelectorAll('li .nm mat-icon')).toHaveLength(2);
  });

  it('has no legend entry for a kind it does not draw (all doors are passages: only the wall)', () => {
    const p = previewResponse();
    render(layoutOf({ doors: p.doors.filter((d) => d.kind === DungeonDoorKind.ARCHWAY), stairs: [] }));
    const names = Array.from(el.querySelectorAll('ul.mr-legend li')).map((li) => li.textContent!.trim());
    expect(names).toEqual(['Parede']);
    expect(el.querySelectorAll('app-map-layers app-door-mark')).toHaveLength(0);
    expect(el.querySelector('.dp__counts')?.textContent).toContain('nenhuma escada');
  });
});
