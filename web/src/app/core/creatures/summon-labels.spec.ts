import { create } from '@bufbuild/protobuf';

import { ReplacedCreatureSchema } from '../../../gen/meurpg/characters/v1/characters_pb';
import { castNotice, castVerb, replacesText } from './summon-labels';

/** The words, with the no-break spaces `tight()` puts in written as plain ones (`text.spec.ts` covers those). */
const plain = (t: string) => t.replace(/\u00a0/g, ' ');
const wolf = (n: number) => create(ReplacedCreatureSchema, { id: `w${n}`, name: `Lobo ${n}`, monsterNamePt: 'Lobo' });

describe('what the casting sheet says in words', () => {
  it('one verb for each spell, the same in every state', () => {
    expect(castVerb('spell:find-familiar', 'Encontrar Familiar')).toBe('Convocar o familiar');
    expect(castVerb('spell:animate-dead', 'Animar os Mortos')).toBe('Animar os mortos');
    expect(castVerb('spell:conjure-animals', 'Conjurar Animais')).toBe('Conjurar os animais');
    expect(castVerb('spell:other', 'Outra')).toBe('Conjurar Outra');
  });

  it('a new familiar takes the old one\'s place; a new concentration ends the old one\'s creatures', () => {
    expect(plain(replacesText('Encontrar Familiar', false, [create(ReplacedCreatureSchema, { name: 'Nanquim' })]))).toBe('Nanquim sai da ficha: um novo familiar toma o lugar.');
    expect(plain(replacesText('Conjurar Animais', true, [wolf(1), wolf(2)]))).toBe('Isso encerra Conjurar Animais e dispensa 2 criaturas: Lobo 1 e Lobo 2.');
    expect(plain(replacesText('Conjurar Animais', true, [1, 2, 3, 4, 5, 6, 7, 8].map(wolf)))).toBe('Isso encerra Conjurar Animais e dispensa 8 criaturas: Lobo 1, Lobo 2, Lobo 3 e mais 5.');
    expect(plain(replacesText('Conjurar Animais', true, []))).toBe('');
  });

  it('the notice after a cast says what arrived, what it cost and what left', () => {
    expect(plain(castNotice('Nanquim chegou.', 'Encontrar Familiar', true, '1 hora', 0))).toBe('Nanquim chegou. Encontrar Familiar, ritual de 1 hora. Nenhum espaço de magia foi gasto.');
    expect(plain(castNotice('8 criaturas chegaram.', 'Conjurar Animais', false, '1 ação', 2))).toContain('2 criaturas foram dispensadas no lugar.');
    expect(plain(castNotice('A criatura chegou.', 'Conjurar Animais', false, '1 ação', 1))).toContain('Uma criatura foi dispensada no lugar.');
  });
});
