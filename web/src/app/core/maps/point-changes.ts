import type { MessageInitShape } from '@bufbuild/protobuf';

import type {
  LightSpecSchema,
  MapPointKind,
  TrapSpecSchema,
} from '../../../gen/meurpg/maps/v1/maps_pb';

/** What a point's editor saves in one call (`UpdateMapPoint`). */
export interface PointChanges {
  readonly kind?: MapPointKind;
  readonly name?: string;
  readonly description?: string;
  readonly xBp?: number;
  readonly yBp?: number;
  /** A SUBMAP point's target; `''` removes it. */
  readonly targetMapId?: string;
  readonly revealed?: boolean;
  /** "Ganchos e anotações" of a SCENE point (MR-029); `''` clears them. */
  readonly hooks?: string;
  /** "Mostrar a CD aos jogadores" of a SCENE point (RN-20). */
  readonly showDc?: boolean;
  /** A TRAP point's whole spec (E9-02): every field, an unspecified state keeps the current one. */
  readonly trap?: MessageInitShape<typeof TrapSpecSchema>;
  /** A LIGHT point's whole spec: a preset's key and radii, or the custom radii. */
  readonly light?: MessageInitShape<typeof LightSpecSchema>;
  /** A TREASURE's value in PO, 0 to 1.000.000. */
  readonly treasureValuePo?: number;
}
