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
};
