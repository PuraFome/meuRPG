import { violationText } from './content-violations';

/** The sentence of every reason at a field of each shape: the branches `violationText` walks, in the order it walks them. */
const CTX = { aOne: 'uma magia' };
const CLASS_CTX = { aOne: 'uma classe', maxFeatures: 40, featureCount: 41 };

const CASES: Record<string, [field: string, reason: string, ctx?: typeof CLASS_CTX]> = {
  'entry limit': ['', 'limit'],
  'entry size': ['', 'size_limit'],
  'entry overlay': ['', 'overlay'],
  'entry other': ['', 'anything'],
  'name duplicate': ['table_race.name_pt', 'duplicate_name'],
  'name bad': ['table_race.name_pt', 'bad_name'],
  'name bad text': ['table_race.name_pt', 'bad_text'],
  'name reserved': ['table_race.name_pt', 'reserved_key'],
  'name bad key': ['table_race.name_pt', 'bad_key'],
  'name duplicate key': ['table_race.name_pt', 'duplicate_key'],
  'name other reason': ['table_race.name_pt', 'bad_value'],
  'name immutable': ['table_race.name_pt', 'immutable'],
  'level immutable': ['table_spell.level', 'immutable'],
  'immutable elsewhere': ['table_spell.school_key', 'immutable'],
  'size anywhere': ['table_spell.desc_pt[0]', 'size_limit'],
  'bad formula': ['table_race.traits[0].effects[0].value', 'bad_formula'],
  'forbidden effect': ['table_race.traits[0].effects[0].type', 'forbidden_effect'],
  'resource bad value': ['table_race.traits[0].effects[0].resource', 'bad_value'],
  'effect value bad value': ['table_race.traits[0].effects[0].value', 'bad_value'],
  'effect other reason': ['table_race.traits[0].effects[0].when', 'required'],
  'dangling anywhere': ['table_race.languages[0]', 'dangling_reference'],
  'dangling class key': ['table_spell.class_keys[0]', 'dangling_reference'],
  'class key twice': ['table_spell.class_keys[1]', 'duplicate_key'],
  'skills dangling': ['table_background.skills[0]', 'dangling_reference'],
  'skills twice': ['table_background.skills[0]', 'bad_value'],
  'text too long': ['table_spell.components.material_pt', 'bad_text'],
  'text with no component': ['table_spell.components.material_pt', 'bad_value'],
  'unknown path, bad text': ['table_x.unknown', 'bad_text'],
  'unknown path, limit': ['table_x.unknown', 'limit'],
  'unknown path, duplicate name': ['table_x.unknown', 'duplicate_name'],
  'unknown path, duplicate key': ['table_x.unknown', 'duplicate_key'],
  'unknown path, any reason': ['table_x.unknown', 'zzz'],
  'class level slots': ['table_class.levels[3].slots[1]', 'bad_value'],
  'class features limit': ['table_class.levels[3].features[1]', 'limit', CLASS_CTX],
  'class features limit, no count': ['table_class.levels[3].features[1]', 'limit', CTX as never],
  'subclass features limit': ['table_subclass.levels[0].features[0]', 'limit', CLASS_CTX],
  'subclass features limit, defaults': [
    'table_subclass.levels[0].features[0]',
    'limit',
    CTX as never,
  ],
  'class feature other reason': ['table_class.levels[3].features[1]', 'bad_value'],
  'feat prerequisite': ['table_feat.prerequisite.minimums.strength', 'bad_value'],
  'spell damage dice': ['table_spell.damage[0].dice', 'bad_value'],
};

const EXPECTED: Record<string, string> = {
  'entry limit': 'A mesa já tem 1000 entradas. Arquive ou use o que já existe.',
  'entry size': 'Esta entrada passou de 64 KiB de dados. Tire um pouco de texto.',
  'entry overlay': 'O conteúdo da mesa não fecha com esta entrada. Confira o que ela usa.',
  'entry other': 'O servidor recusou esta entrada. Confira os campos.',
  'name duplicate': 'Já existe uma magia da mesa com este nome. Escolha outro.',
  'name bad': 'O nome tem de 1 a 60 letras.',
  'name bad text': 'O nome tem de 1 a 60 letras.',
  'name reserved': 'Este nome não pode ser usado. Escolha outro.',
  'name bad key': 'Este nome não pode ser usado. Escolha outro.',
  'name duplicate key': 'Este nome não pode ser usado. Escolha outro.',
  'name other reason': 'Este valor não é aceito. Confira o campo.',
  'name immutable': 'Isto não muda depois de criado.',
  'level immutable': 'Um truque não vira magia de 1º nível ou mais, nem o contrário. Crie outra.',
  'immutable elsewhere': 'Isto não muda depois de criado.',
  'size anywhere': 'Esta entrada passou de 64 KiB de dados. Tire um pouco de texto.',
  'bad formula': 'Esta fórmula não funciona. Use só as funções da lista e confira os parênteses.',
  'forbidden effect': 'Este efeito não está no menu.',
  'resource bad value':
    'Use só letras minúsculas sem acento, números e _ (de 1 a 40), como vigilia.',
  'effect value bad value': 'Este valor não serve para este efeito.',
  'effect other reason': 'Confira este campo do efeito.',
  'dangling anywhere': 'Este idioma não existe. Escolha da lista.',
  'dangling class key': 'Esta classe não existe mais. Tire-a da lista.',
  'class key twice': 'Esta classe está na lista duas vezes.',
  'skills dangling': 'Esta perícia não existe. Escolha da lista.',
  'skills twice': 'Escolha duas perícias diferentes.',
  'text too long': 'Este texto é longo demais. Encurte o parágrafo.',
  'text with no component': 'O material vai com o componente M, e só com ele.',
  'unknown path, bad text': 'Este texto é longo demais. Encurte o parágrafo.',
  'unknown path, limit': 'Isto passou do limite.',
  'unknown path, duplicate name': 'Já existe uma entrada com isto. Escolha outro.',
  'unknown path, duplicate key': 'Já existe uma entrada com isto. Escolha outro.',
  'unknown path, any reason': 'Este valor não é aceito. Confira o campo.',
  'class level slots':
    'Espaços de magia: de 0 a 9 por nível de magia, e 0 antes de a conjuração começar. Pacto: espaços de um nível só.',
  'class features limit':
    'Esta classe tem 41. Uma classe da mesa não pode ter mais de 40 características.',
  'class features limit, no count': 'Uma classe da mesa não pode ter mais de 60 características.',
  'subclass features limit':
    'Esta subclasse tem 41. Uma subclasse da mesa não pode ter mais de 40 características.',
  'subclass features limit, defaults':
    'Uma subclasse da mesa não pode ter mais de 60 características.',
  'class feature other reason': 'Confira esta característica.',
  'feat prerequisite': 'A pontuação mínima vai de 1 a 30.',
  'spell damage dice': 'Escreva o dado assim: 2d8 (de 1 a 20 dados).',
};

describe('the sentence of a refusal, by the shape of its field and its reason', () => {
  it.each(Object.keys(CASES))('%s', (name) => {
    const [field, reason, ctx] = CASES[name];
    const text = violationText({ field, reason }, ctx ?? CTX);
    expect(text).toBe(EXPECTED[name]);
  });
});
