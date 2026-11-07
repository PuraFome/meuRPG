import type { GetMapResponse, GetMapVisionResponse } from '../../../gen/meurpg/maps/v1/maps_pb';
import {
  type GetTableScreenResponse,
  TableScreenKind,
  type TvMap,
  TvState,
} from '../../../gen/meurpg/play/v1/table_screen_pb';

/** What the TV draws of a map: the page feeds `map` and `vision` to the map views, or says the fog hides it. */
export interface TvMapVm {
  readonly name: string;
  readonly hiddenByFog: boolean;
  readonly map: GetMapResponse | null;
  readonly vision: GetMapVisionResponse | null;
  readonly revision: number;
}

export interface TvRollVm {
  readonly who: string;
  readonly expression: string;
  readonly total: number;
  readonly faces: readonly number[];
}

export interface TvCombatantVm {
  readonly name: string;
  readonly initial: string;
  /** "Ileso", "Ferido"…; `null` when the server tells nothing (a player character). */
  readonly stateWord: string | null;
  readonly onTurn: boolean;
  readonly isPlayer: boolean;
}

/** What the TV shows, as plain data: one case per kind of screen. */
export type TvStageVm =
  | { readonly kind: 'idle'; readonly party: readonly string[] }
  | { readonly kind: 'map'; readonly map: TvMapVm }
  | {
      readonly kind: 'image';
      readonly name: string;
      readonly url: string;
      readonly width: number;
      readonly height: number;
    }
  | {
      readonly kind: 'scene';
      readonly name: string;
      readonly description: string;
      readonly stage: readonly { readonly name: string; readonly speaking: boolean }[];
    }
  | {
      readonly kind: 'combat';
      readonly round: number;
      readonly order: readonly TvCombatantVm[];
      readonly mastersTurn: boolean;
      readonly log: readonly string[];
      /** `null`: the players do not see the map; with `hiddenByFog`, the card "O mapa fica na tela de cada um". */
      readonly map: TvMapVm | null;
    };

export interface TvScreenVm {
  readonly campaignName: string;
  readonly sessionNumber: number;
  /** "Sessão 3". */
  readonly sessionLabel: string;
  readonly party: readonly string[];
  readonly stage: TvStageVm;
  /** The free rolls of the table, newest first. */
  readonly rolls: readonly TvRollVm[];
}

const STATE_WORDS: Partial<Record<TvState, string>> = {
  [TvState.UNHURT]: 'Ileso',
  [TvState.HURT]: 'Ferido',
  [TvState.VERY_HURT]: 'Muito ferido',
  [TvState.DEFEATED]: 'Derrotado',
  [TvState.FALLEN]: 'Caído',
};

function toMapVm(map: TvMap): TvMapVm {
  return {
    name: map.name,
    hiddenByFog: map.hiddenByFog,
    map: map.map ?? null,
    vision: map.vision ?? null,
    revision: map.revision,
  };
}

function toStageVm(res: GetTableScreenResponse): TvStageVm {
  const body = res.stage?.body;
  switch (body?.case) {
    case 'map':
      return { kind: 'map', map: toMapVm(body.value) };
    case 'image': {
      const { name, url, width, height } = body.value;
      return { kind: 'image', name, url, width, height };
    }
    case 'scene': {
      const { name, description, stage } = body.value;
      return {
        kind: 'scene',
        name,
        description,
        stage: stage.map((n) => ({ name: n.name, speaking: n.speaking })),
      };
    }
    case 'combat': {
      const c = body.value;
      return {
        kind: 'combat',
        round: c.round,
        mastersTurn: c.mastersTurn,
        log: [...c.log],
        map: c.map ? toMapVm(c.map) : null,
        order: c.order.map((o) => ({
          name: o.name,
          initial: o.initial,
          stateWord: STATE_WORDS[o.state] ?? null,
          onTurn: o.onTurn,
          isPlayer: o.isPlayer,
        })),
      };
    }
    default:
      // Idle, or a body this app does not know: the waiting screen.
      return { kind: 'idle', party: [...res.party] };
  }
}

/** What the TV renders for one `GetTableScreen`. */
export function toTvScreen(res: GetTableScreenResponse): TvScreenVm {
  return {
    campaignName: res.campaignName,
    sessionNumber: res.sessionNumber,
    sessionLabel: `Sessão ${res.sessionNumber}`,
    party: [...res.party],
    stage: toStageVm(res),
    rolls: (res.stage?.rolls ?? []).map((r) => ({
      who: r.who,
      expression: r.expression,
      total: r.total,
      faces: [...r.faces],
    })),
  };
}

/** The kind a stage shows, for the labels of the master's panel. */
export function stageKindOf(res: GetTableScreenResponse): TableScreenKind {
  return res.stage?.kind ?? TableScreenKind.UNSPECIFIED;
}
