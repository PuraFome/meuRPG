import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';

import { CombatantKind, OpportunityAttackSchema, OpportunityOfferSchema } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import type { ReactorAttack } from '../../../../core/combat/opportunity';
import { type MasterAnswer, OpportunityCard } from './opportunity-card';

const plain = (t: string | null | undefined) => (t ?? '').replace(/ /g, ' ').replace(/\s+/g, ' ').trim();

const toren = combatant({ id: 'toren', label: 'Toren', kind: CombatantKind.PLAYER, characterId: 'toren-c', movementUsedDft: 150 });
const g2 = combatant({ id: 'g2', label: 'Goblin 2' });

// The Toren leaves Goblin 2's reach (E9-13, frame 1).
const toGoblin = create(OpportunityOfferSchema, {
  id: 'o1',
  moverId: 'toren',
  moverLabel: 'Toren',
  reactorId: 'g2',
  reactorLabel: 'Goblin 2',
  forYou: true,
  attacks: [create(OpportunityAttackSchema, { key: 'attack:scimitar', namePt: 'Cimitarra' })],
});
// Goblin 2 leaves Toren's reach: a player answers.
const toToren = create(OpportunityOfferSchema, {
  id: 'o2',
  moverId: 'g2',
  moverLabel: 'Goblin 2',
  reactorId: 'toren',
  reactorLabel: 'Toren',
  forYou: true,
});
const scimitar: ReactorAttack = { key: 'attack:scimitar', name: 'Cimitarra', detail: 'Cimitarra +4 · 1d6 + 2 cortante', attack: null };

function setup(offers: (typeof toGoblin)[]) {
  const fixture = TestBed.createComponent(OpportunityCard);
  fixture.componentRef.setInput('encounter', encounter({ combatants: [toren, g2], opportunityOffers: offers }));
  fixture.componentRef.setInput('attacksByOffer', new Map([['o1', [scimitar]]]));
  fixture.componentRef.setInput('info', new Map([['toren-c', { classSummary: '', playerName: 'Caio', kindLabel: '', raceName: '' }]]));
  const answers: MasterAnswer[] = [];
  const skipped: string[] = [];
  fixture.componentInstance.answer.subscribe((a) => answers.push(a));
  fixture.componentInstance.skip.subscribe((o) => skipped.push(o.id));
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement, answers, skipped };
}

describe('OpportunityCard', () => {
  it('asks the master for an NPC\'s attack, with the numbers and the cost', () => {
    const { el } = setup([toGoblin]);
    const text = plain(el.textContent);
    expect(text).toContain('O Toren saiu do alcance do Goblin 2.');
    expect(text).toContain('Goblin 2 ataca o Toren?');
    expect(text).toContain('Cimitarra +4 · 1d6 + 2 cortante · gasta a reação dele');
    expect(text).toContain('O turno do Toren espera a sua resposta. Já andou 4,5 m: o movimento valeu.');
    expect(plain(el.querySelector('section')?.getAttribute('aria-label'))).toBe('Ataque de oportunidade de Goblin 2');
  });

  it('has "Não atacar" and "Atacar com Cimitarra", the same width, with the safe one focused', async () => {
    const { fixture, el, answers } = setup([toGoblin]);
    await fixture.whenStable();
    const [no, yes] = Array.from(el.querySelectorAll<HTMLButtonElement>('.op__btn'));
    expect(plain(no.textContent)).toBe('Não atacar');
    expect(plain(yes.textContent)).toBe('Atacar com Cimitarra');
    expect(document.activeElement).toBe(no);
    no.click();
    yes.click();
    expect(answers.map((a) => a.attack?.key ?? null)).toEqual([null, 'attack:scimitar']);
  });

  it('waits on a player\'s reactor with "Seguir sem esperar" and names the player', () => {
    const { el, skipped } = setup([toToren]);
    const text = plain(el.textContent);
    expect(text).toContain('Esperando a reação do Caio (Toren)');
    expect(text).toContain('Se você seguir sem esperar, o Toren perde essa reação.');
    expect(el.querySelector('.op__btn[data-safe]')).toBeNull();
    Array.from(el.querySelectorAll('button')).find((b) => plain(b.textContent) === 'Seguir sem esperar')!.click();
    expect(skipped).toEqual(['o2']);
  });

  it('draws nothing when no offer waits', () => {
    const { el } = setup([]);
    expect(el.querySelector('section')).toBeNull();
  });
});
