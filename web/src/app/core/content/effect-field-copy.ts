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
  cost: 'Custo (usos)',
  cost_max: 'Custo máximo (o jogador escolhe)',
  value_per_point: 'Por ponto gasto a mais',
  heal: 'Cura (dados)',
  temp_hp: 'Pontos de vida temporários',
  event: 'Quando acontece',
  restore: 'Devolve usos de',
  heal_percent_max: 'Cura (% do máximo)',
  ignore_cover: 'Ignora meia cobertura e três quartos',
  range_multiplier: 'Multiplica o alcance da magia por',
  no_long_range_disadvantage: 'Sem desvantagem no alcance longo',
  spell: 'Magia',
  from_level: 'A partir do nível',
  uses: 'Cada conjuração gasta um uso de',
  also_with_slot: 'Também com espaço de magia',
  ritual_only: 'Só como ritual',
  counts_as_known: 'Conta como magia conhecida',
  only_proficient: 'Só onde já tem proficiência',
  dice: 'Dados (quantos)',
  dice_sides: 'Lados do dado',
  damage_type: 'Tipo de dano',
  weapon: 'Arma',
  frequency: 'Quantas vezes',
  no_crit_doubling: 'Não dobra no acerto crítico',
  applies_to: 'Vale para',
  spell_change: 'O que muda na magia',
};

/** One line under a field, where the name alone does not say. This is screen copy; the formula functions and the tag
 * prefixes it mentions come from the server's menu (`EffectMenuVm.formulaHint`, `tagHint`). */
export const EFFECT_FIELD_HINTS: Readonly<Record<string, string>> = {
  resource: 'Só letras minúsculas sem acento, números e _ (de 1 a 40).',
  cost: 'Quantos usos cada vez gasta; vazio é 1. Não passa do máximo do recurso.',
  cost_max:
    'Com ele, o jogador escolhe na hora quantos gastar, do custo até este valor (ou o que restar).',
  value_per_point:
    'Soma a cada ponto gasto acima do custo: dados na cura ("1d6") ou uma fórmula nos pontos temporários.',
  heal: 'Dados, como 1d6 ou 2d4+2. Nunca passa do máximo, e quem está morto não é curado.',
  temp_hp: 'Não se acumulam: vale o maior entre o que o alvo tinha e este valor.',
  restore:
    'O nome de um recurso desta entrada; devolve só o que foi gasto, então o mesmo evento duas vezes devolve uma vez.',
  heal_percent_max: 'De 1 a 100, arredondado para baixo, no lugar dos dados.',
  text_pt: 'O que a ficha mostra. {value} vira o valor com sinal.',
  ignore_cover: 'O alvo não ganha o bônus na CA. A cobertura total continua impedindo o ataque.',
  range_multiplier: 'Só para magias: 2 dobra o alcance, 3 triplica. Vazio, o alcance não muda.',
  no_long_range_disadvantage: 'Só para armas: até o alcance longo, a rolagem não tem desvantagem.',
  only_proficient:
    'Dobra a perícia só de quem já é proficiente nela por outra fonte. Quem não é, fica como está.',
  from_level:
    'Um número ou uma fórmula. Vazio: desde o 1º nível. Vale o nível da classe numa característica de classe ou de subclasse, o nível do personagem no resto.',
  uses: 'O nome de um recurso que esta entrada define (um efeito Recurso). Vazio: à vontade, sem limite.',
  also_with_slot: 'O personagem também pode conjurar com um espaço de magia, sem gastar usos.',
  ritual_only:
    'Só conjura como ritual (10 minutos a mais, sem espaço). Pede uma magia com a etiqueta de ritual.',
  dice: 'Uma fórmula, como 1 ou prof(). Vale junto com os lados do dado.',
  dice_sides: 'De 4 a 12: d4, d6, d8, d10 ou d12.',
  no_crit_doubling: 'Por padrão os dados dobram no acerto crítico; o dano fixo nunca dobra.',
  counts_as_known:
    'A magia aparece também entre as conhecidas da ficha, sem gastar nenhuma escolha da classe.',
};

/** The label or hint of a field where one type reads it differently from the rest (`ability` is a maneuver's modifier, and a granted spell's casting ability). */
export const EFFECT_TYPE_FIELD_LABELS: Readonly<Record<string, Readonly<Record<string, string>>>> =
  {
    granted_spell: { ability: 'Habilidade de conjuração' },
    extra_damage: { value: 'Dano fixo' },
    spell_modifier: {
      from: 'Tipos de dano que troca',
      value: 'Dano fixo somado',
      count: 'Dados a mais por nível acima',
      damage_type: 'Tipo do dano somado',
    },
  };

export const EFFECT_TYPE_FIELD_HINTS: Readonly<Record<string, Readonly<Record<string, string>>>> = {
  granted_spell: {
    ability:
      'Dá o bônus de ataque da magia (proficiência + modificador) e a CD de resistência (8 + o mesmo).',
  },
};
