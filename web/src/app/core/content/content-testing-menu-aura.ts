import { create } from '@bufbuild/protobuf';

import { EffectMenuTypeSchema } from '../../../gen/meurpg/rules/v1/table_content_pb';
import { menuResponse } from './content-testing-menu';
import { EffectMenuVm } from './effect-draft';

/** The menu with the real one's aura type (backend/internal/rules/tablemenu.go): its own `aura` field, the effect of the block, and the usual `when` and `text_pt`. */
export function auraMenu(): EffectMenuVm {
  const res = menuResponse();
  res.types.push(
    create(EffectMenuTypeSchema, {
      type: 'aura',
      namePt: 'Aura',
      hintPt:
        'Um efeito em volta de quem leva a característica, em todo o combate enquanto estiver de pé e consciente.',
      fields: [
        { name: 'aura', kind: 'aura', required: true },
        { name: 'when', kind: 'condition' },
        { name: 'text_pt', kind: 'text' },
      ],
    }),
  );
  return new EffectMenuVm(res);
}
