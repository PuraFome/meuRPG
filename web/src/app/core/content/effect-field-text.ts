/**
 * The screen copy of the effect fields: the label and the one-line hint of each field of the menu, by its proto name. Nothing
 * here lists a value (the lists come from the server's menu); the words are ours.
 */

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
  except: 'Todos os tipos de dano, menos',
  while: 'Enquanto',
  kind: 'O que acontece',
  damage_types: 'Tipos de dano',
  conditions: 'Condições',
  against_condition: 'Só nos testes de resistência contra a condição',
  only_proficient: 'Só onde já tem proficiência',
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
  damage_types:
    'Escolha os tipos, ou "Todos os tipos de dano" sozinho. Use este campo ou "Todos os tipos de dano, menos".',
  except:
    'Em vez de listar os tipos: vale para todos menos este. Deixe vazio se escolheu os tipos acima.',
  while: 'Vazio, vale sempre. Em fúria, só enquanto o personagem estiver em fúria.',
  only_proficient:
    'Dobra a perícia só de quem já é proficiente nela por outra fonte. Quem não é, fica como está.',
  against_condition:
    'Vazio, a vantagem ou desvantagem fica como lembrete da situação. Com uma condição, o app aplica nos testes de resistência para não ficar com ela.',
};
