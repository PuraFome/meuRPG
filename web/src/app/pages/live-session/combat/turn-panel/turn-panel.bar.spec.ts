import { TestBed } from '@angular/core/testing';

import { type Combatant, CombatantKind } from '../../../../../gen/meurpg/play/v1/combat_pb';
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
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const buttons = () =>
      [...el.querySelectorAll('button')].map((b) =>
        (b.textContent ?? '').replace(/\s+/g, ' ').trim(),
      );
    return { el, buttons };
  }
  it('is the master\'s band: the map and the round, whose turn it is and who is next, and "Encerrar turno" alone', () => {
    const { el, buttons } = setup();
    expect(el.querySelector('.banner h2')?.textContent).toBe('Sua vez, Pensantus');
    expect(el.querySelector('.banner__round')?.textContent).toMatch(/Combate · .*Rodada/s);
    expect(el.querySelector('.banner__sub')?.textContent).toMatch(/Em seguida: Brisa/);
    // The economy and "Mover" are the first row of "Ações" now, not the band's (584-02).
    expect(el.querySelector('app-economy-shapes')).toBeNull();
    expect(buttons().filter((b) => b.endsWith('Mover'))).toHaveLength(0);
    expect(buttons().filter((b) => b.endsWith('Encerrar turno'))).toHaveLength(1);
    expect(el.querySelector('.banner app-end-turn .end--filled')).not.toBeNull();
  });

  it('puts one chip with the form and its time under the title, and no card for the form', () => {
    const { el } = setup(
      { wildShapeBeastKey: 'monster:wolf', wildShapeBeastNamePt: 'Lobo' },
      { formTime: '52 min restantes' },
    );
    const chip = el.querySelector('.banner app-form-chip');
    expect(chip?.textContent?.replace(/\s+/g, ' ').trim()).toContain('Na forma de Lobo');
    expect(chip?.textContent).toContain('52 min');
    expect(chip?.textContent).not.toContain('restantes');
    expect(el.querySelector('app-wild-band')).toBeNull();
  });

  it('gives a form with no time readable (another reader) the name alone', () => {
    const { el } = setup({ wildShapeBeastKey: 'monster:wolf', wildShapeBeastNamePt: 'Lobo' });
    expect(el.querySelector('.banner app-form-chip')?.textContent).not.toContain('·');
  });
});
