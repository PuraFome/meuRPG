import { type MessageInitShape, create } from '@bufbuild/protobuf';

import {
  type GetRestPreviewResponse,
  GetRestPreviewResponseSchema,
  type RestPreview,
  RestPreviewSchema,
} from '../../../gen/meurpg/play/v1/resources_pb';

/** One character's preview of a rest for specs: nothing spent, 10 of 10 hit points. */
export function restPreview(
  over: MessageInitShape<typeof RestPreviewSchema> & { characterId: string; name: string },
): RestPreview {
  return create(RestPreviewSchema, {
    hitPointsCurrent: 10,
    hitPointsMax: 10,
    ...over,
  });
}

/** The answer of `GetRestPreview` for specs. */
export function restPreviewResponse(
  characters: readonly RestPreview[],
  longRestAlreadyTaken = false,
): GetRestPreviewResponse {
  return create(GetRestPreviewResponseSchema, {
    characters: [...characters],
    longRestAlreadyTaken,
  });
}

const RAGE_USES = 3;
const LAY_ON_HANDS_POINTS = 25;
const SORCERY_POINTS = 5;
const KI_POINTS = 5;
const INSPIRATIONS = 3;

/** The party of the board (PM-07b 9) for a long rest: Ragna's Fúria, Tavo's Cura pelas Mãos, Nael's sorcery points and
 * created slot, Kai's Chi, Orla's Inspiração de Bardo. */
export function boardParty(): GetRestPreviewResponse {
  const used = (key: string, namePt: string, total: number) => ({ key, namePt, spent: 1, total });
  return restPreviewResponse([
    restPreview({
      characterId: 'ragna',
      name: 'Ragna',
      resources: [used('rage', 'Fúria', RAGE_USES)],
    }),
    restPreview({
      characterId: 'tavo',
      name: 'Tavo',
      resources: [used('lay_on_hands', 'Cura pelas Mãos', LAY_ON_HANDS_POINTS)],
    }),
    restPreview({
      characterId: 'nael',
      name: 'Nael',
      resources: [used('sorcery_points', 'Pontos de Feitiçaria', SORCERY_POINTS)],
      createdSlotsLost: 1,
    }),
    restPreview({ characterId: 'kai', name: 'Kai', resources: [used('ki', 'Chi', KI_POINTS)] }),
    restPreview({
      characterId: 'orla',
      name: 'Orla',
      resources: [used('bardic_inspiration', 'Inspiração de Bardo', INSPIRATIONS)],
      spellSlotsBack: [{ level: 1, count: 2 }],
    }),
  ]);
}
