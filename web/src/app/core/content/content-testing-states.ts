import { create } from '@bufbuild/protobuf';

import {
  EffectMenuListSchema,
  EffectMenuTypeSchema,
} from '../../../gen/meurpg/rules/v1/table_content_pb';
import { menuResponse } from './content-testing-menu';
import { EffectMenuVm } from './effect-draft';

/** The menu with the real one's state and weapon_rule effects, for the specs of their editors. */
export function stateMenu(): EffectMenuVm {
  const res = menuResponse();
  res.types.push(
    create(EffectMenuTypeSchema, {
      type: 'state',
      namePt: 'Estado',
      hintPt: 'Algo que o personagem liga por um tempo.',
      fields: [
        { name: 'state_name', kind: 'text', required: true },
        { name: 'duration_rounds', kind: 'formula', alwaysShown: true },
        { name: 'ends_on', kind: 'choices', list: 'state_ends', alwaysShown: true },
        { name: 'economy', kind: 'choice', required: true, list: 'state_economies' },
        { name: 'resource', kind: 'resource_ref' },
        { name: 'text_pt', kind: 'text' },
      ],
    }),
    create(EffectMenuTypeSchema, {
      type: 'weapon_rule',
      namePt: 'Regra de arma',
      hintPt: 'Muda as armas que o filtro escolhe.',
      fields: [
        { name: 'weapon', kind: 'choice', required: true, list: 'weapon_filters' },
        { name: 'weapon_name', kind: 'text' },
        { name: 'attack_bonus', kind: 'number', min: -5, max: 10, alwaysShown: true },
        { name: 'damage_bonus', kind: 'number', min: -5, max: 10, alwaysShown: true },
        { name: 'reach_ft', kind: 'number', min: 5, max: 60, alwaysShown: true },
        { name: 'while', kind: 'state_ref' },
      ],
    }),
  );
  res.lists.push(
    create(EffectMenuListSchema, {
      name: 'state_ends',
      values: [
        { key: 'dismiss', namePt: 'Quando o jogador encerra' },
        { key: 'long_rest', namePt: 'Descanso longo' },
      ],
    }),
    create(EffectMenuListSchema, {
      name: 'state_economies',
      values: [{ key: 'bonus_action', namePt: 'Ação bônus' }],
    }),
    create(EffectMenuListSchema, {
      name: 'weapon_filters',
      values: [{ key: 'melee', namePt: 'Corpo a corpo' }],
    }),
  );
  return new EffectMenuVm(res);
}
