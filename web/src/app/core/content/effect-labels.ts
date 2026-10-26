/** The words of the effect editor: the label of each field (a type may word one its own way) and the line under some. */

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
  ignore_cover: 'Ignora meia cobertura e três quartos',
  range_multiplier: 'Multiplica o alcance da magia por',
  no_long_range_disadvantage: 'Sem desvantagem no alcance longo',
  dice: 'Dados (quantos)',
  dice_sides: 'Lados do dado',
  damage_type: 'Tipo de dano',
  weapon: 'Arma',
  frequency: 'Quantas vezes',
  no_crit_doubling: 'Não dobra no acerto crítico',
  applies_to: 'Vale para',
  spell_change: 'O que muda na magia',
  only_proficient: 'Só onde já tem proficiência',
};

/** A label that reads differently in one type: a spell_modifier's `from` lists the damage types it swaps, its `count` is dice. */
const TYPE_FIELD_LABELS: Readonly<Record<string, Readonly<Record<string, string>>>> = {
  extra_damage: { value: 'Dano fixo' },
  spell_modifier: {
    from: 'Tipos de dano que troca',
    value: 'Dano fixo somado',
    count: 'Dados a mais por nível acima',
    damage_type: 'Tipo do dano somado',
    resource: 'Gasta um uso de',
  },
};

/** The label of a field of a type: the type's own wording when it has one. */
export function effectFieldLabel(type: string, name: string): string {
  return TYPE_FIELD_LABELS[type]?.[name] ?? EFFECT_FIELD_LABELS[name] ?? name;
}

/** One line under a field, where the name alone does not say. This is screen copy; the formula functions and the tag
 * prefixes it mentions come from the server's menu (`EffectMenuVm.formulaHint`, `tagHint`). */
export const EFFECT_FIELD_HINTS: Readonly<Record<string, string>> = {
  resource: 'Só letras minúsculas sem acento, números e _ (de 1 a 40).',
  text_pt: 'O que a ficha mostra. {value} vira o valor com sinal.',
  ignore_cover: 'O alvo não ganha o bônus na CA. A cobertura total continua impedindo o ataque.',
  range_multiplier: 'Só para magias: 2 dobra o alcance, 3 triplica. Vazio, o alcance não muda.',
  no_long_range_disadvantage: 'Só para armas: até o alcance longo, a rolagem não tem desvantagem.',
  dice: 'Uma fórmula, como 1 ou prof(). Vale junto com os lados do dado.',
  only_proficient:
    'Dobra a perícia só de quem já é proficiente nela por outra fonte. Quem não é, fica como está.',
  dice_sides: 'De 4 a 12: d4, d6, d8, d10 ou d12.',
  no_crit_doubling: 'Por padrão os dados dobram no acerto crítico; o dano fixo nunca dobra.',
};
