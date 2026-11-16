import { NpcCheckSource } from '../../../gen/meurpg/play/v1/scene_pb';
import {
  boxHint,
  boxStart,
  checkOptions,
  defaultCheck,
  groupsOf,
  noBonusLine,
  ofName,
  parseD20,
  rollLabel,
  rowId,
  showsBox,
} from './npc-check-picker';
import { candidate } from './npc-check-testing';

const bartolo = candidate({
  characterId: 'npc-bartolo',
  monsterKey: '',
  name: 'Bartolo',
  hasStatBlock: false,
  checks: [{ key: 'skill:persuasion', name: 'Persuasão', bonus: 0, known: false }],
});
const espiao = candidate({
  characterId: 'npc-espiao',
  monsterKey: '',
  name: 'Espião',
  source: NpcCheckSource.TOKEN,
  hidden: true,
  mapMeasures: true,
});

describe('groupsOf', () => {
  it('lists the stage, the map and the bestiary in that order, and the bestiary even when empty', () => {
    const groups = groupsOf([espiao, candidate(), bartolo]);
    expect(groups.map((g) => [g.label, g.rows.map((r) => r.name)])).toEqual([
      ['Em cena', ['Capitão bandido', 'Bartolo']],
      ['No mapa', ['Espião']],
      ['Do bestiário', []],
    ]);
  });

  it('drops a group of the stage or the map that has nobody', () => {
    expect(groupsOf([]).map((g) => g.label)).toEqual(['Do bestiário']);
  });

  it('says what the players see of each creature and that one has no stat block', () => {
    const rows = groupsOf([espiao, candidate(), bartolo]).flatMap((g) => g.rows);
    expect(rows.map((r) => [r.name, r.sees, r.noStatBlock])).toEqual([
      ['Capitão bandido', 'Os jogadores veem', false],
      ['Bartolo', 'Os jogadores veem', true],
      ['Espião', 'Escondido dos jogadores', false],
    ]);
  });

  it('names a creature by its character, else by its creature key', () => {
    expect(rowId(candidate())).toBe('npc-capitao');
    expect(rowId(candidate({ characterId: '' }))).toBe('monster:bandit-captain');
  });
});

describe('the checkbox "Os bardos veem a criatura e ela os ouve"', () => {
  it('shows only when the map cannot measure the creature against a bard', () => {
    expect(showsBox(candidate())).toBe(true);
    expect(showsBox(espiao)).toBe(false);
  });

  it('starts ticked for a creature the players see and unticked for a hidden one', () => {
    expect(boxStart(candidate())).toBe(true);
    expect(boxStart(candidate({ hidden: true }))).toBe(false);
  });

  it('says who decides, naming the creature, and what a hidden one means for the bards', () => {
    expect(boxHint(candidate())).toBe(
      'O Capitão bandido não é medido pelo mapa: é você quem diz. Com o token da criatura e o de um bardo na grade do mapa, o app mede os 18 m e a visão.',
    );
    expect(boxHint(candidate({ name: 'Espião', hidden: true }))).toBe(
      'O Espião está escondido dos jogadores, então a caixa começa desmarcada: ninguém é perguntado. Se você marcar, os bardos leem “uma criatura”, sem o nome.',
    );
  });

  it('says the app measures a creature the map measures, and that a hidden token opens no question', () => {
    expect(boxHint(espiao)).toBe(
      'O token do Espião e os dos bardos estão no mapa: o app mede os 18 m e a visão. Com o token escondido, nenhum bardo é perguntado.',
    );
  });
});

describe('the check', () => {
  it('lists each check with its bonus, and says when there is none', () => {
    expect(checkOptions(candidate()).map((o) => [o.key, o.name, o.side])).toEqual([
      ['ability:cha', 'Carisma', '+2'],
      ['skill:deception', 'Enganação', '+4'],
      ['skill:persuasion', 'Persuasão', '+2'],
    ]);
    expect(checkOptions(bartolo)[0]).toMatchObject({ side: 'sem bônus', known: false });
    expect(checkOptions(candidate({ checks: [{ key: 'skill:x', name: 'X', bonus: -1, known: true }] }))[0].side).toBe('−1');
  });

  it('starts on Enganação when the creature has it, else on its first check', () => {
    expect(defaultCheck(candidate())).toBe('skill:deception');
    expect(defaultCheck(bartolo)).toBe('skill:persuasion');
    expect(defaultCheck(candidate({ checks: [] }))).toBe('');
  });

  it('says a check with no number rolls +0 and the master adds the bonus', () => {
    expect(noBonusLine(bartolo, 'skill:persuasion')).toBe(
      'Sem bônus para Persuasão: o d20 conta com +0 e você soma o bônus que achar justo.',
    );
    expect(noBonusLine(candidate(), 'skill:deception')).toBe('');
  });

  it('names the roll on the button, with the creature as the board writes it', () => {
    expect(rollLabel(candidate(), 'skill:deception')).toBe('Rolar Enganação do Capitão bandido');
    expect(rollLabel(candidate({ name: 'Brisa' }), 'skill:deception')).toBe('Rolar Enganação da Brisa');
  });

  it('writes "do" or "da" before a name', () => {
    expect(ofName('Espião')).toBe('do Espião');
    expect(ofName('Aranha-lobo')).toBe('da Aranha-lobo');
  });
});

describe('parseD20', () => {
  it('takes a whole number from 1 to 20', () => {
    expect(parseD20('11')).toBe(11);
    expect(parseD20(' 20 ')).toBe(20);
    expect(parseD20('0')).toBeNull();
    expect(parseD20('21')).toBeNull();
    expect(parseD20('1,5')).toBeNull();
    expect(parseD20('')).toBeNull();
    expect(parseD20('abc')).toBeNull();
  });
});
