import { create } from '@bufbuild/protobuf';

import {
  TableClassLevelSchema,
  TableClassSchema,
  TableContentKind,
  TableContentViolationSchema,
  TableSubclassLevelSchema,
  TableSubclassSchema,
  TableRaceSchema,
  TableSubraceSchema,
  type TableContentViolation,
} from '../../../gen/meurpg/rules/v1/table_content_pb';
import { entry, feature } from './content-testing';
import {
  NO_LEFT_OUT,
  buildLeftOut,
  leftOutTitle,
  parentKeyOf,
  placeOfPath,
  togetherWith,
  usingLineOf,
  violationOfEntry,
} from './left-out';

function v(key: string, field: string, reason: string): TableContentViolation {
  return create(TableContentViolationSchema, { key, field, reason, message: 'english' });
}

const TARGET = 'table_class.levels[0].features[0].effects[0].target';

function guardiao(using = 1) {
  return entry(TableContentKind.CLASS, 'Guardião do Vale', {
    key: 'class:guardiao@mesa',
    charactersUsing: using,
    body: {
      case: 'tableClass',
      value: create(TableClassSchema, {
        namePt: 'Guardião do Vale',
        hitDie: 10,
        levels: [
          create(TableClassLevelSchema, {
            features: [
              feature('Vigília', [{ type: 'modifier', target: 'gone', mode: 'add', value: '2' }]),
            ],
          }),
        ],
      }),
    },
  });
}

describe('where a violation is, in words', () => {
  it('names the level, the feature, the effect and the field of a class feature effect', () => {
    expect(placeOfPath(guardiao(), TARGET)).toEqual({
      parts: ['Nível 1', 'Vigília', 'efeito 1'],
      field: 'O que muda',
    });
  });

  it('takes the level of a subclass from the level itself, not from its place in the list', () => {
    const sub = entry(TableContentKind.SUBCLASS, 'Juramento da Névoa', {
      body: {
        case: 'tableSubclass',
        value: create(TableSubclassSchema, {
          namePt: 'Juramento da Névoa',
          classKey: 'class:guardiao@mesa',
          levels: [
            create(TableSubclassLevelSchema, {
              level: 3,
              features: [feature('Véu', [{ type: 'modifier' }])],
            }),
          ],
        }),
      },
    });
    expect(placeOfPath(sub, 'table_subclass.levels[0].features[0].effects[0].value')).toEqual({
      parts: ['Nível 3', 'Véu', 'efeito 1'],
      field: 'Valor',
    });
  });

  it('names a race trait and its option, and says "traço 2" for a trait with no name', () => {
    const race = entry(TableContentKind.RACE, 'Corujeiro', {
      body: {
        case: 'tableRace',
        value: create(TableRaceSchema, {
          namePt: 'Corujeiro',
          traits: [feature('Olho de coruja'), { ...feature('x'), namePt: '', options: [] }],
        }),
      },
    });
    expect(placeOfPath(race, 'table_race.traits[0].effects[1].mode').parts).toEqual([
      'Olho de coruja',
      'efeito 2',
    ]);
    expect(placeOfPath(race, 'table_race.traits[1].name_pt')).toEqual({
      parts: ['traço 2'],
      field: 'Nome',
    });
    expect(placeOfPath(race, 'table_race.traits[0].options[3].name_pt')).toEqual({
      parts: ['Olho de coruja', 'opção 4'],
      field: 'Nome',
    });
  });

  it('labels a field of the entry itself, and uses the label of the nearest field for a deeper path', () => {
    expect(placeOfPath(guardiao(), 'table_class.hit_die')).toEqual({
      parts: [],
      field: 'Dado de vida',
    });
    expect(placeOfPath(guardiao(), 'table_class.casting.list_from').field).toBe('Lista de magias');
    expect(placeOfPath(guardiao(), 'table_class.skill_from[2]').field).toBe('Perícias da lista');
  });

  it('says what it can for the entry as a whole, a feature of a background, a path it cannot read and an index past the end', () => {
    expect(placeOfPath(guardiao(), '')).toEqual({ parts: [], field: '' });
    expect(placeOfPath(guardiao(), 'table_class.levels[7].features[0].name_pt').parts).toEqual([
      'Nível 8',
      'característica 1',
    ]);
    expect(placeOfPath(guardiao(), 'table_class.@@.hit_die').field).toBe('Dado de vida');
    const bg = entry(TableContentKind.BACKGROUND, 'Cartógrafo', {});
    expect(placeOfPath(bg, 'table_background.feature.effects[0].target').parts).toEqual([
      'característica',
      'efeito 1',
    ]);
  });
});

describe('the entries left out', () => {
  const sub = entry(TableContentKind.SUBCLASS, 'Juramento da Névoa', {
    key: 'subclass:juramento@mesa',
    body: {
      case: 'tableSubclass',
      value: create(TableSubclassSchema, {
        namePt: 'Juramento da Névoa',
        classKey: 'class:guardiao@mesa',
      }),
    },
  });
  const talento = entry(TableContentKind.FEAT, 'Chama Insistente', {
    key: 'feat:chama@mesa',
    charactersUsing: 3,
  });
  const ok = entry(TableContentKind.RACE, 'Corujeiro');

  it('has nothing for a campaign with nothing left out', () => {
    expect(buildLeftOut([guardiao(), ok], [])).toBe(NO_LEFT_OUT);
    expect(NO_LEFT_OUT.count).toBe(0);
  });

  it('makes a row of each entry to fix, with its subclass under the class, and counts all three', () => {
    const model = buildLeftOut(
      [guardiao(), sub, talento, ok],
      [
        v('class:guardiao@mesa', TARGET, 'bad_value'),
        v('subclass:juramento@mesa', '', 'dangling_reference'),
        v('feat:chama@mesa', 'table_feat.effects[0].damage_type', 'bad_value'),
      ],
    );
    expect(model.count).toBe(3);
    expect([...model.keys].sort()).toEqual([
      'class:guardiao@mesa',
      'feat:chama@mesa',
      'subclass:juramento@mesa',
    ]);
    expect(model.rows.map((r) => r.entry.namePt)).toEqual(['Guardião do Vale', 'Chama Insistente']);
    const [classe, feat] = model.rows;
    expect(classe.placeLine).toBe('Nível 1 · Vigília · efeito 1');
    expect(classe.shortPlace).toBe('Vigília, efeito 1');
    expect(classe.place.field).toBe('O que muda');
    expect(classe.text).toBe('este alvo não está mais no menu de efeitos. Escolha outro.');
    expect(classe.kindWord).toBe('Classe');
    expect(classe.usingLine).toBe('1 ficha usa esta classe.');
    expect(classe.usingShort).toBe('1 ficha usa');
    expect(classe.dependents.map((d) => d.entry.namePt)).toEqual(['Juramento da Névoa']);
    expect(classe.dependents[0].text).toBe(
      'Saiu porque a classe dela, Guardião do Vale, está fora. Volta sozinha quando a classe voltar.',
    );
    expect(feat.dependents).toEqual([]);
    expect(feat.usingLine).toBe('3 fichas usam este talento.');
  });

  it('keeps an entry whose parent is not out as a row of its own, with the reason it has', () => {
    const model = buildLeftOut(
      [sub],
      [v('subclass:juramento@mesa', 'table_subclass.class_key', 'dangling_reference')],
    );
    expect(model.rows).toHaveLength(1);
    expect(model.rows[0].dependents).toEqual([]);
    expect(model.rows[0].text).toBe('A classe desta subclasse não existe mais.');
  });

  it('keeps a subclass that is out for a reason of its own as a row, even under a class that is out', () => {
    const model = buildLeftOut(
      [guardiao(), sub],
      [
        v('class:guardiao@mesa', TARGET, 'bad_value'),
        v('subclass:juramento@mesa', 'table_subclass.level', 'bad_value'),
      ],
    );
    expect(model.rows.map((r) => r.entry.namePt)).toEqual([
      'Guardião do Vale',
      'Juramento da Névoa',
    ]);
    expect(model.rows[0].dependents).toEqual([]);
  });

  it('says "raça" for a subrace and the class feature follows its class', () => {
    const race = entry(TableContentKind.RACE, 'Corujeiro', { key: 'race:corujeiro@mesa' });
    const subrace = entry(TableContentKind.SUBRACE, 'Das Torres', {
      key: 'subrace:torres@mesa',
      body: {
        case: 'tableSubrace',
        value: create(TableSubraceSchema, { namePt: 'Das Torres', raceKey: 'race:corujeiro@mesa' }),
      },
    });
    expect(parentKeyOf(subrace)).toBe('race:corujeiro@mesa');
    const model = buildLeftOut(
      [race, subrace],
      [
        v('race:corujeiro@mesa', 'table_race.speed_ft', 'bad_value'),
        v('subrace:torres@mesa', '', 'dangling_reference'),
      ],
    );
    expect(model.rows[0].dependents[0].text).toContain('a raça dela, Corujeiro');
    const feature_ = entry(TableContentKind.CLASS_FEATURE, 'Golpe');
    expect(parentKeyOf(feature_)).toBe('class:wizard');
    expect(parentKeyOf(ok)).toBe('');
  });

  it('says the entries as a whole, "Fora da campanha", with the right number', () => {
    expect(leftOutTitle(1)).toBe('1 entrada fora da campanha');
    expect(leftOutTitle(3)).toBe('3 entradas fora da campanha');
  });

  it('writes "Junto com ela" for one dependent and for many', () => {
    const model = buildLeftOut(
      [guardiao(), sub],
      [
        v('class:guardiao@mesa', TARGET, 'bad_value'),
        v('subclass:juramento@mesa', '', 'dangling_reference'),
      ],
    );
    expect(togetherWith(model.rows[0])).toEqual({
      names: [{ name: 'Juramento da Névoa', word: 'subclasse' }],
      tail: 'que depende dela',
    });
    const two = {
      ...model.rows[0],
      dependents: [...model.rows[0].dependents, ...model.rows[0].dependents],
    };
    expect(togetherWith(two).tail).toBe('que dependem dela');
  });

  it('writes no line for an entry no sheet uses, and finds the violation of an entry by its key', () => {
    expect(usingLineOf(guardiao(0))).toBe('');
    const list = [v('a', 'x', 'bad_value'), v('b', 'y', 'bad_value')];
    expect(violationOfEntry(list, 'b')?.field).toBe('y');
    expect(violationOfEntry(list, 'c')).toBeUndefined();
  });
});
