import type { CharacterVitals } from '../../../../gen/meurpg/play/v1/play_pb';

/** What the sheet reads of the druid's Wild Shape in the vitals: the uses, and the form with its time and its pool. */
export interface WildRead {
  readonly uses: {
    readonly left: number;
    readonly total: number;
    readonly recharge: string;
  } | null;
  readonly form: string;
  /** "52 min restantes", as the server writes it; empty for a reader who may not have it. */
  readonly time: string;
  readonly beastKey: string;
  readonly hp: { readonly current: number; readonly max: number } | null;
}

export function readWild(v: CharacterVitals | null): WildRead {
  const r = v?.resources.find((x) => x.key === 'wild_shape');
  const shape = v?.wildShape;
  return {
    uses: r
      ? {
          left: r.total - r.used,
          total: r.total,
          recharge: r.recharge === 2 ? 'long_rest' : 'short_rest',
        }
      : null,
    form: shape?.beastNamePt ?? '',
    time: shape?.timeLeftPt ?? '',
    beastKey: shape?.beastKey ?? '',
    hp: shape ? { current: shape.hitPointsCurrent, max: shape.hitPointsMax } : null,
  };
}
