import { create } from '@bufbuild/protobuf';

import { HiddenRevealQuestionSchema } from '../../../gen/meurpg/play/v1/combat_pb';
import { combatant, encounter } from './combat-testing';
import {
  heldWait,
  hitNames,
  lastArea,
  namesText,
  questionKicker,
  questionTitle,
  revealBarText,
  revealWhy,
} from './hidden-reveal';

const plain = (t: string) => t.replace(/ /g, ' ');

const question = (id: string, ids: string[]) =>
  create(HiddenRevealQuestionSchema, {
    id,
    casterId: 'p',
    spellKey: 'spell:fireball',
    combatantIds: ids,
  });

describe('the turn held by a question about hidden creatures', () => {
  it('tells a player only that the turn waits for the master, on their turn and on another', () => {
    const held = encounter({ turnHeld: true });
    expect(heldWait(held, true)).toEqual({
      title: 'Esperando o mestre',
      detail: 'A sua vez continua quando ele responder.',
    });
    expect(heldWait(held, false)?.detail).toBe('O turno continua quando ele responder.');
    expect(heldWait(encounter({ turnHeld: false }), true)).toBeNull();
  });

  it("says nothing of a spell, a creature or a count in a player's wait", () => {
    const text = JSON.stringify(heldWait(encounter({ turnHeld: true }), true));
    expect(text).not.toMatch(/escond|criatura|magia|pergunta|\d/i);
  });

  it("writes the master's bar and the reason the turn does not pass, for one question and for two", () => {
    const one = encounter({ pendingHiddenReveals: [question('q1', ['g3', 'g4'])] });
    const two = encounter({
      pendingHiddenReveals: [question('q1', ['g3', 'g4']), question('q2', ['g5'])],
    });
    expect(revealBarText(one)).toBe('Esperando a sua resposta: escondidas atingidas');
    expect(revealWhy(one)).toBe('Responda ao pedido abaixo para seguir.');
    expect(revealBarText(two)).toBe('Esperando a sua resposta: 2 perguntas de escondidas');
    expect(revealWhy(two)).toBe('Responda aos pedidos abaixo para seguir.');
    expect(revealBarText(encounter())).toBe('');
    expect(revealWhy(encounter())).toBe('');
  });

  it('names the question and the creatures it hit, from the combat the master reads', () => {
    const e = encounter({
      combatants: [
        combatant({ id: 'g3', label: 'Goblin 3' }),
        combatant({ id: 'g4', label: 'Goblin 4' }),
      ],
    });
    expect(plain(questionTitle('Bola de Fogo', 2))).toBe(
      'Bola de Fogo atingiu 2 criaturas escondidas',
    );
    expect(plain(questionTitle('Onda Trovejante', 1))).toBe(
      'Onda Trovejante atingiu 1 criatura escondida',
    );
    expect(plain(namesText(hitNames(e, question('q', ['g3', 'g4']))))).toBe('Goblin 3 e Goblin 4');
  });

  it('numbers the questions only when there is more than one', () => {
    expect(questionKicker(0, 1)).toBe('');
    expect(questionKicker(0, 2)).toBe('Pergunta 1 de 2 · responda esta primeiro');
    expect(questionKicker(1, 2)).toBe('Pergunta 2 de 2 · depois da primeira');
  });
});

describe('a question with no hidden creature, and the area of the last spell', () => {
  it('names the bar and the title without claiming a hidden creature was hit', () => {
    const none = encounter({ pendingHiddenReveals: [question('q1', [])] });
    expect(revealBarText(none)).toBe('Esperando a sua resposta: magia de área');
    expect(plain(questionTitle('Bola de Fogo', 0))).toBe(
      'Bola de Fogo: nenhuma criatura escondida na área',
    );
  });

  it("draws the oldest question's area for the master, and nothing for a player or without squares", () => {
    const withArea = create(HiddenRevealQuestionSchema, {
      id: 'q1',
      casterId: 'p',
      spellKey: 'spell:fireball',
      combatantIds: ['g3'],
      area: {
        origin: { col: 4, row: 3 },
        squares: [
          { col: 4, row: 3 },
          { col: 5, row: 3 },
        ],
      },
    });
    expect(lastArea(encounter({ pendingHiddenReveals: [withArea] }))).toEqual({
      origin: { col: 4, row: 3 },
      squares: [
        { col: 4, row: 3 },
        { col: 5, row: 3 },
      ],
    });
    expect(lastArea(encounter({ pendingHiddenReveals: [] }))).toBeNull();
    expect(lastArea(encounter({ pendingHiddenReveals: [question('q2', [])] }))).toBeNull();
  });
});
