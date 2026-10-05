import { create } from '@bufbuild/protobuf';
import { describe, expect, it } from 'vitest';

import { CombatantKind, OpportunityAttackSchema, OpportunityOfferSchema } from '../../../gen/meurpg/play/v1/combat_pb';
import { AttackKind, AttackSchema } from '../../../gen/meurpg/rules/v1/rules_pb';
import { combatant, encounter } from './combat-testing';
import {
  attackLabel,
  barText,
  masterAsk,
  masterNews,
  offersOn,
  offersToAnswer,
  playerQuestion,
  spendText,
  reactorAttacks,
  reactorIsMasters,
  waitingText,
} from './opportunity';

const plain = (t: string) => t.replace(/ /g, ' ');

// E9-13: Toren leaves the reach of Goblin 2.
const toGoblin = create(OpportunityOfferSchema, {
  id: 'o1',
  moverId: 'toren',
  moverLabel: 'Toren',
  reactorId: 'g2',
  reactorLabel: 'Goblin 2',
  forYou: true,
  leftCol: 17,
  leftRow: 6,
  attacks: [create(OpportunityAttackSchema, { key: 'attack:scimitar', namePt: 'Cimitarra' })],
});
const toToren = create(OpportunityOfferSchema, {
  id: 'o2',
  moverId: 'g2',
  moverLabel: 'Goblin 2',
  reactorId: 'toren',
  reactorLabel: 'Toren',
  forYou: true,
  attacks: [create(OpportunityAttackSchema, { key: 'attack:longsword', namePt: 'Espada longa' })],
});
const unseen = create(OpportunityOfferSchema, { id: 'o3', moverId: 'toren', moverLabel: 'Toren', reactorId: '', reactorLabel: '', forYou: false });

const toren = combatant({ id: 'toren', label: 'Toren', kind: CombatantKind.PLAYER, mine: true, characterId: 'toren-c' });
const g2 = combatant({ id: 'g2', label: 'Goblin 2' });

describe('opportunity attacks in words', () => {
  it('lists what the caller answers, and what holds a mover', () => {
    const e = encounter({ combatants: [toren, g2], opportunityOffers: [toGoblin, unseen] });
    expect(offersToAnswer(e).map((o) => o.id)).toEqual(['o1']);
    expect(offersOn(e, 'toren').map((o) => o.id)).toEqual(['o1', 'o3']);
    expect(reactorIsMasters(e, toGoblin)).toBe(true);
    expect(reactorIsMasters(encounter({ combatants: [toren, g2] }), toToren)).toBe(false);
  });

  it('names the creature that would react when it is not the character: "do alcance do Lobo atroz 1", "Gasta a reação do Lobo atroz 1"', () => {
    const toWolf = { ...toToren, reactorLabel: 'Lobo atroz 1' } as typeof toToren;
    expect(playerQuestion(toWolf, false)).toBe('O Goblin 2 está saindo do alcance do Lobo atroz 1. Ataque de oportunidade?');
    expect(spendText(toWolf, false)).toBe('Gasta a reação do Lobo atroz 1.');
    expect(spendText(toToren)).toBe('Gasta a sua reação.');
  });

  it('asks the player, and tells the master', () => {
    expect(playerQuestion(toToren)).toBe('O Goblin 2 está saindo do seu alcance. Ataque de oportunidade?');
    expect(plain(masterNews(toGoblin))).toBe('O Toren saiu do alcance do Goblin 2.');
    expect(plain(masterAsk(toGoblin))).toBe('Goblin 2 ataca o Toren?');
  });

  it('says who the mover waits for, and names no reactor it does not see', () => {
    const e = encounter({ combatants: [toren, g2] });
    const mine = waitingText(e, [create(OpportunityOfferSchema, { ...toGoblin, forYou: false })]);
    expect(mine?.title).toBe('Esperando a reação do mestre');
    expect(plain(mine!.detail)).toContain('O Goblin 2 pode fazer um ataque de oportunidade.');
    const hidden = waitingText(e, [unseen]);
    expect(hidden).toEqual({
      title: 'Esperando o mestre',
      detail: 'Seu movimento já valeu; a sua vez continua quando ele responder.',
    });
    expect(hidden?.detail).not.toContain('Goblin');
    expect(waitingText(e, [])).toBeNull();
    const aPlayer = waitingText(e, [create(OpportunityOfferSchema, { ...toToren, moverId: 'x', forYou: false })]);
    expect(aPlayer?.title).toBe('Esperando a reação do jogador de Toren');
  });

  it('says the master\'s wait on the bar', () => {
    const e = encounter({ combatants: [toren, g2], opportunityOffers: [toGoblin] });
    expect(barText(e, () => '')).toBe('Esperando a sua reação: Goblin 2');
    const player = encounter({ combatants: [toren, g2], opportunityOffers: [toToren] });
    expect(barText(player, () => 'Caio')).toBe('Esperando a reação do Caio (Toren)');
    expect(barText(encounter({ combatants: [toren] }), () => '')).toBe('');
  });

  it('puts numbers on the offer\'s attacks from the reactor\'s options', () => {
    const scimitar = create(AttackSchema, {
      key: 'attack:scimitar',
      name: 'Scimitar',
      namePt: 'Cimitarra',
      kind: AttackKind.WEAPON,
      attackBonus: 4,
      damage: '1d6+2',
      damageTypePt: 'cortante',
      rangeFt: 5,
    });
    const [a] = reactorAttacks(toGoblin, [scimitar]);
    expect(plain(a.detail)).toBe('Cimitarra +4 · 1d6 + 2 cortante');
    expect(attackLabel(a)).toBe('Atacar com Cimitarra');
    // An attack the options do not list is still offered by its name.
    const [b] = reactorAttacks(toToren, []);
    expect(b.attack).toBeNull();
    expect(attackLabel(b)).toBe('Atacar com Espada longa');
  });
});
