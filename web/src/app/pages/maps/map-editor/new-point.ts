import type { MessageInitShape } from '@bufbuild/protobuf';

import { MapPointKind, type TrapSpecSchema } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { TrapTargets, TrapTrigger } from '../../../../gen/meurpg/rules/v1/rules_pb';
import type { LightPresets } from '../../../core/maps/light-presets';
import type { TrapPresets } from '../../../core/traps/trap-presets';

/** The name a new point of this kind starts with (the master renames it at once). */
export function defaultName(kind: MapPointKind): string {
  switch (kind) {
    case MapPointKind.BATTLE:
      return 'Nova batalha';
    case MapPointKind.SUBMAP:
      return 'Novo submapa';
    case MapPointKind.LIGHT:
      return 'Nova luz';
    case MapPointKind.TRAP:
      return 'Nova armadilha';
    case MapPointKind.TREASURE:
      return 'Novo tesouro';
    default:
      return 'Nova cena';
  }
}

/** The DC to find a new trap when the server lists no sample trap to copy it from. */
const FALLBACK_FIND_DC = 10;

/** The sample traps and light sources a new point of the kinds that have one starts from. */
export interface PointPresets {
  readonly lights: Pick<LightPresets, 'list'>;
  readonly traps: Pick<TrapPresets, 'list'>;
}

/** A new light starts as the first source the server lists (the server fills the radii of a preset sent with none). */
async function newLight(presets: PointPresets, campaignId: string): Promise<{ presetKey: string }> {
  const first = (await presets.lights.list(campaignId))[0];
  return { presetKey: first?.key ?? '' };
}

/** A new trap starts with the numbers of the first sample trap the server lists, with no effect yet: the form edits the rest. */
async function newTrap(
  presets: PointPresets,
  campaignId: string,
): Promise<MessageInitShape<typeof TrapSpecSchema>> {
  const first = (await presets.traps.list(campaignId)).presets[0];
  return {
    findDc: first?.findDc ?? FALLBACK_FIND_DC,
    areaSize: 1,
    trigger: TrapTrigger.ENTER,
    effect: { targets: TrapTargets.AREA },
  };
}

/** What `CreateMapPoint` takes for a new point of `kind` at `at`: the kinds with a spec of their own carry their starting one. */
export async function newPointBody(
  kind: MapPointKind,
  at: { xBp: number; yBp: number },
  campaignId: string,
  presets: PointPresets,
) {
  return {
    kind,
    name: defaultName(kind),
    description: '',
    xBp: at.xBp,
    yBp: at.yBp,
    ...(kind === MapPointKind.LIGHT ? { light: await newLight(presets, campaignId) } : {}),
    ...(kind === MapPointKind.TRAP ? { trap: await newTrap(presets, campaignId) } : {}),
    ...(kind === MapPointKind.TREASURE ? { treasureValuePo: 0 } : {}),
  };
}
