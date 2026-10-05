import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';

import { MapPointKind, MapPointSchema, TrapRevealHow } from '../../../../../gen/meurpg/maps/v1/maps_pb';
import { MapsClient } from '../../../../core/maps/maps-client';
import { TrapRevealSheet, type TrapRevealData, trapRevealLabel } from './trap-reveal-sheet';

const PLAYERS = [
  { id: 'p', name: 'Pensantus', playerName: 'Vinicius' },
  { id: 't', name: 'Toren', playerName: 'Caio' },
  { id: 'b', name: 'Brisa', playerName: 'Lia' },
];

describe('trapRevealLabel', () => {
  it('says who gets it, with no article', () => {
    expect(trapRevealLabel([], 3, 3)).toBe('Revelar a armadilha');
    expect(trapRevealLabel(['Toren'], 3, 3)).toBe('Revelar para Toren');
    expect(trapRevealLabel(['A', 'B'], 3, 3)).toBe('Revelar para 2 jogadores');
    expect(trapRevealLabel(['A', 'B', 'C'], 3, 3)).toBe('Revelar para todos');
    expect(trapRevealLabel(['A', 'B'], 2, 3)).toBe('Revelar aos outros');
  });
});

describe('TrapRevealSheet', () => {
  function setup() {
    const calls: unknown[][] = [];
    const api = { revealTrap: async (...a: unknown[]) => (calls.push(a), create(MapPointSchema, { id: 'x', name: 'Fosso escondido' })) };
    const close = vi.fn();
    const point = create(MapPointSchema, {
      id: 'x',
      kind: MapPointKind.TRAP,
      name: 'Fosso escondido',
      description: 'No corredor',
      trapRevealedTo: [{ characterId: 'b', characterName: 'Brisa', how: TrapRevealHow.SEARCHED }],
    });
    const data: TrapRevealData = { campaignId: 'c', mapId: 'm', point, players: PLAYERS };
    TestBed.configureTestingModule({
      providers: [
        { provide: MapsClient, useValue: api },
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close } },
      ],
    });
    const fixture = TestBed.createComponent(TrapRevealSheet);
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement, calls, close };
  }

  it('opens with nobody checked, and the one who already knows is checked, disabled and says why', () => {
    const { el } = setup();
    const boxes = Array.from(el.querySelectorAll<HTMLInputElement>('input[type=checkbox]'));
    expect(boxes.filter((b) => b.checked && !b.disabled)).toHaveLength(0);
    const brisa = boxes.find((b) => b.closest('label')?.textContent?.includes('Brisa'))!;
    expect(brisa.checked && brisa.disabled).toBe(true);
    expect(el.textContent).toContain('já achou esta armadilha');
    expect(el.textContent).toContain('Ninguém marcado');
    expect(el.querySelector('.pf__off')?.getAttribute('aria-disabled')).toBe('true');
  });

  it('reveals to the chosen character and names who receives it on the button', async () => {
    const { fixture, el, calls, close } = setup();
    el.querySelectorAll<HTMLInputElement>('input[type=checkbox]')[1].click();
    fixture.detectChanges();
    const go = el.querySelector<HTMLButtonElement>('.pf__go')!;
    expect(go.textContent).toContain('Revelar para Toren');
    go.click();
    await fixture.whenStable();
    expect(calls).toEqual([['c', 'm', 'x', { characterIds: ['t'] }]]);
    expect(close).toHaveBeenCalled();
  });
});
