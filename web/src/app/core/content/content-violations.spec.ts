import { create } from '@bufbuild/protobuf';

import { TableContentViolationSchema } from '../../../gen/meurpg/rules/v1/table_content_pb';
import { inputFor, parentOf, placeViolations, refusalSummary, shapeOf, violationText } from './content-violations';

const v = (field: string, reason: string, key = '') => create(TableContentViolationSchema, { field, reason, key });
const ctx = { aOne: 'uma magia', nameOf: (k: string) => (k === 'race:corujeiro@mesa' ? 'Corujeiro' : '') };

describe('the refusals of the server, back on the fields (E10-01 state 5)', () => {
  it('reads the path of a violation: its shape and its parent', () => {
    expect(shapeOf('table_race.traits[1].effects[0].value')).toBe('table_race.traits[].effects[].value');
    expect(parentOf('table_race.traits[1].effects[0].value')).toBe('table_race.traits[1].effects[0]');
    expect(parentOf('table_race.traits[1].effects[0]')).toBe('table_race.traits[1].effects');
    expect(parentOf('table_race.traits[1]')).toBe('table_race.traits');
    expect(parentOf('table_race')).toBe('');
  });

  it('finds the nearest input at or above a path', () => {
    const known = new Set(['table_race.traits[1].effects[0]', 'table_race.traits[1].name_pt']);
    expect(inputFor('table_race.traits[1].effects[0].value', (p) => known.has(p))).toBe('table_race.traits[1].effects[0]');
    expect(inputFor('table_race.traits[1].name_pt', (p) => known.has(p))).toBe('table_race.traits[1].name_pt');
    expect(inputFor('table_race.size', (p) => known.has(p))).toBe('');
  });

  // Copy of the Go table `entryViolationRows` in backend/internal/rules/overlay_paths_test.go (TestEntryViolationPaths): the
  // path and the reason the server sends for each field of a spell, a race, a sub-race and a background. Change a row there
  // and here together. `text` is what the app says.
  const SERVER_ROWS: readonly (readonly [string, string, string])[] = [
    ['table_spell.level', 'bad_value', 'Escolha um círculo de 0 (truque) a 9.'],
    ['table_spell.school_key', 'dangling_reference', 'Escolha a escola da magia.'],
    ['table_spell.range.kind', 'bad_value', 'Esse alcance não combina com o alvo. “Só quem conjura” pede Pessoal; criaturas escolhidas pedem distância ou Toque.'],
    ['table_spell.range.distance_ft', 'limit', 'O alcance vai de 1,5 m a 1.584 m, em passos de 1,5 m.'],
    ['table_spell.target.kind', 'bad_value', 'Este alvo não aceita os outros campos preenchidos. Confira o alvo.'],
    ['table_spell.target.shape', 'bad_value', 'Escolha a forma da área: cone, cubo, cilindro, linha ou esfera.'],
    ['table_spell.target.size_ft', 'limit', 'A área vai de 1,5 m a 90 m, em passos de 1,5 m.'],
    ['table_spell.target.count', 'limit', 'Escreva um número de criaturas: 2 ou mais, e até 10 a mais por círculo.'],
    ['table_spell.target.per_slot_level', 'limit', 'Escreva um número de criaturas: 2 ou mais, e até 10 a mais por círculo.'],
    ['table_spell.casting_time.unit', 'bad_value', 'Escolha o tempo de conjuração.'],
    ['table_spell.casting_time.amount', 'limit', 'O tempo vai de 1 a 60 minutos ou horas; uma ação vale 1.'],
    ['table_spell.casting_time.trigger_pt', 'bad_value', 'Só uma reação tem gatilho.'],
    ['table_spell.duration.kind', 'bad_value', 'Escolha a duração.'],
    ['table_spell.duration.amount', 'limit', 'A duração vai de 1 a 999.'],
    ['table_spell.duration.unit', 'bad_value', 'Escolha a unidade da duração.'],
    ['table_spell.concentration', 'bad_value', 'Concentração pede uma duração por tempo.'],
    ['table_spell.ritual', 'bad_value', 'Um truque não é um ritual.'],
    ['table_spell.components.material_pt', 'bad_value', 'O material vai com o componente M, e só com ele.'],
    ['table_spell.attack', 'bad_value', 'Uma magia de área não tem ataque. Escolha um teste de resistência, ou outro alvo.'],
    ['table_spell.save', 'bad_value', 'Uma magia tem ataque ou teste de resistência, não os dois.'],
    ['table_spell.save.ability', 'bad_value', 'Escolha a habilidade do teste de resistência.'],
    ['table_spell.save.on_success', 'bad_value', 'Escolha o que acontece ao passar: metade do dano (que pede dano) ou nada.'],
    ['table_spell.damage', 'limit', 'Uma magia tem até 4 tipos de dano.'],
    ['table_spell.damage[0].dice', 'bad_value', 'Escreva o dado assim: 2d8 (de 1 a 20 dados).'],
    ['table_spell.damage[1].dice', 'bad_value', 'Escreva o dado assim: 2d8 (de 1 a 20 dados).'],
    ['table_spell.damage[0].per_slot_level', 'bad_value', 'Escreva o dado assim: 2d8 (de 1 a 20 dados).'],
    ['table_spell.damage[0].per_tier', 'bad_value', 'Escreva o dado assim: 2d8 (de 1 a 20 dados).'],
    ['table_spell.damage[0].damage_type_key', 'dangling_reference', 'Escolha um tipo de dano da lista.'],
    ['table_spell.heal.dice', 'bad_value', 'Escreva o dado assim: 2d8 (de 1 a 20 dados).'],
    ['table_spell.heal.per_slot_level', 'bad_value', 'Escreva o dado assim: 2d8 (de 1 a 20 dados).'],
    ['table_spell.heal', 'bad_value', 'Um truque não cura.'],
    ['table_spell.desc_pt[1]', 'bad_text', 'Este texto é longo demais. Encurte o parágrafo.'],
    ['table_spell.class_keys[0]', 'dangling_reference', 'Esta classe não existe mais. Tire-a da lista.'],
    ['table_spell.class_keys[1]', 'bad_value', 'Esta classe está na lista duas vezes.'],
    ['table_race.size', 'bad_value', 'Escolha o tamanho.'],
    ['table_race.speed_ft', 'limit', 'O deslocamento vai de 1,5 m a 36 m, em passos de 1,5 m.'],
    ['table_race.darkvision_ft', 'limit', 'A visão no escuro é 0 ou vai de 1,5 m a 36 m, em passos de 1,5 m.'],
    ['table_race.ability_bonuses.wisdom', 'limit', 'O bônus vai de −4 a +4.'],
    ['table_race.choice_bonuses', 'limit', 'No máximo 6 bônus à escolha.'],
    ['table_race.choice_bonuses[1]', 'limit', 'Cada bônus à escolha vai de +1 a +4.'],
    ['table_race.language_choices', 'limit', 'Idiomas à escolha: de 0 a 4.'],
    ['table_race.languages[1]', 'dangling_reference', 'Este idioma não existe. Escolha da lista.'],
    ['table_race.traits[0].effects[0].roll', 'bad_value', 'Este valor não serve para este efeito.'],
    ['table_subrace.race_key', 'dangling_reference', 'A raça desta sub-raça não existe mais. Crie a sub-raça numa raça que exista.'],
    ['table_subrace.ability_bonuses.dexterity', 'limit', 'O bônus vai de −4 a +4.'],
    ['table_background.skills', 'bad_value', 'Escolha duas perícias diferentes.'],
    ['table_background.skills[1]', 'dangling_reference', 'Esta perícia não existe. Escolha da lista.'],
    ['table_background.tools', 'limit', 'No máximo 4 ferramentas.'],
    ['table_background.tools[0]', 'dangling_reference', 'Esta não é uma ferramenta do SRD. Escolha da lista.'],
    ['table_background.language_choices', 'limit', 'Idiomas à escolha: de 0 a 4.'],
    ['table_background.equipment_pt', 'bad_text', 'O equipamento é longo demais. Encurte o texto.'],
    ['table_background.feature.effects[0].type', 'forbidden_effect', 'Este efeito não está no menu.'],
    ['table_spell.name_pt', 'duplicate_name', 'Já existe uma magia da mesa com este nome. Escolha outro.'],
    ['table_spell.name_pt', 'bad_name', 'O nome tem de 1 a 60 letras.'],
    ['table_race.traits[1].effects[0].value', 'bad_formula', 'Esta fórmula não funciona. Use só as funções da lista e confira os parênteses.'],
    ['table_race.traits[0].effects[0].resource', 'bad_value', 'Use só letras minúsculas sem acento, números e _ (de 1 a 40), como vigilia.'],
    ['table_spell.level', 'immutable', 'Uma magia não passa de truque para círculo, nem o contrário. Crie outra.'],
    ['', 'limit', 'A mesa já tem 300 entradas. Arquive ou use o que já existe.'],
    ['', 'size_limit', 'Esta entrada passou de 64 KiB de dados. Tire um pouco de texto.'],
  ];

  it.each(SERVER_ROWS)('writes %s (%s) in Portuguese from the code and the path, never the message', (field, reason, text) => {
    expect(violationText({ field, reason }, ctx)).toBe(text);
  });

  it('never gives a spell\'s saving throw a class\'s field, and never says the generic line for a path of the table', () => {
    for (const [field, reason] of SERVER_ROWS.filter(([f]) => f !== '')) {
      expect(field).not.toContain('saving_throws');
      expect(violationText({ field, reason }, ctx)).not.toBe('Este valor não é aceito. Confira o campo.');
    }
  });

  it('puts each violation on its input, in the order the server listed them, and counts them', () => {
    const known = new Set(['table_spell.name_pt', 'table_spell.range.distance_ft', 'table_spell.damage[0].dice']);
    const placed = placeViolations(
      [v('table_spell.name_pt', 'duplicate_name'), v('table_spell.range.distance_ft', 'limit'), v('table_spell.damage[0].dice', 'bad_value')],
      (p) => known.has(p),
      ctx,
    );
    expect(placed.fields).toEqual(['table_spell.name_pt', 'table_spell.range.distance_ft', 'table_spell.damage[0].dice']);
    expect(placed.byField.get('table_spell.name_pt')?.[0].text).toContain('Já existe uma magia da mesa');
    expect(placed.top).toEqual([]);
    expect(placed.count).toBe(3);
    expect(refusalSummary('a magia', placed.count)).toBe('Não foi possível salvar a magia. 3 campos precisam de ajuste. Nada foi salvo; o que você digitou continua aqui.');
    expect(refusalSummary('a raça', 1)).toContain('1 campo precisa de ajuste');
  });

  it('takes a path with no input to the nearest one above, and a violation with no field or about another entry to the top', () => {
    const known = new Set(['table_race.traits[1].effects[0]']);
    const placed = placeViolations(
      [v('table_race.traits[1].effects[0].value', 'bad_formula'), v('', 'limit'), v('table_race.name_pt', 'duplicate_name', 'race:corujeiro@mesa'), v('table_race.size', 'bad_value')],
      (p) => known.has(p),
      { aOne: 'uma raça', nameOf: ctx.nameOf },
    );
    expect(placed.fields).toEqual(['table_race.traits[1].effects[0]']);
    expect(placed.top.map((t) => t.text)).toEqual([
      'A mesa já tem 300 entradas. Arquive ou use o que já existe.',
      'Sobre Corujeiro: Já existe uma raça da mesa com este nome. Escolha outro.',
      'Escolha o tamanho.',
    ]);
  });

  it('says the same sentence once on one input', () => {
    const placed = placeViolations([v('a.b', 'bad_value'), v('a.b', 'bad_value')], (p) => p === 'a.b', ctx);
    expect(placed.byField.get('a.b')).toHaveLength(1);
  });
});
