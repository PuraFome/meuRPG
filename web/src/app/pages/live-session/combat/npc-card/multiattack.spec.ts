import { create } from '@bufbuild/protobuf';

import {
  type Attack,
  AttackSchema,
  CreatureActionSchema,
  CreatureMultiattackRoutineSchema,
  CreatureSchema,
} from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { routinesOf } from './multiattack';

const part = (name: string, count = 1, kind = 'melee', countText = '') => ({
  name,
  count,
  kind,
  countText,
});

const action = (name: string, namePt: string, hasAttack: boolean, routines: object[][] = []) =>
  create(CreatureActionSchema, {
    name,
    namePt,
    hasAttack,
    multiattack: routines.map((attacks) =>
      create(CreatureMultiattackRoutineSchema, { attacks: attacks as never }),
    ),
  });

const attack = (key: string, namePt: string): Attack =>
  create(AttackSchema, { key, name: namePt, namePt });

describe('routinesOf: the Multiattack routines of a creature', () => {
  it('is empty with no stat block and for a creature with no Multiattack', () => {
    expect(routinesOf(null, [])).toEqual([]);
    const plain = create(CreatureSchema, { actions: [action('Slam', 'Pancada', true)] });
    expect(routinesOf(plain, [attack('basic:0', 'Pancada')])).toEqual([]);
  });

  it('the animated armor has one routine, two slams, written in Portuguese', () => {
    const armor = create(CreatureSchema, {
      actions: [
        action('Multiattack', 'Ataques múltiplos', false, [[part('Slam', 2)]]),
        action('Slam', 'Pancada', true),
      ],
    });
    const [only, ...rest] = routinesOf(armor, [attack('basic:0', 'Pancada')]);
    expect(rest).toEqual([]);
    expect(only.label).toBe('2 × Pancada');
    expect(only.total).toBe(2);
    expect(only.parts).toEqual([{ name: 'Pancada', count: 2, attackKey: 'basic:0' }]);
  });

  it('the brown bear: bite and claws are one routine of two attacks, each with its own attack', () => {
    const bear = create(CreatureSchema, {
      actions: [
        action('Multiattack', 'Ataques múltiplos', false, [
          [part('Bite', 1, 'ability'), part('Claws', 1)],
        ]),
        action('Bite', 'Mordida', true),
        action('Claws', 'Garras', true),
      ],
    });
    const [only] = routinesOf(bear, [attack('basic:0', 'Mordida'), attack('basic:1', 'Garras')]);
    expect(only.label).toBe('1 × Mordida + 1 × Garras');
    expect(only.total).toBe(2);
    expect(only.parts.map((p) => p.attackKey)).toEqual(['basic:0', 'basic:1']);
  });

  it('a choice between routines lists each one, and a part that is no attack (a spell, a gaze) does not count', () => {
    const naga = create(CreatureSchema, {
      actions: [
        action('Multiattack', 'Ataques múltiplos', false, [
          [part('Longsword', 2)],
          [part('Longbow', 1, 'ranged')],
          [part('Frightful Presence', 1, 'ability'), part('Longsword', 1)],
        ]),
        action('Longsword', 'Espada longa', true),
        action('Longbow', 'Arco longo', true),
        action('Frightful Presence', 'Presença Aterradora', false),
      ],
    });
    const routines = routinesOf(naga, [
      attack('basic:0', 'Espada longa'),
      attack('basic:1', 'Arco longo'),
    ]);
    expect(routines.map((r) => r.total)).toEqual([2, 1, 1]);
    expect(routines[2].label).toBe('1 × Presença Aterradora + 1 × Espada longa');
  });
});
