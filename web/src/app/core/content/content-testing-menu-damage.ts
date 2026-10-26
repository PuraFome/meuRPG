import { create } from '@bufbuild/protobuf';

import {
  EffectMenuListSchema,
  EffectMenuTypeSchema,
} from '../../../gen/meurpg/rules/v1/table_content_pb';
import { EffectMenuVm } from './effect-draft';
import { menuResponse } from './content-testing-menu';

/** The menus of the damage effects (extra_damage with dice, spell_modifier), for their own specs. */

const damageTypeList = () =>
  create(EffectMenuListSchema, {
    name: 'damage_types',
    values: [
      { key: 'damage-type:cold', namePt: 'Frio' },
      { key: 'damage-type:fire', namePt: 'Fogo' },
    ],
  });

/** The menu with the real one's extra_damage (a fixed part and dice, the type, the weapon kind, how often), for its own specs. */
export function extraDamageMenu(): EffectMenuVm {
  const res = menuResponse();
  res.types.push(
    create(EffectMenuTypeSchema, {
      type: 'extra_damage',
      namePt: 'Dano extra',
      hintPt: 'Dano somado a um ataque com arma que acertou.',
      fields: [
        { name: 'applies', kind: 'choice', required: true, list: 'extra_damage_applies' },
        { name: 'value', kind: 'formula', alwaysShown: true },
        { name: 'dice', kind: 'formula', alwaysShown: true },
        { name: 'dice_sides', kind: 'number', min: 4, max: 12, alwaysShown: true },
        { name: 'damage_type', kind: 'choice', list: 'damage_types', alwaysShown: true },
        { name: 'weapon', kind: 'choice', list: 'extra_damage_weapons', alwaysShown: true },
        { name: 'frequency', kind: 'choice', list: 'extra_damage_frequencies', alwaysShown: true },
        { name: 'no_crit_doubling', kind: 'yes_no' },
        { name: 'resource', kind: 'resource_ref' },
        { name: 'when', kind: 'condition' },
        { name: 'text_pt', kind: 'text' },
      ],
    }),
  );
  res.lists.push(
    create(EffectMenuListSchema, {
      name: 'extra_damage_applies',
      values: [{ key: 'any_target', namePt: 'Qualquer alvo' }],
    }),
    create(EffectMenuListSchema, {
      name: 'extra_damage_weapons',
      values: [
        { key: 'any', namePt: 'Qualquer ataque com arma' },
        { key: 'ranged', namePt: 'Ataque com arma à distância' },
      ],
    }),
    create(EffectMenuListSchema, {
      name: 'extra_damage_frequencies',
      values: [
        { key: 'once_per_turn', namePt: 'Uma vez por turno' },
        { key: 'each_hit', namePt: 'Em todo acerto' },
      ],
    }),
    damageTypeList(),
  );
  return new EffectMenuVm(res);
}

/** The menu with a spell_modifier ("Opção de magia"): the change decides which of the other fields are asked. */
export function spellModifierMenu(): EffectMenuVm {
  const res = menuResponse();
  res.types.push(
    create(EffectMenuTypeSchema, {
      type: 'spell_modifier',
      namePt: 'Opção de magia',
      hintPt: 'Uma opção que a ficha oferece ao conjurar.',
      fields: [
        { name: 'applies_to', kind: 'choice', required: true, list: 'spell_modifier_applies' },
        { name: 'spell_change', kind: 'choice', required: true, list: 'spell_changes' },
        { name: 'from', kind: 'choices', list: 'damage_types', alwaysShown: true },
        { name: 'value', kind: 'formula', alwaysShown: true },
        { name: 'damage_type', kind: 'choice', list: 'damage_types', alwaysShown: true },
        { name: 'count', kind: 'number', min: 1, max: 4, alwaysShown: true },
        { name: 'resource', kind: 'resource_ref' },
        { name: 'when', kind: 'condition' },
        { name: 'text_pt', kind: 'text' },
      ],
    }),
  );
  res.lists.push(
    create(EffectMenuListSchema, {
      name: 'spell_modifier_applies',
      values: [{ key: 'damage_spell', namePt: 'Magia que causa dano' }],
    }),
    create(EffectMenuListSchema, {
      name: 'spell_changes',
      values: [
        { key: 'damage_type_swap', namePt: 'Trocar o tipo de dano' },
        { key: 'bonus_damage', namePt: 'Dano extra' },
        { key: 'extra_dice_per_slot_over', namePt: 'Dados a mais por nível' },
      ],
    }),
    damageTypeList(),
  );
  return new EffectMenuVm(res);
}
