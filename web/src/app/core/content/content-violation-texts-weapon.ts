/** What the server's reason at a field of a table weapon says in Portuguese (#580): the paths and reasons of
 * backend/internal/rules/overlay_weapon.go. The name's reasons are worded for every kind by `violationText`. */
export const WEAPON_FIELD_TEXTS: readonly { shape: RegExp; reason?: string; text: string }[] = [
  {
    shape: /^table_weapon\.damage$/,
    text: 'Use dados de d4, d6, d8, d10 ou d12, como 1d8 ou 2d6.',
  },
  {
    shape: /^table_weapon\.damage_type$/,
    text: 'Escolha um tipo de dano da lista.',
  },
  {
    shape: /^table_weapon\.two_handed_damage$/,
    text: 'O Versátil pede o dano com duas mãos, um dado como 1d10.',
  },
  {
    shape: /^table_weapon\.weight_oz$/,
    text: 'O peso e o preço não são negativos. O peso vai até 10.000 lb.',
  },
  {
    shape: /^table_weapon\.price_cp$/,
    text: 'O peso e o preço não são negativos. O preço vai até 1.000.000 PO.',
  },
  {
    shape: /^table_weapon\.properties$/,
    reason: 'limit',
    text: 'Uma arma tem no máximo 10 propriedades.',
  },
  {
    shape: /^table_weapon\.properties$/,
    text: 'Estas propriedades não vão juntas. Tire uma delas.',
  },
  {
    shape: /^table_weapon\.properties\[\]$/,
    reason: 'dangling_reference',
    text: 'Esta propriedade não existe. Tire-a.',
  },
  {
    shape: /^table_weapon\.properties\[\]$/,
    text: 'Esta propriedade está repetida. Tire uma.',
  },
  {
    shape: /^table_weapon\.normal_range_ft$/,
    text: 'Arremesso e uma arma à distância pedem a distância normal e a longa.',
  },
  {
    shape: /^table_weapon\.long_range_ft$/,
    text: 'A distância longa não pode ser menor que a normal.',
  },
];
