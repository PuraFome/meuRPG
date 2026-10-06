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
    expect(text).toContain('Se você seguir sem esperar, o Toren não ataca e continua com a reação.');
    expect(el.querySelector('.op__btn[data-safe]')).toBeNull();
    Array.from(el.querySelectorAll('button')).find((b) => plain(b.textContent) === 'Seguir sem esperar')!.click();
    expect(skipped).toEqual(['o2']);
  });

  describe('an offer the master made by hand (a combat without a map, RN-25)', () => {
    const byHandToToren = create(OpportunityOfferSchema, { ...toToren, id: 'o3', byHand: true });
    const byHandToGoblin = create(OpportunityOfferSchema, { ...toGoblin, id: 'o4', byHand: true });

    it('waits on the player with a status, "Seguir sem esperar" and "Retirar a oferta", and keeps the reaction', () => {
      const { fixture, el, skipped } = setup([byHandToToren]);
      const withdrawn: string[] = [];
      fixture.componentInstance.withdraw.subscribe((o) => withdrawn.push(o.id));
      const text = plain(el.textContent);
      expect(plain(el.querySelector('[role="status"]')?.textContent)).toContain('Esperando a resposta do Caio (Toren).');
      expect(text).toContain('O Goblin 2 saiu do alcance dele. O turno continua depois da resposta.');
      expect(text).toContain('o Toren não ataca e continua com a reação');
      expect(text).not.toContain('perde essa reação');
      const buttons = Array.from(el.querySelectorAll('button')).filter((b) => ['Seguir sem esperar', 'Retirar a oferta'].includes(plain(b.textContent)));
      // Both ways out are outlined buttons of one size; the footnote says which to use when.
      expect(buttons.length).toBe(2);
      expect(buttons.every((b) => b.classList.contains('mat-mdc-outlined-button') && b.classList.contains('op__btn'))).toBe(true);
      expect(text).toContain('“Seguir sem esperar” é para quando o jogador não responde');
      expect(text).toContain('“Retirar a oferta” é para quando você ofereceu sem querer');
      const press = (name: string) => Array.from(el.querySelectorAll('button')).find((b) => plain(b.textContent) === name)!.click();
      press('Seguir sem esperar');
      press('Retirar a oferta');
      expect(skipped).toEqual(['o3']);
      expect(withdrawn).toEqual(['o3']);
    });

    it('never talks about a square: nothing goes back anywhere, and the master can take the offer back', () => {
      const { fixture, el } = setup([byHandToGoblin]);
      const withdrawn: string[] = [];
      fixture.componentInstance.withdraw.subscribe((o) => withdrawn.push(o.id));
      const text = plain(el.textContent);
      expect(text).toContain('Goblin 2 ataca o Toren?');
      expect(text).not.toContain('quadrado');
      expect(text).not.toContain('o token volta');
      Array.from(el.querySelectorAll('button')).find((b) => plain(b.textContent) === 'Retirar a oferta')!.click();
      expect(withdrawn).toEqual(['o4']);
    });

    it('keeps the square line and no "Retirar a oferta" for an offer a move made', () => {
      const { el } = setup([toGoblin]);
      expect(plain(el.textContent)).toContain('o token volta ao último quadrado');
      expect(plain(el.textContent)).not.toContain('Retirar a oferta');
    });
  });

  it('draws nothing when no offer waits', () => {
    const { el } = setup([]);
    expect(el.querySelector('section')).toBeNull();
  });
});
