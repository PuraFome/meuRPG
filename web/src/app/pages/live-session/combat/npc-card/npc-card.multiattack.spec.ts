import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';

import { CombatClient } from '../../../../core/combat/combat-client';
import { CombatUndone } from '../../../../core/combat/combat-undone';
import type { CombatState } from '../../../../core/combat/combat-state';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { fakeCombatClient } from '../../../../core/combat/combat-client-testing';
import { CreaturesClient } from '../../../../core/creatures/creatures-client';
import {
  type Creature,
  CreatureActionSchema,
  CreatureMultiattackRoutineSchema,
  CreatureSchema,
} from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { settle } from '../../../../../test-settle';
import { NpcCard } from './npc-card';

const part = (name: string, count = 1, kind = 'melee') => ({ name, count, kind, countText: '' });
const act = (name: string, namePt: string, hasAttack: boolean, routines: object[][] = []) =>
  create(CreatureActionSchema, {
    name,
    namePt,
    hasAttack,
    multiattack: routines.map((attacks) =>
      create(CreatureMultiattackRoutineSchema, { attacks: attacks as never }),
    ),
  });

const armor = (): Creature =>
  create(CreatureSchema, {
    actions: [
      act('Multiattack', 'Ataques múltiplos', false, [[part('Slam', 2)]]),
      act('Slam', 'Pancada', true),
    ],
  });
const bear = (): Creature =>
  create(CreatureSchema, {
    actions: [
      act('Multiattack', 'Ataques múltiplos', false, [[part('Bite', 1, 'ability'), part('Claws')]]),
      act('Bite', 'Mordida', true),
      act('Claws', 'Garras', true),
    ],
  });
const naga = (): Creature =>
  create(CreatureSchema, {
    actions: [
      act('Multiattack', 'Ataques múltiplos', false, [
        [part('Longsword', 2)],
        [part('Longbow', 1, 'ranged')],
      ]),
      act('Longsword', 'Espada longa', true),
      act('Longbow', 'Arco longo', true),
    ],
  });

const attackOf = (key: string, namePt: string) => ({
  attack: {
    key,
    saveDc: 0,
    attackBonus: 4,
    namePt,
    damage: '1d6',
    damageTypePt: 'concussão',
    rangeFt: 5,
    longRangeFt: 0,
  },
});

/** The options of the turn: the attacks, one target for each, and the economy the server counts. */
function optionsOf(names: [string, string][], per: number, left: number) {
  return {
    options: {
      attacks: names.map(([key, namePt]) => attackOf(key, namePt)),
      spells: [],
      economy: { attacksPerAction: per, attacksLeft: left },
    },
    attackTargets: names.map(([key]) => ({
      attackKey: key,
      targets: [{ combatantId: 'toren', label: 'Toren' }],
    })),
    pendingDamages: [],
    spellTargets: [],
  } as never;
}

async function open(
  block: Creature | null,
  names: [string, string][],
  per: number,
  left: number,
  over = {},
) {
  TestBed.resetTestingModule();
  const statBlock = vi.fn(async () => {
    if (!block) {
      throw new Error('no stat block');
    }
    return block;
  });
  TestBed.configureTestingModule({
    providers: [
      { provide: CombatClient, useValue: fakeCombatClient({}) },
      { provide: CreaturesClient, useValue: { statBlock } },
    ],
  });
  const fixture = TestBed.createComponent(NpcCard);
  const npc = combatant({
    id: 'npc',
    label: 'Armadura 1',
    bestiaryCreatureKey: 'monster:animated-armor',
    actionUsed: left < per,
    ...over,
  });
  fixture.componentRef.setInput('campaignId', 'c');
  fixture.componentRef.setInput('encounter', encounter({ combatants: [npc] }));
  fixture.componentRef.setInput('subject', npc);
  fixture.componentRef.setInput('state', { apply: vi.fn() } as unknown as CombatState);
  fixture.componentRef.setInput('options', optionsOf(names, per, left));
  await settle(fixture);
  return { fixture, root: fixture.nativeElement as HTMLElement, statBlock };
}

const text = (root: HTMLElement) => (root.textContent ?? '').replace(/\s+/g, ' ');
const button = (root: HTMLElement, label: string) =>
  [...root.querySelectorAll('button')].find((b) => (b.textContent ?? '').includes(label));

describe("NpcCard: the creature's Multiattack", () => {
  it('shows the routine of the animated armor, "Ataques múltiplos: 2 × Pancada", and the action still available', async () => {
    const { root, statBlock } = await open(armor(), [['basic:0', 'Pancada']], 2, 2);
    expect(statBlock).toHaveBeenCalledWith('c', 'monster:animated-armor');
    expect(text(root)).toContain('Ataques múltiplos: 2 × Pancada');
    expect(button(root, 'Encerrar os ataques')).toBeUndefined(); // nothing made yet
  });

  it('after the first slam the action is under way, not "Usada", and the second slam is offered with "Encerrar os ataques"', async () => {
    const { root } = await open(armor(), [['basic:0', 'Pancada']], 2, 1);
    const chips = text(root);
    expect(chips).not.toContain('Ação Usada');
    expect(chips).toContain('Ação 1 de 2');
    expect(text(root)).toContain('1 de 2 ataques feitos');
    expect(root.querySelector('app-roll-picker')).not.toBeNull(); // the next roll
    expect(button(root, 'Encerrar os ataques')).toBeDefined();
  });

  it('when the routine is done the action reads "Usada" and no roll is offered', async () => {
    const { root } = await open(armor(), [['basic:0', 'Pancada']], 2, 0);
    expect(text(root)).toContain('Ação Usada');
    expect(text(root)).toContain('Ataques múltiplos concluídos');
    expect(root.querySelector('app-roll-picker')).toBeNull();
    expect(button(root, 'Encerrar os ataques')).toBeUndefined();
  });

  it('"Encerrar os ataques" counts the action as used before the routine is done, and an undo brings the rolls back', async () => {
    const { fixture, root } = await open(armor(), [['basic:0', 'Pancada']], 2, 1);
    button(root, 'Encerrar os ataques')?.click();
    await settle(fixture);
    expect(text(root)).toContain('Ação Usada');
    expect(root.querySelector('app-roll-picker')).toBeNull();

    TestBed.inject(CombatUndone).mark();
    await settle(fixture);
    expect(text(root)).toContain('Ação 1 de 2');
    expect(root.querySelector('app-roll-picker')).not.toBeNull();
  });

  it('a new turn starts the routine again', async () => {
    const { fixture, root } = await open(armor(), [['basic:0', 'Pancada']], 2, 1);
    button(root, 'Encerrar os ataques')?.click();
    await settle(fixture);
    fixture.componentRef.setInput('options', optionsOf([['basic:0', 'Pancada']], 2, 2));
    fixture.componentRef.setInput(
      'subject',
      combatant({ id: 'npc', label: 'Armadura 1', bestiaryCreatureKey: 'monster:animated-armor' }),
    );
    await settle(fixture);
    expect(text(root)).toContain('Ação Disponível');
    expect(root.querySelector('app-roll-picker')).not.toBeNull();
  });

  it('the brown bear shows bite and claws in one routine', async () => {
    const { root } = await open(
      bear(),
      [
        ['basic:0', 'Mordida'],
        ['basic:1', 'Garras'],
      ],
      2,
      2,
    );
    expect(text(root)).toContain('Ataques múltiplos: 1 × Mordida + 1 × Garras');
  });

  it('a creature with a choice between routines lets the master pick one, and a shorter routine is done sooner', async () => {
    const { fixture, root } = await open(
      naga(),
      [
        ['basic:0', 'Espada longa'],
        ['basic:1', 'Arco longo'],
      ],
      2,
      1,
    );
    const pick = fixture.componentInstance as unknown as { pickRoutine(i: number): void };
    expect(root.querySelector('[data-routine-select]')).not.toBeNull();
    expect(text(root)).toContain('2 × Espada longa');
    expect(text(root)).toContain('1 × Arco longo');
    pick.pickRoutine(1);
    await settle(fixture);
    // One attack was made and the bow's routine has one: it is done.
    expect(text(root)).toContain('Ação Usada');
  });

  it('a creature without Multiattack, or with no stat block to read, keeps the card as it was', async () => {
    const plain = create(CreatureSchema, { actions: [act('Slam', 'Pancada', true)] });
    for (const block of [plain, null]) {
      const { root } = await open(block, [['basic:0', 'Pancada']], 1, 0);
      expect(text(root)).not.toContain('Ataques múltiplos');
      expect(text(root)).toContain('Ação Usada');
      expect(button(root, 'Encerrar os ataques')).toBeUndefined();
      expect(root.querySelector('app-roll-picker')).not.toBeNull();
    }
  });
});
