import type { PointChanges } from '../../../core/maps/maps-client';
import { MapPointKind } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { HOOKS_MAX, textLength } from '../../../core/maps/scene-clues';

export const POINT_NAME_MAX = 80;
export const POINT_DESCRIPTION_MAX = 2000;

/** What the side panel edits, kept apart from the saved point until
 * "Salvar ponto". */
export interface PointDraft {
  readonly kind: MapPointKind;
  readonly name: string;
  readonly description: string;
  /** "Ganchos e anotações" (a SCENE point's private text). */
  readonly hooks: string;
  readonly revealed: boolean;
  /** A Submapa's target map ID, or `''`. */
  readonly targetMapId: string;
}

export interface DraftSource {
  readonly kind: MapPointKind;
  readonly name: string;
  readonly description: string;
  readonly hooks?: string;
  readonly revealed: boolean;
  readonly targetMap?: { readonly id: string } | undefined;
}

export function draftOf(point: DraftSource): PointDraft {
  return {
    kind: point.kind,
    name: point.name,
    description: point.description,
    hooks: point.hooks ?? '',
    revealed: point.revealed,
    targetMapId: point.kind === MapPointKind.SUBMAP ? (point.targetMap?.id ?? '') : '',
  };
}

/** What counts: only a Submapa has a target, and only a Cena has hooks. */
function effective(d: PointDraft): PointDraft {
  return {
    ...d,
    targetMapId: d.kind === MapPointKind.SUBMAP ? d.targetMapId : '',
    hooks: d.kind === MapPointKind.SCENE ? d.hooks : '',
  };
}

export function isDirty(draft: PointDraft, point: DraftSource): boolean {
  const a = effective(draft);
  const b = effective(draftOf(point));
  return (
    a.kind !== b.kind ||
    a.name !== b.name ||
    a.description !== b.description ||
    a.hooks !== b.hooks ||
    a.revealed !== b.revealed ||
    a.targetMapId !== b.targetMapId
  );
}

export interface DraftErrors {
  readonly name?: string;
  readonly description?: string;
  readonly hooks?: string;
}

/** The server's rules (maps.proto), checked first so the field can say
 * what is wrong; the server stays the authority. */
export function draftErrors(draft: PointDraft): DraftErrors {
  const name = draft.name.trim();
  const errors: { name?: string; description?: string; hooks?: string } = {};
  if (name === '') {
    errors.name = 'Dê um nome ao ponto.';
  } else if ([...name].length > POINT_NAME_MAX) {
    errors.name = `Use até ${POINT_NAME_MAX} caracteres.`;
  } else if (/\p{Cc}/u.test(name)) {
    errors.name = 'Use um nome numa linha só.';
  }
  if ([...draft.description].length > POINT_DESCRIPTION_MAX) {
    errors.description = 'Use até 2.000 caracteres.';
  }
  if (draft.kind === MapPointKind.SCENE && textLength(draft.hooks) > HOOKS_MAX) {
    errors.hooks = 'Use até 4.000 caracteres.';
  }
  return errors;
}

/** Only what changed, for `UpdateMapPoint`; `null` when nothing did. The
 * server trims the texts; a kind other than Submapa drops the target on its
 * own. */
export function changesOf(draft: PointDraft, point: DraftSource): PointChanges | null {
  const before = effective(draftOf(point));
  const now = effective(draft);
  const changes: {
    kind?: MapPointKind;
    name?: string;
    description?: string;
    hooks?: string;
    revealed?: boolean;
    targetMapId?: string;
  } = {};
  if (now.kind !== before.kind) {
    changes.kind = now.kind;
  }
  if (now.name.trim() !== before.name) {
    changes.name = now.name.trim();
  }
  if (now.description !== before.description) {
    changes.description = now.description;
  }
  // A kind other than Cena drops the hooks on its own (the server clears them).
  if (now.kind === MapPointKind.SCENE && now.hooks !== before.hooks) {
    changes.hooks = now.hooks;
  }
  if (now.revealed !== before.revealed) {
    changes.revealed = now.revealed;
  }
  // Only a Submapa has a target; another kind drops it on the server.
  if (now.kind === MapPointKind.SUBMAP && now.targetMapId !== before.targetMapId) {
    changes.targetMapId = now.targetMapId;
  }
  return Object.keys(changes).length > 0 ? changes : null;
}
