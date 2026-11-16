import { NpcCheckAskState } from '../../../gen/meurpg/play/v1/combat_pb';
import { cardOf, nobodyReasonOf, nobodyText } from './npc-check-card';
import { ask, npcCheckRead } from './npc-check-testing';

const { WAITING, CUT, PASSED, CLOSED } = NpcCheckAskState;

describe('cardOf', () => {
  it('names the card by its roll and, while it waits, shows the d20 and the bards in order', () => {
    const card = cardOf(npcCheckRead());
    expect(card.title).toBe('Capitão bandido · Enganação');
    expect(card.settled).toBe(false);
    expect(card.box).toBe(17);
    expect(card.formula).toBe('d20 (13) + 4 = 17');
    expect(card.detail).toBe(
      'Antes das Palavras de Interrupção: o total fecha quando os bardos responderem.',
    );
    expect(card.rows.map((r) => [r.name, r.tag, r.sub, r.tone])).toEqual([
      ['Brisa', 'Esperando Brisa', 'd8 · a pergunta está aberta', 'waiting'],
      ['bilbo', 'Na fila', 'd6 · depois de Brisa', 'queue'],
    ]);
  });

  it('takes the dice already cut off the total while it waits', () => {
    const card = cardOf(
      npcCheckRead({
        asks: [ask('Brisa', CUT, { face: 4 }), ask('bilbo', WAITING, { sides: 6 })],
      }),
    );
    expect(card.box).toBe(13);
    expect(card.formula).toBe('17 − 4 = 13, por enquanto');
    expect(card.detail).toBe('d20 (13) + 4, menos o corte de Brisa');
    expect(card.rows[0]).toMatchObject({ tag: 'Cortou 4', sub: 'd8 saiu 4', tone: 'cut' });
    expect(card.rows[1]).toMatchObject({ tag: 'Esperando bilbo', tone: 'waiting' });
  });

  it('settles with the server total and says each bard did', () => {
    const card = cardOf(
      npcCheckRead({
        settled: true,
        total: 13,
        cut: 4,
        asks: [ask('Brisa', CUT, { face: 4 }), ask('bilbo', CLOSED, { sides: 6 })],
      }),
    );
    expect(card.settled).toBe(true);
    expect(card.box).toBe(13);
    expect(card.formula).toBe('17 − 4 = 13');
    expect(card.detail).toBe('d20 (13) + 4, menos o corte de Brisa');
    expect(card.rows.map((r) => r.tag)).toEqual(['Cortou 4', 'Sem resposta']);
    expect(card.rows[1].sub).toBe('d6 · você seguiu sem esperar · nada gasto');
    expect(card.nobody).toBe(false);
  });

  it('names every bard that cut', () => {
    const card = cardOf(
      npcCheckRead({
        settled: true,
        total: 10,
        cut: 7,
        asks: [ask('Brisa', CUT, { face: 4 }), ask('bilbo', CUT, { face: 3, sides: 6 })],
      }),
    );
    expect(card.detail).toBe('d20 (13) + 4, menos os cortes de Brisa e bilbo');
  });

  it('says a bard let it pass without any spent', () => {
    const card = cardOf(
      npcCheckRead({ settled: true, total: 17, asks: [ask('Brisa', PASSED)] }),
    );
    expect(card.formula).toBe('d20 (13) + 4 = 17');
    expect(card.rows[0]).toMatchObject({ tag: 'Deixou passar', sub: 'd8 · nada gasto' });
    expect(card.detail).toBe('Ninguém cortou o teste.');
  });

  it('settles at once with nobody to ask and says nobody could cut, never why', () => {
    const card = cardOf(
      npcCheckRead({
        holdId: '',
        settled: true,
        total: 16,
        d20: { diceCount: 1, diceSides: 20, faces: [14], modifier: 2, total: 16 },
        checkName: 'Intimidação',
        asks: [],
      }),
    );
    expect(card.box).toBe(16);
    expect(card.formula).toBe('d20 (14) + 2 = 16');
    expect(card.nobody).toBe(true);
    expect(card.nobodyText).toBe('Ninguém foi perguntado: nenhum bardo podia cortar.');
    expect(card.rows).toEqual([]);
  });

  it('says out loud that an unknown bonus rolled +0, so the master adds it by hand', () => {
    const waiting = cardOf(
      npcCheckRead({
        creatureLabel: 'Bartolo',
        checkName: 'Persuasão',
        modifierKnown: false,
        d20: { diceCount: 1, diceSides: 20, faces: [9], modifier: 0, total: 9 },
      }),
    );
    expect(waiting.noBonus).toBe(
      'Sem bônus: Bartolo não tem número para Persuasão. O d20 contou com +0: some o bônus que achar justo.',
    );
    expect(waiting.formula).toBe('d20 (9) + 0 = 9');
    const settled = cardOf(
      npcCheckRead({ modifierKnown: false, settled: true, total: 13, asks: [] }),
    );
    expect(settled.noBonus).toContain('Sem bônus: Capitão bandido');
    expect(cardOf(npcCheckRead()).noBonus).toBe('');
  });

  it('writes a negative bonus with a minus sign', () => {
    const card = cardOf(
      npcCheckRead({ d20: { diceCount: 1, diceSides: 20, faces: [10], modifier: -1, total: 9 } }),
    );
    expect(card.formula).toBe('d20 (10) − 1 = 9');
  });

  it('gives a card with no hold a key that changes with the roll, so a new roll is not hidden by an old X', () => {
    const a = cardOf(npcCheckRead({ holdId: '', settled: true, asks: [] }));
    const b = cardOf(
      npcCheckRead({
        holdId: '',
        settled: true,
        asks: [],
        d20: { diceCount: 1, diceSides: 20, faces: [2], modifier: 4, total: 6 },
      }),
    );
    expect(a.key).not.toBe(b.key);
    expect(cardOf(npcCheckRead()).key).toBe('hold-1');
  });

  it('words why nobody was asked from the reason the server will send, in neutral sentences', () => {
    expect(nobodyText('no-bard')).toBe(
      'Ninguém foi perguntado: nenhum bardo tem Palavras de Interrupção com um uso de Inspiração de Bardo.',
    );
    expect(nobodyText('hidden')).toBe(
      'Ninguém foi perguntado: o token da criatura está escondido no mapa.',
    );
    expect(nobodyText('immune')).toBe('Ninguém foi perguntado: a criatura não pode ser enfeitiçada.');
    expect(nobodyText('unseen')).toBe('Ninguém foi perguntado: nenhum bardo vê a criatura.');
    expect(nobodyText('unknown')).toBe('Ninguém foi perguntado: nenhum bardo podia cortar.');
    const read = npcCheckRead({ holdId: '', settled: true, total: 16, asks: [] });
    expect(cardOf(read, 'hidden').nobodyText).toBe(nobodyText('hidden'));
    expect(cardOf(read).nobodyText).toBe(nobodyText('unknown'));
  });

  it('reads the reason from one mapping, which says "unknown" until the server sends the enum', () => {
    expect(nobodyReasonOf(npcCheckRead({ settled: true, asks: [] }))).toBe('unknown');
    expect(cardOf(npcCheckRead({ settled: true, asks: [] }), 'hidden').nobodyText).toBe(
      'Ninguém foi perguntado: o token da criatura está escondido no mapa.',
    );
  });
});
