import { create } from '@bufbuild/protobuf';

import { TableContentViolationSchema } from '../../../gen/meurpg/rules/v1/table_content_pb';
import { violationText } from './content-violations';

const v = (field: string, reason: string) =>
  create(TableContentViolationSchema, { field, reason, key: '' });
const CTX = { aOne: 'uma arma' };

describe('the refusals of a table weapon in words (#580)', () => {
  it.each([
    ['table_weapon.name_pt', 'bad_name', 'O nome tem de 1 a 60 letras.'],
    [
      'table_weapon.name_pt',
      'duplicate_name',
      'Já existe uma arma da mesa com este nome. Escolha outro.',
    ],
    [
      'table_weapon.damage',
      'bad_value',
      'Use dados de d4, d6, d8, d10, d12 ou d20, como 1d8 ou 2d6, ou um dano fixo de 1 a 99.',
    ],
    ['table_weapon.damage_type', 'dangling_reference', 'Escolha um tipo de dano da lista.'],
    [
      'table_weapon.two_handed_damage',
      'bad_value',
      'O dano com duas mãos é um dado, como 1d10, e só vale para uma arma Versátil.',
    ],
    ['table_weapon.weight_oz', 'bad_value', 'O peso vai de 0 a 10.000 lb.'],
    ['table_weapon.price_cp', 'bad_value', 'O preço vai de 0 a 1.000.000 PO.'],
    ['table_weapon.properties', 'limit', 'Uma arma tem no máximo 10 propriedades.'],
    ['table_weapon.properties[2]', 'dangling_reference', 'Esta propriedade não existe. Tire-a.'],
    ['table_weapon.properties[2]', 'bad_value', 'Esta propriedade está repetida. Tire uma.'],
    ['table_weapon.properties', 'bad_value', 'Estas propriedades não vão juntas. Tire uma delas.'],
    [
      'table_weapon.normal_range_ft',
      'bad_value',
      'Uma arma à distância ou de Arremesso pede a distância normal, de 1 a 300 m. Uma corpo a corpo sem Arremesso não tem distância.',
    ],
    [
      'table_weapon.long_range_ft',
      'bad_value',
      'A distância longa não pode ser menor que a normal, e vai até 300 m.',
    ],
  ])('%s (%s)', (field, reason, text) => {
    expect(violationText(v(field, reason), CTX)).toBe(text);
  });
});
