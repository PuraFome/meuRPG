import type { WritableSignal } from '@angular/core';

import { CharacterKind } from '../../../../gen/meurpg/characters/v1/characters_pb';
import type { Map as MapMessage, MapPoint, MapToken } from '../../../../gen/meurpg/maps/v1/maps_pb';
import type { LightPresets } from '../../../core/maps/light-presets';
import { tokenKey } from '../../../core/maps/map-state';
import type { RosterClient, RosterEntry } from '../../../core/maps/roster-client';
import type { CluePlayer } from '../../../core/maps/scene-clues';
import { BP_MAX } from '../../../shared/map-view/map-geometry';
import type { MapSelection } from '../../../shared/map-view/map-view';
import type { PickRow } from '../../../shared/person-pick/person-pick';
import type { Occupant } from './editor-painting';

/** The characters a player plays (an NPC has no player account). */
function playerCharacters(everyone: readonly RosterEntry[]): RosterEntry[] {
  return everyone.filter((c) => c.kind === CharacterKind.PLAYER && c.playerUserId !== '');
}

/** The player characters, to say who has each clue ("Todos", "Só Brisa"). */
export function cluePlayers(everyone: readonly RosterEntry[]): CluePlayer[] {
  return playerCharacters(everyone).map((c) => ({
    id: c.id,
    name: c.name,
    playerName: c.playerName ?? '',
  }));
}

/** Who found a treasure: the living player characters, with their class and player. */
export function finderRows(everyone: readonly RosterEntry[]): PickRow[] {
  return playerCharacters(everyone).map((c) => ({
    id: c.id,
    name: c.name,
    sub: [c.classSummary, c.playerName].filter(Boolean).join(' · '),
  }));
}

/** The characters that are not on the map yet, to add as tokens. */
export function notOnMap(
  everyone: readonly RosterEntry[],
  tokens: readonly MapToken[],
): RosterEntry[] {
  const onMap = new Set(tokens.map((t) => t.characterId));
  return everyone.filter((c) => !onMap.has(c.id));
}

/** Who stands on which square (the tokens), for the question before a door goes over them. */
export function occupantsOf(
  map: Pick<MapMessage, 'gridColumns' | 'gridRows'> | null,
  tokens: readonly MapToken[],
): Occupant[] {
  const cols = map?.gridColumns ?? 0;
  const rows = map?.gridRows ?? 0;
  if (cols === 0 || rows === 0) {
    return [];
  }
  return tokens.map((t) => ({
    col: Math.min(cols - 1, Math.floor((t.xBp / BP_MAX) * cols)),
    row: Math.min(rows - 1, Math.floor((t.yBp / BP_MAX) * rows)),
    name: t.name,
  }));
}

/** The point the selection names, if it names one. */
export function selectedPointOf(
  points: readonly MapPoint[],
  selection: MapSelection | null,
): MapPoint | null {
  return selection?.kind === 'point' ? (points.find((p) => p.id === selection.id) ?? null) : null;
}

/** The token the selection names, if it names one. */
export function selectedTokenOf(
  tokens: readonly MapToken[],
  selection: MapSelection | null,
): MapToken | null {
  return selection?.kind === 'token'
    ? (tokens.find((t) => tokenKey(t) === selection.id) ?? null)
    : null;
}

/** Where the roster and the light names land. */
export interface RosterSink {
  readonly everyone: WritableSignal<readonly RosterEntry[]>;
  /** The roster came back: before that (or if it failed), "nobody plays yet" would be a guess. */
  readonly rosterLoaded: WritableSignal<boolean>;
  readonly lightNames: WritableSignal<ReadonlyMap<string, string>>;
}

/** Reads the campaign's characters and the names of the light sources once the editor is drawn; a failure leaves them as they were. */
export function loadRoster(
  roster: Pick<RosterClient, 'list'>,
  lights: Pick<LightPresets, 'list'>,
  campaignId: string,
  sink: RosterSink,
): void {
  roster.list(campaignId).then(
    (list) => {
      sink.everyone.set(list);
      sink.rosterLoaded.set(true);
    },
    () => undefined,
  );
  lights.list(campaignId).then(
    (list) => sink.lightNames.set(new Map(list.map((o) => [o.key, o.name]))),
    () => undefined,
  );
}
