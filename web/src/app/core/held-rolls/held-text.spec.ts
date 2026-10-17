import { create } from '@bufbuild/protobuf';

import { HeldRollKind, HeldRollOfferSchema } from '../../../gen/meurpg/play/v1/combat_pb';
import {
  askingText,
  d20Chip,
  d20Line,
  pendingCountText,
  questionRule,
  questionTitle,
  questionWhat,
} from './held-text';
import { heldDie, heldLucky } from './held-roll-testing';

describe('the words of the master’s pending questions', () => {
  it('titles the question as the roller reads it', () => {
    expect(questionTitle(heldLucky())).toBe('Tirou 1: rolar de novo?');
    expect(questionTitle(heldDie(8, 2, 6))).toBe('Usar a Inspiração de Bardo (d6)?');
  });

  it('says what rolled: the attack and its target when known, else the kind of the roll', () => {
    const attack = heldLucky([1], 5, { kind: HeldRollKind.WEAPON_ATTACK });
    expect(questionWhat(attack, 'Goblin chefe')).toBe('Ataque com arma contra Goblin chefe.');
    expect(questionWhat(attack, '')).toBe('Ataque com arma.');
    const kinds: [HeldRollKind, string][] = [
      [HeldRollKind.SPELL_ATTACK, 'Ataque com magia.'],
      [HeldRollKind.EFFECT_SAVE, 'Teste de resistência contra um efeito.'],
      [HeldRollKind.CONCENTRATION_SAVE, 'Teste de resistência para manter a concentração.'],
      [HeldRollKind.SCENE_CHECK, 'Teste da cena.'],
      [HeldRollKind.GROUP_CHECK, 'Teste em grupo.'],
      [HeldRollKind.TRAP_SEARCH, 'Busca por armadilhas.'],
      [HeldRollKind.HINT_TRY, 'Tentativa de dica do quebra-cabeça.'],
      [HeldRollKind.UNSPECIFIED, 'Rolagem de d20.'],
    ];
    for (const [kind, text] of kinds) {
      expect(questionWhat(heldLucky([1], 5, { kind }), '')).toBe(text);
    }
    // A target only makes sense for an attack.
    expect(questionWhat(heldLucky([1], 5, { kind: HeldRollKind.SCENE_CHECK }), 'Goblin')).toBe(
      'Teste da cena.',
    );
  });

  it('states the rule of the question after the roll', () => {
    expect(questionRule(heldLucky())).toEqual({
      lead: 'Sortudo:',
      text: 'Brisa pode rolar esse d20 de novo e ficar com o novo resultado.',
    });
    expect(questionRule(heldLucky([1, 13]))).toEqual({
      lead: 'Sortudo:',
      text: 'Brisa pode rolar de novo o dado que deu 1 e ficar com o novo resultado.',
    });
    expect(questionRule(heldDie())).toEqual({
      lead: '',
      text: 'O dado é de Orla e se perde depois de rolado.',
    });
    const ungiven = create(HeldRollOfferSchema, {
      question: { case: 'die', value: { sides: 8, fromName: '' } },
    });
    expect(questionRule(ungiven)).toEqual({ lead: '', text: 'O dado se perde depois de rolado.' });
  });

  it('draws the d20 as the chip and the line beside it', () => {
    expect(d20Chip(heldLucky())).toBe('1');
    expect(d20Chip(heldLucky([1, 13]))).toBe('1 e 13');
    expect(d20Chip(heldDie(8, 2))).toBe('8');
    // Lucky does not give the total away: the 1 may still be rolled again.
    expect(d20Line(heldLucky([1], 5))).toBe('d20 · bônus +5');
    expect(d20Line(heldDie(8, 2))).toBe('d20 · bônus +2 · total 10');
    expect(d20Line(heldDie(8, -1))).toBe('d20 · bônus −1 · total 7');
    expect(d20Line(heldDie(8, 0))).toBe('d20 · total 8');
  });

  it('counts the pending questions', () => {
    expect(pendingCountText(0)).toBe('');
    expect(pendingCountText(1)).toBe('1 pergunta pendente');
    expect(pendingCountText(2)).toBe('2 perguntas pendentes');
  });

  it('words what the master’s answer is doing', () => {
    expect(askingText({ kind: 'lucky', reroll: true, roll: { inApp: true } }, heldLucky())).toBe(
      'Rolando o d20 de novo…',
    );
    expect(askingText({ kind: 'lucky', reroll: true, roll: { face: 14 } }, heldLucky())).toBe(
      'Enviando o resultado…',
    );
    expect(askingText({ kind: 'lucky', reroll: false, roll: null }, heldLucky())).toBe(
      'Ficando com o resultado do d20…',
    );
    expect(askingText({ kind: 'die', use: true, roll: { inApp: true } }, heldDie())).toBe(
      'Rolando o d8…',
    );
    expect(askingText({ kind: 'die', use: false, roll: null }, heldDie())).toBe(
      'Guardando o dado…',
    );
  });
});
