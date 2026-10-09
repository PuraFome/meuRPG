import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';

import {
  CombatantKind,
  HiddenRevealQuestionSchema,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { HiddenRevealCard, type RevealAnswer } from './hidden-reveal-card';

const plain = (t: string | null | undefined) =>
  (t ?? '').replace(/ /g, ' ').replace(/\s+/g, ' ').trim();

const people = [
  combatant({ id: 'p', label: 'Pensantus', kind: CombatantKind.PLAYER }),
  combatant({ id: 's', label: 'Sálvia', kind: CombatantKind.PLAYER }),
  combatant({ id: 'g3', label: 'Goblin 3', hidden: true }),
  combatant({ id: 'g4', label: 'Goblin 4', hidden: true }),
  combatant({ id: 'g5', label: 'Goblin 5', hidden: true }),
];
const fireball = create(HiddenRevealQuestionSchema, {
  id: 'q1',
  casterId: 'p',
  spellKey: 'spell:fireball',
  combatantIds: ['g3', 'g4'],
});
const thunder = create(HiddenRevealQuestionSchema, {
  id: 'q2',
  casterId: 's',
  spellKey: 'spell:thunderwave',
  combatantIds: ['g5'],
});
const names = new Map([
  ['spell:fireball', 'Bola de Fogo'],
  ['spell:thunderwave', 'Onda Trovejante'],
]);

function setup(questions: (typeof fireball)[]) {
  const fixture = TestBed.createComponent(HiddenRevealCard);
  fixture.componentRef.setInput(
    'encounter',
    encounter({ combatants: people, pendingHiddenReveals: questions }),
  );
  fixture.componentRef.setInput('spellNames', names);
  const answers: RevealAnswer[] = [];
  let answered = 0;
  fixture.componentInstance.answer.subscribe((a) => answers.push(a));
  fixture.componentInstance.answered.subscribe(() => answered++);
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  const buttons = (card: Element) => Array.from(card.querySelectorAll<HTMLButtonElement>('button'));
  return { fixture, el, answers, buttons, answered: () => answered };
}

describe('HiddenRevealCard', () => {
  it('asks the master about the hidden creatures a player\'s area hit, by name, and answers with "Revelar"', () => {
    const { el, answers, buttons } = setup([fireball]);
    const card = el.querySelector('section[role="group"]')!;
    expect(card.getAttribute('aria-labelledby')).toBe('hr-t-q1');
    expect(plain(card.querySelector('h2')?.textContent)).toBe(
      'Bola de Fogo atingiu 2 criaturas escondidas',
    );
    const text = plain(card.textContent);
    expect(text).toContain(
      'Pensantus conjurou no ponto marcado. O efeito já valeu para Goblin 3 e Goblin 4.',
    );
    expect(text).toContain('Os jogadores ainda não sabem delas.');
    expect(text).toContain('O turno do Pensantus espera por você.');
    expect(text).toContain('A regra da mesa é “Perguntar a cada vez”.');
    // Arriving, it is announced once, and takes no focus.
    expect(plain(el.querySelector('[role="alert"]')?.textContent)).toBe(
      'Bola de Fogo atingiu 2 criaturas escondidas',
    );
    expect(card.contains(document.activeElement)).toBe(false);
    const [reveal, keep] = buttons(card);
    expect(plain(reveal.textContent)).toContain('Revelar');
    expect(plain(keep.textContent)).toContain('Manter escondidas');
    reveal.click();
    keep.click();
    expect(answers).toEqual([
      { id: 'q1', reveal: true },
      { id: 'q1', reveal: false },
    ]);
  });

  it('takes two questions in order: only the first one answers, the second says why it waits', () => {
    const { el, answers, buttons } = setup([fireball, thunder]);
    const [first, second] = Array.from(el.querySelectorAll('section[role="group"]'));
    expect(plain(first.textContent)).toContain('Pergunta 1 de 2 · responda esta primeiro');
    expect(plain(second.textContent)).toContain('Pergunta 2 de 2 · depois da primeira');
    expect(plain(second.querySelector('h2')?.textContent)).toBe(
      'Onda Trovejante atingiu 1 criatura escondida',
    );
    const [reveal2, keep2] = buttons(second);
    expect(reveal2.getAttribute('aria-disabled')).toBe('true');
    expect(reveal2.getAttribute('aria-describedby')).toBe('hr-k-q2');
    reveal2.click();
    keep2.click();
    expect(answers).toEqual([]);
    buttons(first)[1].click();
    expect(answers).toEqual([{ id: 'q1', reveal: false }]);
  });

  it('moves the focus to the next question\'s "Revelar" once the first is answered', async () => {
    const { fixture, el, buttons } = setup([fireball, thunder]);
    buttons(el.querySelector('section')!)[0].focus();
    fixture.componentRef.setInput(
      'encounter',
      encounter({ combatants: people, pendingHiddenReveals: [thunder] }),
    );
    fixture.detectChanges();
    await fixture.whenStable();
    const card = el.querySelector('section')!;
    expect(plain(card.querySelector('h2')?.textContent)).toBe(
      'Onda Trovejante atingiu 1 criatura escondida',
    );
    expect(document.activeElement).toBe(buttons(card)[0]);
    expect(buttons(card)[0].getAttribute('aria-disabled')).not.toBe('true');
  });

  it('hands the focus on when the last question is answered', async () => {
    const { fixture, el, buttons, answered } = setup([fireball]);
    buttons(el.querySelector('section')!)[0].focus();
    fixture.componentRef.setInput('encounter', encounter({ combatants: people }));
    fixture.detectChanges();
    await fixture.whenStable();
    expect(el.querySelector('section')).toBeNull();
    expect(answered()).toBe(1);
  });

  it('comes back from the combat as it is after a reload, and is empty with no question', () => {
    expect(setup([fireball]).el.querySelectorAll('section')).toHaveLength(1);
    expect(setup([]).el.querySelectorAll('section')).toHaveLength(0);
  });
});
