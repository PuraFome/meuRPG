import { changeRows, type SummaryContext } from './levelup-summary';
import { pensantus } from './levelup-testing';

const plain = (s: string) => s.replace(/ /g, ' ');

const ctx: SummaryContext = {
  hpSub: 'Média 4 + Constituição +3',
  cantrips: ['Prestidigitação'],
  spells: ['Passo Nebuloso', 'Reflexos'],
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
    expect(row('slots-2')).toMatchObject({ label: 'Espaços de 2º nível', before: '2', after: '3' });
    expect(row('save-int')).toMatchObject({ label: 'Teste de resistência de Inteligência', before: '+6', after: '+7' });
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

describe('changeRows: a table class (E10-02 state 7)', () => {
  const ctxTable: SummaryContext = { ...ctx, table: true, newFeatures: ['Estilo de luta', 'Conjuração'] };
  const rows = changeRows(pensantus(false), pensantus(true), ctxTable);
  const row = (key: string) => rows.find((r) => r.key === key);

  it('tags the slots that come from the class table with "Da mesa", and says where they come from', () => {
    expect(row('slots-2')).toMatchObject({ table: true, sub: 'Da tabela da classe', before: '2', after: '3' });
  });

  it('tags only the slots: the hit points and the rest are the same rows as the SRD\'s', () => {
    expect(rows.filter((r) => r.table).map((r) => r.key)).toEqual(['slots-2']);
  });

  it('lists the features the level gives, by name, with their count, and no before', () => {
    expect(row('features')).toMatchObject({ label: 'Novas características', before: '', after: '2', sub: expect.any(String) });
    expect(plain(row('features')?.sub ?? '')).toBe('Estilo de luta · Conjuração');
  });

  it('shows only the rows that change: an SRD class has no tag and no feature row unless there are features', () => {
    const srd = changeRows(pensantus(false), pensantus(true), ctx);
    expect(srd.some((r) => r.table)).toBe(false);
    expect(srd.some((r) => r.key === 'features')).toBe(false);
    expect(srd.find((r) => r.key === 'slots-2')?.sub).toBe('');
  });
});

describe('changeRows: a class that starts casting, and how it learns its spells (10.12b fix round 1)', () => {
  const none = (s: DerivedSheetLike) => s;
  type DerivedSheetLike = ReturnType<typeof pensantus>;
  const nonCasterBefore = () => {
    const before = pensantus(false);
    before.spellcasting = [];
    before.spellSlots = [];
    before.spells = [];
    return none(before);
  };

  it('shows a dash, not 0 or +0, before the class casts', () => {
    const rows = changeRows(nonCasterBefore(), pensantus(true), ctx);
    expect(rows.find((r) => r.key === 'dc')).toMatchObject({ before: '—', after: '15' });
    expect(rows.find((r) => r.key === 'attack')).toMatchObject({ before: '—', after: '+7' });
    expect(rows.find((r) => r.key === 'cantrips')).toMatchObject({ before: '—' });
    expect(rows.find((r) => r.key === 'prepared')).toMatchObject({ before: '—', after: '9' });
  });

  it('a class that prepares from its list shows "Magias preparadas" and no "Magias conhecidas"', () => {
    const rows = changeRows(pensantus(false), pensantus(true), { ...ctx, learnsSpells: false, spells: [] });
    expect(rows.some((r) => r.key === 'spells')).toBe(false);
    expect(rows.some((r) => r.key === 'prepared')).toBe(true);
  });

  it('a class that learns its spells keeps "Magias conhecidas" (or the book), and a known-spells class has no prepared row', () => {
    expect(changeRows(pensantus(false), pensantus(true), { ...ctx, learnsSpells: true }).some((r) => r.key === 'spells')).toBe(true);
    const bard = pensantus(true);
    bard.spellcasting[0].preparedMax = 0;
    const rows = changeRows(pensantus(false), bard, { ...ctx, learnsSpells: true });
    expect(rows.some((r) => r.key === 'prepared')).toBe(false);
  });
});

describe('changeRows: a row with no gain is not shown', () => {
  it('has no "Truques" row for a class that has no cantrips: nothing before, 0 after', () => {
    const before = pensantus(false);
    before.spellcasting = [];
    before.spellSlots = [];
    before.spells = [];
    const after = pensantus(true);
    after.spellcasting[0].cantripsKnown = 0;
    const rows = changeRows(before, after, ctx);
    expect(rows.some((r) => r.key === 'cantrips')).toBe(false);
    expect(rows.some((r) => r.key === 'dc')).toBe(true);
  });

  it('keeps it when the class gets cantrips, and when it already had some', () => {
    const before = pensantus(false);
    before.spellcasting = [];
    expect(changeRows(before, pensantus(true), ctx).find((r) => r.key === 'cantrips')).toMatchObject({ before: '—', after: '4' });
    expect(changeRows(pensantus(false), pensantus(true), ctx).some((r) => r.key === 'cantrips')).toBe(true);
  });
});
