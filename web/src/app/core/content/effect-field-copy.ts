/** The screen copy of the effect fields: the label and the one-line hint of each, in our words (the menu names the fields by their proto name). */

/** The labels of the effect fields, in our words (the menu names the fields by their proto name). */
export const EFFECT_FIELD_LABELS: Readonly<Record<string, string>> = {
  target: 'O que muda',
  mode: 'Como muda',
  value: 'Valor',
  when: 'Só quando',
  tags: 'Situação',
  proficiency: 'Proficiência em',
  level: 'Quanto',
  roll: 'Rolagem',
  targets: 'Em quê',
  sense: 'Sentido',
  range_ft: 'Alcance',
  resource: 'Nome do recurso',
  max: 'Usos (máximo)',
  recharge: 'Volta em',
  choice: 'O jogador escolhe',
  count: 'Quantos',
  from: 'Entre',
  economy: 'Tipo de ação',
  applies: 'Onde o dado entra',
  ability: 'Habilidade somada',
  spells: 'Magias',
  text_pt: 'Texto na ficha',
  except: 'Menos o dano de tipo',
  while: 'Enquanto',
  spell: 'Magia',
  from_level: 'A partir do nível',
  uses: 'Cada conjuração gasta um uso de',
  also_with_slot: 'Também com espaço de magia',
  ritual_only: 'Só como ritual',
  counts_as_known: 'Conta como magia conhecida',
  ignore_cover: 'Ignora meia cobertura e três quartos',
  range_multiplier: 'Multiplica o alcance da magia por',
  no_long_range_disadvantage: 'Sem desvantagem no alcance longo',
};

/** One line under a field, where the name alone does not say. This is screen copy; the formula functions and the tag
 * prefixes it mentions come from the server's menu (`EffectMenuVm.formulaHint`, `tagHint`). */
export const EFFECT_FIELD_HINTS: Readonly<Record<string, string>> = {
  resource: 'Só letras minúsculas sem acento, números e _ (de 1 a 40).',
  text_pt: 'O que a ficha mostra. {value} vira o valor com sinal.',
  ignore_cover: 'O alvo não ganha o bônus na CA. A cobertura total continua impedindo o ataque.',
  range_multiplier: 'Só para magias: 2 dobra o alcance, 3 triplica. Vazio, o alcance não muda.',
  no_long_range_disadvantage: 'Só para armas: até o alcance longo, a rolagem não tem desvantagem.',
  from_level:
    'Um número ou uma fórmula. Vazio: desde o 1º nível. Vale o nível da classe numa característica de classe ou de subclasse, o nível do personagem no resto.',
  uses: 'O nome de um recurso que esta entrada define (um efeito Recurso). Vazio: à vontade, sem limite.',
  also_with_slot: 'O personagem também pode conjurar com um espaço de magia, sem gastar usos.',
  ritual_only:
    'Só conjura como ritual (10 minutos a mais, sem espaço). Pede uma magia com a etiqueta de ritual.',
  counts_as_known:
    'A magia aparece também entre as conhecidas da ficha, sem gastar nenhuma escolha da classe.',
};

/** The label or hint of a field where one type reads it differently from the rest (`ability` is a maneuver's modifier, and a granted spell's casting ability). */
export const EFFECT_TYPE_FIELD_LABELS: Readonly<Record<string, Readonly<Record<string, string>>>> =
  {
    granted_spell: { ability: 'Habilidade de conjuração' },
  };

export const EFFECT_TYPE_FIELD_HINTS: Readonly<Record<string, Readonly<Record<string, string>>>> = {
  granted_spell: {
    ability:
      'Dá o bônus de ataque da magia (proficiência + modificador) e a CD de resistência (8 + o mesmo).',
  },
};
