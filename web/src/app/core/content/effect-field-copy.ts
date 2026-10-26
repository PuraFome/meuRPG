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
  while: 'Só enquanto o estado',
  state_name: 'Nome do estado',
  duration_rounds: 'Duração (rodadas)',
  ends_on: 'Também acaba',
  weapon: 'Armas',
  weapon_name: 'Nome da arma',
  attack_ability: 'Habilidade do ataque',
  damage_ability: 'Habilidade do dano',
  attack_bonus: 'Bônus no ataque',
  damage_bonus: 'Bônus no dano',
  reach_ft: 'Alcance corpo a corpo',
  attack_name: 'Nome do ataque',
  damage_dice: 'Dados de dano',
  damage_type: 'Tipo de dano',
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
};

/** One line under a field, where the name alone does not say. This is screen copy; the formula functions and the tag
 * prefixes it mentions come from the server's menu (`EffectMenuVm.formulaHint`, `tagHint`). */
export const EFFECT_FIELD_HINTS: Readonly<Record<string, string>> = {
  resource: 'Só letras minúsculas sem acento, números e _ (de 1 a 40).',
  state_name:
    'Só letras minúsculas sem acento, números e _ (de 1 a 40). Os outros efeitos da entrada usam este nome em "Só enquanto o estado" e em active("nome").',
  duration_rounds:
    'Um número ou uma fórmula, em rodadas (1 minuto são 10). Vazio ou 0: dura até acabar.',
  weapon_name:
    'O nome da arma em português, como na ficha (sem diferença de maiúsculas e acentos).',
  attack_bonus: 'De -5 a 10, somado à jogada de ataque.',
  damage_bonus: 'De -5 a 10, somado ao dano.',
  reach_ft: 'Em metros, de 1,5 em 1,5; só para armas de corpo a corpo.',
  attack_name: 'O nome do ataque na lista de ataques da ficha.',
  damage_dice: 'Dados simples, como 1d6 ou 2d8 (d4, d6, d8, d10 ou d12; até 10 dados).',
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
  counts_as_known:
    'A magia aparece também entre as conhecidas da ficha, sem gastar nenhuma escolha da classe.',
};

/** The label or hint of a field where one type reads it differently from the rest (`ability` is a maneuver's modifier, and a granted spell's casting ability). */
export const EFFECT_TYPE_FIELD_LABELS: Readonly<Record<string, Readonly<Record<string, string>>>> =
  {
    granted_spell: { ability: 'Habilidade de conjuração' },
    // A state's resource is spent once at each start; a pseudo-weapon names its kind, its ability and its reach in the words of an attack.
    state: { resource: 'Gasta um uso de' },
    pseudo_weapon: {
      weapon: 'Tipo de ataque',
      ability: 'Habilidade do ataque',
      reach_ft: 'Alcance de contato',
      range_ft: 'Alcance (à distância)',
    },
  };

export const EFFECT_TYPE_FIELD_HINTS: Readonly<Record<string, Readonly<Record<string, string>>>> = {
  granted_spell: {
    ability:
      'Dá o bônus de ataque da magia (proficiência + modificador) e a CD de resistência (8 + o mesmo).',
  },
};
