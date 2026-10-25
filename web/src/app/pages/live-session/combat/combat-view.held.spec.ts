// The master's pending questions inside the order of a combat (a held roll waits for the roller; after ten seconds the master may
// answer too), the chip that counts them, and the roller's notice when the master answered for them.

import {
  CombatantKind,
  CombatantSide,
  EncounterMode,
  HeldRollKind,
} from '../../../../gen/meurpg/play/v1/combat_pb';
import { combatant, encounter } from '../../../core/combat/combat-testing';
import { forgetAnsweredHere } from '../../../core/held-rolls/held-roll-controller';
import { askedAt, heldDie, heldLucky } from '../../../core/held-rolls/held-roll-testing';
import { mountCombatView } from './combat-view-testing';

const plain = (t: string | null | undefined) => (t ?? '').replace(/\s+/g, ' ').trim();

/** `seconds` after the roll was held. */
const ago = (seconds: number) => askedAt(Date.now() - seconds * 1000);

const rows = (brisa = {}, pens = {}) => [
  combatant({
    id: 'b',
    label: 'Brisa',
    kind: CombatantKind.PLAYER,
    side: CombatantSide.PARTY,
    characterId: 'brisa-c',
    hitPointsCurrent: 21,
    hitPointsMax: 24,
    initiative: 18,
    ...brisa,
  }),
  combatant({ id: 'g', label: 'Goblin chefe', initiative: 15 }),
  combatant({
    id: 'p',
    label: 'Pensantus',
    kind: CombatantKind.PLAYER,
    side: CombatantSide.PARTY,
    characterId: 'pens-c',
    hitPointsCurrent: 9,
    hitPointsMax: 20,
    initiative: 12,
    ...pens,
  }),
];

const theatre = (combatants: ReturnType<typeof rows>, revision = 1) =>
  encounter({
    mode: EncounterMode.THEATRE,
    mapId: '',
    gridColumns: 0,
    gridRows: 0,
    revision,
    currentCombatantId: 'b',
    combatants,
  });

describe('CombatView: the questions of the held rolls', () => {
  beforeEach(() => forgetAnsweredHere());

  it('puts the question inside the row of the roller, with the attack and its target', async () => {
    const lucky = heldLucky([1], 5, {
      holdId: 'hb',
      combatantId: 'b',
      rollerLabel: 'Brisa',
      kind: HeldRollKind.WEAPON_ATTACK,
      targetId: 'g',
      ...ago(3),
    });
    const { fixture, el } = mountCombatView({
      enc: theatre(rows({ heldRollOffer: lucky })),
      master: true,
    });
    await fixture.whenStable();
    fixture.detectChanges();
    // The question is in "Turno", with the other answers that wait, and names who rolled.
    const section = el.querySelector('app-held-question section');
    expect(plain(section?.querySelector('h3')?.textContent)).toBe(
      'Brisa · Tirou 1: rolar de novo?',
    );
    expect(plain(section?.textContent)).toContain('Ataque com arma contra Goblin chefe.');
    expect(plain(section?.querySelector('.q__status')?.textContent)).toMatch(
      /Brisa responde primeiro.*você em [67] s/,
    );
    const buttons = Array.from(section?.querySelectorAll('button') ?? []);
    expect(buttons.map((b) => plain(b.textContent))).toEqual([
      'Rolar de novo',
      'Digitar o resultado',
      'Ficar com o 1',
    ]);
    expect(buttons.every((b) => b.disabled)).toBe(true);
  });

  it('counts the pending questions in a chip of the combat’s title row and announces the count politely', async () => {
    const lucky = heldLucky([1], 5, { holdId: 'hb', combatantId: 'b', ...ago(3) });
    const die = heldDie(8, 2, 8, {
      holdId: 'hp',
      combatantId: 'p',
      rollerLabel: 'Pensantus',
      ...ago(30),
    });
    const { fixture, el } = mountCombatView({
      enc: theatre(rows({ heldRollOffer: lucky }, { heldRollOffer: die })),
      master: true,
      matches: ['(min-width: 1024px)'],
    });
    await fixture.whenStable();
    fixture.detectChanges();
    expect(plain(el.querySelector('.bar__held')?.textContent)).toContain('2 perguntas pendentes');
    expect(
      plain(
        el.querySelector('.mr-visually-hidden[aria-live="polite"][data-testid="held-count"]')
          ?.textContent,
      ),
    ).toBe('2 perguntas pendentes');
    // The one that waited more than ten seconds can be answered already.
    const pens = Array.from(el.querySelectorAll('app-held-question')).find((q) =>
      q.textContent?.includes('Pensantus'),
    );
    expect(plain(pens?.querySelector('.q__status')?.textContent)).toContain(
      'Pensantus não respondeu',
    );
    expect(pens?.querySelector<HTMLButtonElement>('section button')?.disabled).toBe(false);
  });

  it('shows no chip and no question when nothing waits', async () => {
    const { fixture, el } = mountCombatView({
      enc: theatre(rows()),
      master: true,
      matches: ['(min-width: 1024px)'],
    });
    await fixture.whenStable();
    fixture.detectChanges();
    expect(el.querySelector('.bar__held')).toBeNull();
    expect(el.querySelector('app-held-question')).toBeNull();
  });

  it('follows the combat: a question the encounter stops carrying says that the roller answered', async () => {
    const lucky = heldLucky([1], 5, { holdId: 'hb', combatantId: 'b', ...ago(3) });
    const { fixture, el, state } = mountCombatView({
      enc: theatre(rows({ heldRollOffer: lucky })),
      master: true,
    });
    await fixture.whenStable();
    fixture.detectChanges();
    state.apply(theatre(rows(), 2));
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    expect(plain(el.querySelector('app-held-question')?.textContent)).toContain('Brisa respondeu.');
  });

  it('tells the roller the master answered when their question vanishes and they did not answer it here', async () => {
    const lucky = heldLucky([1], 5, { holdId: 'hb', combatantId: 'b', ...ago(3) });
    const mine = { mine: true };
    const { fixture, el, state } = mountCombatView({
      enc: theatre(rows({ heldRollOffer: lucky, ...mine })),
    });
    await fixture.whenStable();
    fixture.detectChanges();
    expect(el.textContent).not.toContain('O mestre respondeu por você.');
    state.apply(theatre(rows(mine), 2));
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    expect(plain(el.querySelector('[data-testid="held-vanished"]')?.textContent)).toContain(
      'O mestre respondeu por você.',
    );
    el.querySelector<HTMLButtonElement>('[data-testid="held-vanished"] button')?.click();
    fixture.detectChanges();
    expect(el.querySelector('[data-testid="held-vanished"]')).toBeNull();
  });
});
