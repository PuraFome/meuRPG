import {
  type CastingDraft,
  type ClassDraft,
  type RowDraft,
  emptyCasting,
  emptyClass,
} from '../../../core/content/class-draft';
import { classDefaults } from '../../../core/content/content-testing';
import {
  ASI_OPTIONS,
  GROUP_KEYS,
  HIT_DICE,
  LEVEL_NUMBERS,
  addedRequirement,
  answeredQuestion,
  classViolationContext,
  editedRows,
  emptyDefaults,
  gridRowsOf,
  isKnownPath,
  movedRequirement,
  questionTextOf,
  removedRequirement,
  sectionHasIssue,
  subclassesOf,
  tableNoteOf,
  toggled,
  typedRequirement,
  withProficiency,
} from './class-editor-model';

describe('the class editor model', () => {
  const defaults = classDefaults();
  const name = (k: string) => `<${k}>`;
  const draft = (): ClassDraft => emptyClass(defaults);
  const casting = (kind: CastingDraft['kind'], preparation: CastingDraft['preparation']) => ({
    ...emptyCasting(kind),
    preparation,
  });

  it('puts a refusal in the section that owns its path', () => {
    const fields = ['table_class.levels[3].features[0]', 'table_class.hit_die'];
    expect(sectionHasIssue('features', fields)).toBe(true);
    expect(sectionHasIssue('basics', fields)).toBe(true);
    expect(sectionHasIssue('table', fields)).toBe(false);
    expect(sectionHasIssue('table', ['table_class.levels[3].slots[0]'])).toBe(true);
    expect(sectionHasIssue('table', ['table_class.asi_levels'])).toBe(true);
    expect(sectionHasIssue('profs', ['table_class.any_of.wisdom'])).toBe(true);
    expect(sectionHasIssue('casting', ['table_class.casting.kind'])).toBe(true);
    expect(sectionHasIssue('subclasses', ['table_class.subclass_level'])).toBe(true);
    expect(sectionHasIssue('subclasses', ['table_class.hit_die'])).toBe(false);
  });

  it('lists the levels, the dice and the ASI options', () => {
    expect(HIT_DICE.map((o) => o.label)).toEqual(['d6', 'd8', 'd10', 'd12']);
    expect(LEVEL_NUMBERS).toHaveLength(20);
    expect(ASI_OPTIONS[3]).toEqual({ value: '4', label: '4' });
    expect(emptyDefaults().subclassLevel).toBe(3);
  });

  it('draws one grid row per level with the locked chips and the features of the level', () => {
    const d = { ...draft(), asiLevels: [4], subclassLevel: 3 };
    d.features = [
      {
        id: 'f1',
        level: 4,
        feature: { ...({} as ClassDraft['features'][0]['feature']), name: ' ' },
      },
    ];
    const rows = gridRowsOf(d);
    expect(rows).toHaveLength(20);
    expect(rows[2].chips).toEqual([{ id: 'sub-3', label: 'Escolha de subclasse', locked: true }]);
    expect(rows[3].chips.map((c) => c.label)).toEqual([
      'Incremento no Valor de Habilidade',
      'Sem nome',
    ]);
    expect(rows[3].base).toBe('table_class.levels[3]');
  });

  it('says where the numbers of the table come from', () => {
    expect(tableNoteOf(emptyCasting(''), defaults, name)).toContain('sem conjuração');
    expect(tableNoteOf(casting('half', 'prepared'), defaults, name)).toContain(
      'Preenchida com a tabela do <class:paladin> do SRD.',
    );
    expect(tableNoteOf(casting('half', 'known'), defaults, name)).toContain('conhecidas');
    expect(tableNoteOf(casting('third', 'prepared'), defaults, name)).toContain('de um terço');
  });

  it('words the question about an edited table, for a restore and for a new way of casting', () => {
    const half = casting('half', 'prepared');
    expect(questionTextOf(null, half, defaults, name)).toBe('');
    expect(questionTextOf({ kind: 'restore' }, half, defaults, name)).toBe(
      'Voltar a tabela ao padrão do <class:paladin>? O que você editou nela se perde.',
    );
    expect(
      questionTextOf({ kind: 'casting', next: casting('full', 'prepared') }, half, defaults, name),
    ).toContain('padrão do <class:cleric> para esta conjuração');
  });

  it('types one cell and leaves the other rows', () => {
    const rows: RowDraft[] = draft().rows;
    expect(editedRows(rows, { level: 2, field: 'profBonus', slot: 0, value: 7 })[1].profBonus).toBe(
      7,
    );
    expect(editedRows(rows, { level: 2, field: 'cantrips', slot: 0, value: 3 })[1].cantrips).toBe(
      3,
    );
    expect(editedRows(rows, { level: 2, field: 'spells', slot: 0, value: 4 })[1].spells).toBe(4);
    const slot = editedRows(rows, { level: 2, field: 'slot', slot: 0, value: 5 });
    expect(slot[1].slots[0]).toBe(5);
    expect(slot[0]).toBe(rows[0]);
  });

  it('toggles a skill and puts a proficiency group on once', () => {
    expect(toggled(['a', 'b'], 'a')).toEqual(['b']);
    expect(toggled(['a'], 'b')).toEqual(['a', 'b']);
    expect(withProficiency(['x', 'y'], 'x', true)).toEqual(['y', 'x']);
    expect(withProficiency(['x', 'y'], 'x', false)).toEqual(['y']);
    expect(GROUP_KEYS).toContain('proficiency:shields');
  });

  it('adds, removes, moves and types the multiclass requirements', () => {
    const d = { ...draft() };
    expect(addedRequirement(d, 'minimums', 'wisdom')['wisdom']).toBe(13);
    expect(removedRequirement(d, 'anyOf', 'wisdom')['wisdom']).toBe(0);
    const two = { ...d, anyOf: { ...d.anyOf, strength: 14, wisdom: 13 } };
    expect(movedRequirement(two, 'anyOf', 'strength', 'wisdom')).toBeNull();
    expect(movedRequirement(two, 'anyOf', 'strength', 'strength')).toBeNull();
    expect(movedRequirement(two, 'anyOf', 'strength', 'dexterity')).toMatchObject({
      strength: 0,
      dexterity: 14,
    });
    expect(typedRequirement(d, 'minimums', 'wisdom', '15 pontos')['wisdom']).toBe(15);
    expect(typedRequirement(d, 'minimums', 'wisdom', '')['wisdom']).toBe(1);
  });

  it('answers the table question: restore only when asked to replace, a new way of casting either way', () => {
    const d = draft();
    expect(answeredQuestion({ kind: 'restore' }, false, d, defaults)).toBeNull();
    expect(answeredQuestion({ kind: 'restore' }, true, d, defaults)?.status).toBe(
      'A tabela voltou ao padrão.',
    );
    const next = casting('full', 'prepared');
    const kept = answeredQuestion({ kind: 'casting', next }, false, d, defaults);
    expect(kept?.patch.rows).toBe(d.rows);
    expect(kept?.status).toBe('A conjuração mudou; a tabela ficou como estava.');
    const replaced = answeredQuestion({ kind: 'casting', next }, true, d, defaults);
    expect(replaced?.patch.casting).toBe(next);
    expect(replaced?.status).toBe('A tabela foi refeita com o padrão.');
  });

  it('knows the paths the page has an input for, and the minimums only while they are on', () => {
    const d = { ...draft(), minimums: { ...draft().minimums, wisdom: 13 } };
    const cols = { casts: false, cantrips: false, spells: false, circles: 0 };
    const menu = { list: () => [] } as never;
    expect(isKnownPath('table_class.hit_die', d, cols, menu)).toBe(true);
    expect(isKnownPath('table_class.minimums.wisdom', d, cols, menu)).toBe(true);
    expect(isKnownPath('table_class.minimums.strength', d, cols, menu)).toBe(false);
    expect(isKnownPath('table_class.any_of.wisdom', d, cols, menu)).toBe(false);
    expect(isKnownPath('table_class.nonsense', d, cols, menu)).toBe(false);
  });

  it('lists the subclasses of a class, and the refusal context reads the editor when it is asked', () => {
    const sub = {
      key: 's1',
      namePt: 'Sub',
      body: { case: 'tableSubclass', value: { classKey: 'c1' } },
    };
    const other = { key: 's2', namePt: 'Other', body: { case: 'tableRace', value: {} } };
    expect(subclassesOf([sub, other] as never, 'c1')).toEqual([sub]);
    expect(subclassesOf([sub] as never, '')).toEqual([]);
    let features = 1;
    const ctx = classViolationContext({
      entries: () => [sub] as never,
      menu: () => ({ maxFeatures: 60 }) as never,
      draft: () => ({ features: Array(features) }) as never,
    });
    expect(ctx.nameOf?.('s1')).toBe('Sub');
    expect(ctx.nameOf?.('zzz')).toBe('');
    features = 61;
    expect(ctx.featureCount).toBe(61);
    expect(ctx.maxFeatures).toBe(60);
    expect(ctx.redirect?.('table_class.levels[2].features[4]', 'limit')).toBe(
      'table_class.features',
    );
    expect(ctx.redirect?.('table_class.levels[2].features[4]', 'other')).toBe(
      'table_class.levels[2].features[4]',
    );
  });
});
