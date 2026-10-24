import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';

import { type Combatant, CombatantKind } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { ContestTurnStateSchema } from '../../../../../gen/meurpg/play/v1/contest_types_pb';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { TurnPanel } from './turn-panel';

/** The turn bar of 1024px and up (C6-07): the shapes, "Mover" and "Encerrar turno" on one row with the title. */
describe('TurnPanel: the turn bar from 1024px', () => {
  beforeEach(() => {
    vi.stubGlobal('matchMedia', (query: string) => ({
      matches: query.includes('min-width: 1024') || query.includes('min-width: 1280'),
      media: query,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
    }));
  });
  afterEach(() => vi.unstubAllGlobals());

  const order = (over: Partial<Combatant> = {}) => [
    combatant({ id: 'brisa', label: 'Brisa', kind: CombatantKind.PLAYER }),
    combatant({
      id: 'pen',
      label: 'Pensantus',
      kind: CombatantKind.PLAYER,
      mine: true,
      speedFt: 30,
      movementLeftFt: 30,
      speedDft: 300,
      movementLeftDft: 300,
      ...over,
    }),
  ];

  function setup(
    own: Parameters<typeof order>[0] = {},
    inputs: Record<string, unknown> = {},
    current = 'pen',
  ) {
    const fixture = TestBed.createComponent(TurnPanel);
    fixture.componentRef.setInput(
      'encounter',
      encounter({ combatants: order(own), currentCombatantId: current }),
    );
    for (const [k, v] of Object.entries(inputs)) {
      fixture.componentRef.setInput(k, v);
    }
    const moves: number[] = [];
    fixture.componentInstance.move.subscribe(() => moves.push(1));
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const buttons = () =>
      [...el.querySelectorAll('button')].map((b) =>
        (b.textContent ?? '').replace(/\s+/g, ' ').trim(),
      );
    return { el, moves, buttons };
  }
  const names = (el: HTMLElement) =>
    [...el.querySelectorAll('.banner__shapes li')].map((li) =>
      li.getAttribute('aria-label')?.replace(/ /g, ' '),
    );

  it('is one row: whose turn it is and who is next, the four shapes, "Mover" and "Encerrar turno"', () => {
    const { el, buttons } = setup();
    expect(el.querySelector('.banner h2')?.textContent).toBe('Sua vez, Pensantus');
    expect(el.querySelector('.banner__sub')?.textContent).toMatch(/depois de você: Brisa/);
    expect(names(el)).toEqual([
      'Ação disponível',
      'Ação bônus disponível',
      'Reação disponível',
      'Movimento: 9,0 m',
    ]);
    expect(buttons().filter((b) => b.endsWith('Mover'))).toHaveLength(1);
    expect(buttons().filter((b) => b.endsWith('Encerrar turno'))).toHaveLength(1);
  });

  it('opens the move page with "Mover"', () => {
    const { el, moves } = setup();
    el.querySelector<HTMLButtonElement>('.banner__move')!.click();
    expect(moves).toEqual([1]);
  });

  it('turns "Mover" off while an answer is awaited, and leaves it out when nothing is left to move', () => {
    const waiting = setup({}, { waiting: { title: 'Esperando a reação do mestre', detail: '' } });
    const move = waiting.el.querySelector<HTMLButtonElement>('.banner__move')!;
    expect(move.getAttribute('aria-disabled') === 'true' || move.disabled).toBe(true);
    move.click();
    expect(waiting.moves).toEqual([]);
    TestBed.resetTestingModule();
    const still = setup({ movementLeftFt: 0, movementLeftDft: 0 });
    expect(still.el.querySelector('.banner__move')).toBeNull();
    expect(names(still.el)).toHaveLength(3);
  });

  it('leaves "Mover" out for a grappled character and for a combat without a map', () => {
    const grappled = setup({}, { contest: create(ContestTurnStateSchema, { grappled: true }) });
    expect(grappled.el.querySelector('.banner__move')).toBeNull();
    TestBed.resetTestingModule();
    const theatre = setup({}, { theatre: true });
    expect(theatre.el.querySelector('.banner__move')).toBeNull();
    expect(names(theatre.el)).toHaveLength(3);
  });

  it('says the reaction in the rail, not in the card, off turn', () => {
    const { el } = setup({}, {}, 'brisa');
    expect(el.textContent).not.toContain('Sua reação');
  });
});
