import type { TableContentViolation } from '../../../gen/meurpg/rules/v1/table_content_pb';

/**
 * What the server refused in a write, put back on the fields (MR-025, E10-01 state 5). A violation has the path of
 * the field in the request's body ("table_race.traits[1].effects[0].value") and a reason code; the app writes the
 * Portuguese from the code and the path, and never reads `message` (it is English, for logs). Every editor input
 * carries its own path as `data-field`, so a path finds its input with no table per editor; a path the screen draws
 * no input for goes to its nearest ancestor that has one (an array item to the array), and a violation with no field,
 * or about another entry, goes to the top.
 */

export interface PlacedViolation {
  readonly field: string;
  readonly reason: string;
  /** The Portuguese text. */
  readonly text: string;
}

export interface Placement {
  /** By the path of the input that shows it. */
  readonly byField: ReadonlyMap<string, readonly PlacedViolation[]>;
  /** For the summary at the top: no field, or another entry. */
  readonly top: readonly PlacedViolation[];
  /** The inputs that have a message, in the order the server listed them. */
  readonly fields: readonly string[];
  /** How many things need an adjustment (the summary counts these). */
  readonly count: number;
}

export const NO_PLACEMENT: Placement = { byField: new Map(), top: [], fields: [], count: 0 };

/** "table_race.traits[1].effects[0].value" without its indexes: "table_race.traits[].effects[].value". */
export function shapeOf(path: string): string {
  return path.replace(/\[\d+\]/g, '[]');
}

/** The path one step up: "a.b[2].c" → "a.b[2]" → "a.b" → "a" → "". */
export function parentOf(path: string): string {
  if (path.endsWith(']')) {
    return path.slice(0, path.lastIndexOf('['));
  }
  const dot = path.lastIndexOf('.');
  return dot < 0 ? '' : path.slice(0, dot);
}

/** The path of the nearest input that exists at or above `field`, or `''` when none does. */
export function inputFor(field: string, isKnown: (path: string) => boolean): string {
  let path = field;
  while (path !== '') {
    if (isKnown(path)) {
      return path;
    }
    path = parentOf(path);
  }
  return '';
}

/** What the editor of a kind says in the sentences ("uma magia da mesa"). */
export interface ViolationContext {
  /** "uma magia", "uma raça", "um antecedente". */
  readonly aOne: string;
  /** The name of the entry a `key` points to, for "Sobre Corujeiro: …". */
  readonly nameOf?: (key: string) => string;
  /** The most features a class or a subclass has (the menu's), for "Uma classe da mesa não pode ter mais de 60 características." */
  readonly maxFeatures?: number;
  /** How many features the entry being saved has: "Esta classe tem 61." */
  readonly featureCount?: number;
  /** Where a violation lands when it is not the input of its own path: "too many features" is the panel head's, not a row's. */
  readonly redirect?: (field: string, reason: string) => string;
}

const DICE_TEXT = 'Escreva o dado assim: 2d8 (de 1 a 20 dados).';
const FORMULA_TEXT = 'Esta fórmula não funciona. Use só as funções da lista e confira os parênteses.';
const LONG_TEXT = 'Este texto é longo demais. Encurte o parágrafo.';

/** What the server's reason at a field says in Portuguese, from the shape of the path (the indexes are `[]`). A row with no
 * reason answers any reason. The rows are ordered: the first that matches wins. The paths and reasons are the server's
 * (rules.TestEntryViolationPaths pins them). */
const FIELD_TEXTS: readonly { shape: RegExp; reason?: string; text: string }[] = [
  // A class and a subclass (E10-02): the paths and reasons of backend/internal/rules/overlay_class.go.
  { shape: /^table_class\.hit_die$/, text: 'O dado de vida é d6, d8, d10 ou d12.' },
  { shape: /^table_class\.saving_throws(\[\])?$/, text: 'Escolha dois testes de resistência diferentes.' },
  { shape: /^table_class\.skill_choose$/, text: 'Quantas perícias a classe dá: de 0 até o número de perícias da lista.' },
  { shape: /^table_class\.skill_from\[\]$/, text: 'Esta perícia não existe. Escolha da lista.' },
  { shape: /^table_class\.(multiclass_)?proficiencies\[\]$/, text: 'Escolha uma armadura, arma ou ferramenta da lista.' },
  { shape: /^table_class\.multiclass_skill_choose$/, text: 'As perícias de quem vem de outra classe saem da lista da classe: de 0 até o número dela.' },
  { shape: /^table_class\.(minimums|any_of)\.[a-z]+$/, text: 'O valor mínimo vai de 1 a 30.' },
  { shape: /^table_class\.(minimums|any_of)$/, text: 'O pré-requisito de multiclasse precisa de uma habilidade.' },
  { shape: /^table_class\.subclass_level$/, text: 'Esta classe não tem nível de subclasse entre 1 e 20.' },
  { shape: /^table_class\.asi_levels\[\]$/, text: 'Os níveis de incremento no valor de habilidade são diferentes, de 1 a 20.' },
  { shape: /^table_class\.levels$/, text: 'A tabela dos níveis precisa ter os 20 níveis.' },
  { shape: /^table_subclass\.levels$/, text: 'Uma subclasse que conjura precisa de uma linha em cada nível, do começo da conjuração ao 20.' },
  { shape: /^table_(class|subclass)\.levels\[\]\.level$/, text: 'Os níveis da subclasse vão de 1 a 20, em ordem.' },
  { shape: /^table_class\.levels\[\]\.prof_bonus$/, text: 'O bônus de proficiência vai de +1 a +12 (vazio é o do SRD).' },
  { shape: /^table_(class|subclass)\.levels\[\]\.cantrips_known$/, text: 'Truques: de 0 a 30, e 0 antes de a conjuração começar.' },
  { shape: /^table_(class|subclass)\.levels\[\]\.spells_known$/, text: 'Magias conhecidas: de 0 a 200, e 0 antes de a conjuração começar.' },
  { shape: /^table_(class|subclass)\.levels\[\]\.slots\[\]$/, text: 'Espaços de magia: de 0 a 9 por nível de magia, e 0 antes de a conjuração começar. Pacto: espaços de um nível só.' },
  { shape: /^table_(class|subclass)\.levels\[\]\.slots$/, text: 'Quem conjura precisa de espaços de magia neste nível.' },
  { shape: /^table_class\.levels\[\]\.features\[\]$/, reason: 'limit', text: 'LIMIT_CLASS' },
  { shape: /^table_subclass\.levels\[\]\.features\[\]$/, reason: 'limit', text: 'LIMIT_SUBCLASS' },
  { shape: /^table_subclass\.levels\[\]\.features$/, text: 'Uma subclasse só ganha características a partir do nível em que se escolhe.' },
  { shape: /^table_(class|subclass)\.casting\.kind$/, text: 'Escolha como conjura.' },
  { shape: /^table_(class|subclass)\.casting\.ability$/, text: 'Escolha a habilidade de conjuração.' },
  { shape: /^table_(class|subclass)\.casting\.preparation$/, text: 'Escolha se as magias são preparadas ou conhecidas.' },
  { shape: /^table_(class|subclass)\.casting\.prepared_max$/, text: 'Só quem prepara magias tem este número.' },
  { shape: /^table_(class|subclass)\.casting\.list_from$/, reason: 'dangling_reference', text: 'Esta classe não existe mais. Escolha outra lista.' },
  { shape: /^table_(class|subclass)\.casting\.list_from$/, text: 'Escolha uma classe que tenha lista de magias própria (uma subclasse que conjura precisa de uma).' },
  { shape: /^table_(class|subclass)\.casting\.start_level$/, text: 'A conjuração começa num nível de 1 a 20, e uma subclasse não conjura antes de ser escolhida.' },
  { shape: /^table_(class|subclass)\.casting$/, text: 'Uma subclasse só conjura se a classe dela não conjura. Uma classe sem conjuração não tem lista de magias.' },
  { shape: /^table_subclass\.class_key$/, text: 'A classe desta subclasse não existe mais.' },
  { shape: /^table_subclass\.level$/, text: 'A subclasse é escolhida no nível da classe.' },
  { shape: /^table_subclass\.always_prepared$/, text: 'Magias sempre preparadas pedem uma classe ou subclasse que conjura.' },
  { shape: /^table_subclass\.always_prepared\[\]\.class_level$/, text: 'O nível da classe vai de 1 a 20.' },
  { shape: /^table_subclass\.always_prepared\[\]\.spell_key$/, reason: 'dangling_reference', text: 'Esta magia não existe mais. Escolha outra.' },
  { shape: /^table_subclass\.always_prepared\[\]\.spell_key$/, text: 'Uma magia sempre preparada é de 1º nível ou mais, nunca um truque.' },
  { shape: /^table_(class|subclass)\.levels\[\]\.features\[\]$/, text: 'Confira esta característica.' },
  { shape: /\.level$/, text: 'Escolha um nível de magia de 0 (truque) a 9.' },
  { shape: /\.school_key$/, text: 'Escolha a escola da magia.' },
  { shape: /\.range\.kind$/, text: 'Esse alcance não combina com o alvo. “Só quem conjura” pede Pessoal; criaturas escolhidas pedem distância ou Toque.' },
  { shape: /\.range\.distance_ft$/, text: 'O alcance vai de 1,5 m a 1.584 m, em passos de 1,5 m.' },
  { shape: /\.target\.kind$/, text: 'Este alvo não aceita os outros campos preenchidos. Confira o alvo.' },
  { shape: /\.target\.shape$/, text: 'Escolha a forma da área: cone, cubo, cilindro, linha ou esfera.' },
  { shape: /\.target\.size_ft$/, text: 'A área vai de 1,5 m a 90 m, em passos de 1,5 m.' },
  { shape: /\.target\.(count|per_slot_level)$/, text: 'Escreva um número de criaturas: 2 ou mais, e até 10 a mais por nível de espaço.' },
  { shape: /\.casting_time\.unit$/, text: 'Escolha o tempo de conjuração.' },
  { shape: /\.casting_time\.amount$/, text: 'O tempo vai de 1 a 60 minutos ou horas; uma ação vale 1.' },
  { shape: /\.casting_time\.trigger_pt$/, reason: 'bad_text', text: 'O gatilho é uma linha só, de até 200 letras.' },
  { shape: /\.casting_time\.trigger_pt$/, text: 'Só uma reação tem gatilho.' },
  { shape: /\.duration\.kind$/, text: 'Escolha a duração.' },
  { shape: /\.duration\.amount$/, text: 'A duração vai de 1 a 999.' },
  { shape: /\.duration\.unit$/, text: 'Escolha a unidade da duração.' },
  { shape: /\.concentration$/, text: 'Concentração pede uma duração por tempo.' },
  { shape: /\.ritual$/, text: 'Um truque não é um ritual.' },
  { shape: /\.components\.material_pt$/, reason: 'bad_text', text: LONG_TEXT },
  { shape: /\.components\.material_pt$/, text: 'O material vai com o componente M, e só com ele.' },
  { shape: /\.attack$/, text: 'Uma magia de área não tem ataque. Escolha um teste de resistência, ou outro alvo.' },
  { shape: /\.save$/, text: 'Uma magia tem ataque ou teste de resistência, não os dois.' },
  { shape: /\.save\.ability$/, text: 'Escolha a habilidade do teste de resistência.' },
  { shape: /\.save\.on_success$/, text: 'Escolha o que acontece ao passar: metade do dano (que pede dano) ou nada.' },
  { shape: /\.damage$/, reason: 'limit', text: 'Uma magia tem até 4 tipos de dano.' },
  { shape: /\.damage\[\]\.damage_type_key$/, text: 'Escolha um tipo de dano da lista.' },
  { shape: /\.(damage\[\]\.(dice|per_slot_level|per_tier)|heal\.(dice|per_slot_level))$/, text: DICE_TEXT },
  { shape: /\.heal$/, text: 'Um truque não cura.' },
  { shape: /\.(desc_pt|higher_level_pt)\[\]$/, text: LONG_TEXT },
  { shape: /\.(desc_pt|higher_level_pt)$/, text: 'O texto tem parágrafos demais.' },
  { shape: /\.class_keys\[\]$/, reason: 'dangling_reference', text: 'Esta classe não existe mais. Tire-a da lista.' },
  { shape: /\.class_keys\[\]$/, text: 'Esta classe está na lista duas vezes.' },
  { shape: /\.size$/, text: 'Escolha o tamanho.' },
  { shape: /\.speed_ft$/, text: 'O deslocamento vai de 1,5 m a 36 m, em passos de 1,5 m.' },
  { shape: /\.darkvision_ft$/, text: 'A visão no escuro é 0 ou vai de 1,5 m a 36 m, em passos de 1,5 m.' },
  { shape: /\.ability_bonuses\.[a-z]+$/, text: 'O bônus vai de −4 a +4.' },
  { shape: /\.choice_bonuses$/, text: 'No máximo 6 bônus à escolha.' },
  { shape: /\.choice_bonuses\[\]$/, text: 'Cada bônus à escolha vai de +1 a +4.' },
  { shape: /\.language_choices$/, text: 'Idiomas à escolha: de 0 a 4.' },
  { shape: /\.languages\[\]$/, text: 'Este idioma não existe. Escolha da lista.' },
  { shape: /\.race_key$/, text: 'A raça desta sub-raça não existe mais. Crie a sub-raça numa raça que exista.' },
  { shape: /\.skills$/, text: 'Escolha duas perícias diferentes.' },
  { shape: /\.skills\[\]$/, reason: 'dangling_reference', text: 'Esta perícia não existe. Escolha da lista.' },
  { shape: /\.skills\[\]$/, text: 'Escolha duas perícias diferentes.' },
  { shape: /\.tools$/, text: 'No máximo 4 ferramentas.' },
  { shape: /\.tools\[\]$/, text: 'Esta não é uma ferramenta do SRD. Escolha da lista.' },
  { shape: /\.equipment_pt$/, text: 'O equipamento é longo demais. Encurte o texto.' },
];

/** The Portuguese of one violation, by the shape of its path and its reason. */
export function violationText(v: Pick<TableContentViolation, 'field' | 'reason'>, ctx: ViolationContext): string {
  const shape = shapeOf(v.field);
  const leaf = shape.slice(shape.lastIndexOf('.') + 1);
  const r = v.reason;
  if (shape === '') {
    if (r === 'limit') {
      return 'A mesa já tem 300 entradas. Arquive ou use o que já existe.';
    }
    if (r === 'size_limit') {
      return 'Esta entrada passou de 64 KiB de dados. Tire um pouco de texto.';
    }
    if (r === 'overlay') {
      return 'O conteúdo da mesa não fecha com esta entrada. Confira o que ela usa.';
    }
    return 'O servidor recusou esta entrada. Confira os campos.';
  }
  if (leaf === 'name_pt') {
    if (r === 'duplicate_name') {
      return `Já existe ${ctx.aOne} da mesa com este nome. Escolha outro.`;
    }
    if (r === 'bad_name' || r === 'bad_text') {
      return 'O nome tem de 1 a 60 letras.';
    }
    if (r === 'bad_key' || r === 'reserved_key' || r === 'duplicate_key') {
      return 'Este nome não pode ser usado. Escolha outro.';
    }
  }
  if (r === 'immutable') {
    return leaf === 'level'
      ? 'Um truque não vira magia de 1º nível ou mais, nem o contrário. Crie outra.'
      : 'Isto não muda depois de criado.';
  }
  if (r === 'size_limit') {
    return 'Esta entrada passou de 64 KiB de dados. Tire um pouco de texto.';
  }
  if (r === 'bad_formula') {
    return FORMULA_TEXT;
  }
  if (r === 'forbidden_effect') {
    return 'Este efeito não está no menu.';
  }
  if (shape.endsWith('.effects[].resource') && r === 'bad_value') {
    return 'Use só letras minúsculas sem acento, números e _ (de 1 a 40), como vigilia.';
  }
  const row = FIELD_TEXTS.find((f) => f.shape.test(shape) && (f.reason === undefined || f.reason === r));
  if (row) {
    if (row.text === 'LIMIT_CLASS' || row.text === 'LIMIT_SUBCLASS') {
      const max = ctx.maxFeatures ?? 60;
      const one = row.text === 'LIMIT_CLASS' ? 'classe' : 'subclasse';
      const has = ctx.featureCount !== undefined ? `Esta ${one} tem ${ctx.featureCount}. ` : '';
      return `${has}Uma ${one} da mesa não pode ter mais de ${max} características.`;
    }
    return row.text;
  }
  if (r === 'dangling_reference') {
    return 'Isto aponta para algo que não existe mais. Escolha de novo.';
  }
  if (shape.includes('.effects[].')) {
    return r === 'bad_value' ? 'Este valor não serve para este efeito.' : 'Confira este campo do efeito.';
  }
  if (r === 'bad_text') {
    return LONG_TEXT;
  }
  if (r === 'limit') {
    return 'Isto passou do limite.';
  }
  if (r === 'duplicate_name' || r === 'duplicate_key') {
    return 'Já existe uma entrada com isto. Escolha outro.';
  }
  return 'Este valor não é aceito. Confira o campo.';
}

/** Puts every violation on its input: `isKnown` says which paths the editor draws an input for right now. */
export function placeViolations(
  violations: readonly TableContentViolation[],
  isKnown: (path: string) => boolean,
  ctx: ViolationContext,
): Placement {
  const byField = new Map<string, PlacedViolation[]>();
  const top: PlacedViolation[] = [];
  const fields: string[] = [];
  for (const v of violations) {
    const text = violationText(v, ctx);
    const field = v.field;
    if (v.key !== '') {
      // About another entry: it never marks a field of this one.
      const who = ctx.nameOf?.(v.key) ?? '';
      top.push({ field: '', reason: v.reason, text: who ? `Sobre ${who}: ${text}` : text });
      continue;
    }
    const at = field === '' ? '' : inputFor(ctx.redirect?.(field, v.reason) ?? field, isKnown);
    const placed: PlacedViolation = { field: at, reason: v.reason, text };
    if (at === '') {
      top.push(placed);
      continue;
    }
    const list = byField.get(at) ?? [];
    // The same sentence twice on one input says nothing new.
    if (!list.some((p) => p.text === text)) {
      list.push(placed);
    }
    byField.set(at, list);
    if (!fields.includes(at)) {
      fields.push(at);
    }
  }
  return { byField, top, fields, count: fields.length + top.length };
}

/** "Não foi possível salvar a magia. 3 campos precisam de ajuste. Nada foi salvo; o que você digitou continua aqui." */
export function refusalSummary(what: string, count: number): string {
  const n = count === 1 ? '1 campo precisa' : `${count} campos precisam`;
  return `${n} de ajuste. Nada foi salvo; o que você digitou continua aqui.`.replace(/^/, `Não foi possível salvar ${what}. `);
}
