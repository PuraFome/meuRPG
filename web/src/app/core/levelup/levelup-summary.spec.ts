import { changeRows, type SummaryContext } from './levelup-summary';
import { pensantus } from './levelup-testing';

const plain = (s: string) => s.replace(/ /g, ' ');

const ctx: SummaryContext = {
  hpSub: 'Média 4 + Constituição +3',
  cantrips: ['Prestidigitação'],
  spells: ['Passo Nebuloso', 'Imagem Espelhada'],
  prepared: ['Passo Nebuloso', 'Detectar Magia'],
  spellbook: true,
  spellsMissing: 0,
};

describe('changeRows: the summary of E8-15', () => {
  const rows = changeRows(pensantus(false), pensantus(true), ctx);
  const row = (key: string) => rows.find((r) => r.key === key);

  it('lists what moves, before → after, in the artboard order, and leaves out what does not', () => {
    expect(rows.map((r) => r.key)).toEqual([
      'level',
      'ability-int',
      'hp',
      'hit-dice',
      'dc',
      'attack',
      'cantrips',
      'spells',
      'slots-2',
      'prepared',
      'save-int',
      'skills-6>7',
      'passive-investigation',
    ]);
  });

  it('words each line', () => {
    expect(row('level')).toMatchObject({ before: 'Mago 3', after: 'Mago 4' });
    expect(row('ability-int')).toMatchObject({ label: 'Inteligência', before: '18', after: '20', sub: 'Modificador +4 → +5' });
    expect(row('hp')).toMatchObject({ before: '23', after: '30', sub: 'Média 4 + Constituição +3' });
    expect(row('hit-dice')).toMatchObject({ before: '3d6', after: '4d6' });
    expect(row('dc')).toMatchObject({ before: '14', after: '15' });
    expect(row('attack')).toMatchObject({ before: '+6', after: '+7' });
    expect(row('prepared')).toMatchObject({ before: '7', after: '9', sub: 'Novas: Passo Nebuloso e Detectar Magia' });
    expect(row('cantrips')).toMatchObject({ before: '3', after: '4', sub: 'Novo: Prestidigitação' });
    expect(row('slots-2')).toMatchObject({ label: 'Espaços de 2º círculo', before: '2', after: '3' });
    expect(row('save-int')).toMatchObject({ label: 'Salvaguarda de Inteligência', before: '+6', after: '+7' });
    expect(row('passive-investigation')).toMatchObject({ label: 'Investigação passiva', before: '16', after: '17' });
  });

  it('groups the skills that move by the same amount', () => {
    expect(row('skills-6>7')).toMatchObject({ label: 'Arcanismo e História', before: '+6', after: '+7' });
  });

  it('says how many spells of the book are still missing', () => {
    const half = changeRows(pensantus(false), pensantus(true), { ...ctx, spells: ['Passo Nebuloso'], spellsMissing: 1 });
    expect(plain(half.find((r) => r.key === 'spells')?.after ?? '')).toContain('(falta 1)');
  });
});
