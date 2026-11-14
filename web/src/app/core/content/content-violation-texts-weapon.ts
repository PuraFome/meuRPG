/** What the server's reason at a field of a table weapon says in Portuguese (#580): the paths and reasons of
 * backend/internal/rules/overlay_weapon.go. The name's reasons are worded for every kind by `violationText`. */
export const WEAPON_FIELD_TEXTS: readonly { shape: RegExp; reason?: string; text: string }[] = [
  {
    shape: /^table_weapon\.damage$/,
    text: 'Use dados de d4, d6, d8, d10, d12 ou d20, como 1d8 ou 2d6, ou um dano fixo de 1 a 99.',
  },
  {
    shape: /^table_weapon\.damage_type$/,
    text: 'Escolha um tipo de dano da lista.',
  },
  {
    shape: /^table_weapon\.two_handed_damage$/,
    text: 'O dano com duas mãos é um dado, como 1d10, e só vale para uma arma Versátil.',
  },
  { shape: /^table_weapon\.weight_oz$/, text: 'O peso vai de 0 a 10.000 lb.' },
  { shape: /^table_weapon\.price_cp$/, text: 'O preço vai de 0 a 1.000.000 PO.' },
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
    text: 'Uma arma à distância ou de Arremesso pede a distância normal, de 1 a 300 m. Uma corpo a corpo sem Arremesso não tem distância.',
  },
  {
    shape: /^table_weapon\.long_range_ft$/,
    text: 'A distância longa não pode ser menor que a normal, e vai até 300 m.',
  },
];
