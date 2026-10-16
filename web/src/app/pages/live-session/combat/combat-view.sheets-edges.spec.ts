import { create, type MessageInitShape } from '@bufbuild/protobuf';

import {
  type Combatant,
  CombatantKind,
  CombatantSide,
  EncounterMode,
  OpportunityAttackSchema,
  OpportunityOfferSchema,
} from '../../../../gen/meurpg/play/v1/combat_pb';
import { TurnOptionsSchema } from '../../../../gen/meurpg/rules/v1/rules_pb';
import { combatant, encounter } from '../../../core/combat/combat-testing';
import { mountCombatView } from './combat-view-testing';

// The edges of the sheets the page opens that the other specs leave: an answer that is nothing, a combat that is gone, a
// character that does not exist.

const toren = (over: Partial<Combatant> = {}) =>
  combatant({
    id: 't',
    label: 'Toren',
    kind: CombatantKind.PLAYER,
    side: CombatantSide.PARTY,
    characterId: 'toren-c',
    mine: true,
    col: 1,
    row: 1,
    ...over,
  });
const goblin = () => combatant({ id: 'g', label: 'Goblin', col: 3, row: 3 });
const offer = (over: MessageInitShape<typeof OpportunityOfferSchema> = {}) =>
  create(OpportunityOfferSchema, {
    id: 'o1',
    moverId: 't',
    moverLabel: 'Toren',
    reactorId: 'g',
    reactorLabel: 'Goblin',
    forYou: true,
    attacks: [create(OpportunityAttackSchema, { key: 'scimitar', namePt: 'Cimitarra' })],
    ...over,
  });
const withOffer = () =>
  encounter({
    mode: EncounterMode.GRID,
    currentCombatantId: 't',
    combatants: [toren(), goblin()],
    opportunityOffers: [offer()],
  });
const reactorOptions = {
  options: create(TurnOptionsSchema, {
    attacks: [{ attack: { key: 'scimitar', name: 'Scimitar', namePt: 'Cimitarra' } }],
  }),
  attackTargets: [],
  spellTargets: [],
  pendingDamages: [],
  resourceTargets: [],
};

describe('CombatView sheets, the edges', () => {
  it('opens no attack for an opportunity prompt answered with nothing', () => {
    const { opened } = mountCombatView({ enc: withOffer() });
    opened[0].close(null);
    expect(opened).toHaveLength(1);
  });

  it('does not ask again once the combat is gone', () => {
    const { opened, state } = mountCombatView({ enc: withOffer() });
    opened[0].close({ attackKey: 'scimitar', options: reactorOptions });
    state.apply(null);
    opened[1].close(true);
    expect(opened).toHaveLength(2);
  });

  it('asks nothing about a familiar when the screen has no character of its own', () => {
    const { view, opened } = mountCombatView({
      enc: encounter({ combatants: [goblin()] }),
      master: true,
    });
    view.sheets.actions.familiarEyes('Nanquim');
    expect(opened).toEqual([]);
  });
});
