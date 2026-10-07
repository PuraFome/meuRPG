import { CombatantKind } from '../../../gen/meurpg/play/v1/combat_pb';
import { combatant, encounter } from './combat-testing';
import {
  creatureTurnTitle,
  endLabel,
  groupName,
  kindWord,
  ofOwner,
  pluralName,
} from './creature-names';

const salvia = combatant({
  id: 's',
  label: 'Sálvia',
  kind: CombatantKind.PLAYER,
  mine: true,
  characterId: 'char-s',
});
const wolf = (n: number) =>
  combatant({
    id: `w${n}`,
    label: `Lobo atroz ${n}`,
    kind: CombatantKind.CREATURE,
    controlledByMe: true,
    ownerCharacterId: 'char-s',
    monsterKey: 'monster:dire-wolf',
    monsterNamePt: 'Lobo atroz',
  });
const raven = combatant({
  id: 'n',
  label: 'Nanquim',
  kind: CombatantKind.CREATURE,
  ownerCharacterId: 'char-p',
  monsterKey: 'monster:raven',
  monsterNamePt: 'Corvo',
});

describe("the names of a player's creatures", () => {
  it('pluralizes word by word', () => {
    expect(pluralName('Lobo atroz')).toBe('Lobos atrozes');
    expect(pluralName('Urso-marrom')).toBe('Ursos-marrons');
    expect(pluralName('Cobra de pedra')).toBe('Cobras de pedra');
    expect(pluralName('Esqueleto')).toBe('Esqueletos');
  });

  it("names a group, and keeps one creature's own name", () => {
    expect(groupName([wolf(1), wolf(2)])).toBe('Lobos atrozes');
    expect(groupName([wolf(1), raven])).toBe('Criaturas');
    expect(groupName([raven])).toBe('Nanquim');
  });

  it('says whose it is and what it is', () => {
    const e = encounter({ combatants: [salvia, wolf(1)] });
    expect(ofOwner(e, wolf(1))).toBe('da Sálvia');
    expect(kindWord(wolf(1))).toBe('Criatura');
    expect(kindWord(raven)).toBe('Corvo');
  });

  it('titles the turn for its player and for the others', () => {
    const e = encounter({ combatants: [salvia, wolf(1), wolf(2)] });
    expect(creatureTurnTitle(e, [wolf(1), wolf(2)], true)).toBe('Vez dos seus Lobos atrozes');
    expect(creatureTurnTitle(e, [wolf(1), wolf(2)], false)).toBe('Vez dos Lobos atrozes da Sálvia');
    expect(creatureTurnTitle(e, [raven], true)).toBe('Vez do seu Nanquim');
    expect(endLabel([wolf(1), wolf(2)])).toBe('Encerrar a parte dos Lobos');
    expect(endLabel([raven])).toBe('Encerrar a vez do Nanquim');
  });
});
