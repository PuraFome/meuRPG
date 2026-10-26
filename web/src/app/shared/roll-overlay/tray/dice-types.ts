/** The shared shapes of the dice tray: no Angular, no DOM. */

export type Vec3 = readonly [number, number, number];
/** A 3×3 matrix, row-major, nine numbers. */
export type Mat3 = readonly number[];

/** The solids a die is built from. A d100 is two d10. */
export type Kind = 4 | 6 | 8 | 10 | 12 | 20;
export type DiceSides = Kind | 100;
export type DieStyle = 'paper' | 'garnet';

/** One die the caller asks for. `result` always comes from the caller (the server): the tray never decides it. */
export interface DieSpec {
  readonly sides: DiceSides;
  readonly result: number;
  /** Drawn at half strength, labelled "descartado". */
  readonly discarded?: boolean;
  /** A label such as "vale". */
  readonly tag?: string;
  /** A gold glow, a gold number, sparks and "Crítico!". */
  readonly critical?: boolean;
  /** Darkened, a short shake, "Falha crítica". */
  readonly fumble?: boolean;
  /** Garnet for a d20 and for the tens die of a d100, paper otherwise. */
  readonly style?: DieStyle;
  readonly unitsStyle?: DieStyle;
}

/** One solid on the tray: a d100 expands to two of these. */
export interface Body {
  readonly source: number;
  readonly discarded: boolean;
  readonly tag: string;
  readonly critical: boolean;
  readonly fumble: boolean;
  readonly part: '' | 'dezenas' | 'unidades';
  readonly kind: Kind;
  /** What is printed on the faces (or, for a d4, at the corners), in order. */
  readonly labels: readonly string[];
  /** The label that must end up on top. */
  readonly target: string;
  readonly style: DieStyle;
}

export type SoundKind = 'floor' | 'roll' | 'rock' | 'dice' | 'wall';

/** A sound event: when (simulation seconds), how hard (0 to 1), what hit what, their pitch and where across the tray. */
export interface SoundEvent {
  readonly t: number;
  readonly s: number;
  readonly kind: SoundKind;
  readonly i: number;
  readonly j: number;
  readonly pitch: number;
  /** 0 left, 1 right: the pan. */
  readonly x: number;
}

export interface Box {
  readonly l: number;
  readonly t: number;
  readonly r: number;
  readonly b: number;
}

/** The mutable state of one die during the simulation. */
export interface DieState {
  readonly i: number;
  /** Collision radius against the walls, and against other dice. */
  readonly rw: number;
  readonly rh: number;
  readonly stepLen: number;
  readonly keep: number;
  readonly pitch: number;
  x: number;
  y: number;
  vx: number;
  vy: number;
  z: number;
  vz: number;
  angle: number;
  spin: number;
  /** The tumble: progress through tipping onto the next face, its speed, the faces done, and the direction it tips. */
  p: number;
  w: number;
  k: number;
  tdx: number;
  tdy: number;
  /** The impact squash and its direction (NaN: from a landing). */
  sq: number;
  sqAng: number;
  entry: 'left' | 'right' | 'bottom';
  entering: boolean;
  rest: boolean;
}

/** The recording of a whole roll. */
export interface Sim {
  readonly R: number;
  readonly S: number;
  readonly n: number;
  readonly steps: number;
  /** FRAME floats per die per step: x, y, angle, z, p, k, tdx, tdy, squash, squash angle. */
  readonly frames: Float64Array;
  readonly events: readonly SoundEvent[];
  readonly forced: boolean;
  /** How much rolling goes on at each step, for the rumble. */
  readonly rolling: Float32Array;
  readonly duration: number;
  readonly flips: readonly number[];
  readonly angles: readonly number[];
}

export interface SimOptions {
  readonly width: number;
  readonly height: number;
  readonly seed: number;
}

/** What `simulate` reports, for tests and tuning. */
export interface SimulationReport {
  readonly durationMs: number;
  readonly forced: boolean;
  readonly flips: readonly number[];
  /** Read back from the final pose (not copied from the input): proves the dice land on the results. */
  readonly finalFaces: readonly string[];
  readonly events: number;
  readonly sound: { readonly byKind: Readonly<Record<string, number>>; readonly quietMs: number };
  readonly dieRadius: number;
  readonly steps: number;
  /** The angle (radians) each die's number is turned from upright at rest. */
  readonly restTilts: readonly number[];
}

export interface Edge {
  readonly a: number;
  readonly b: number;
  readonly f0: number;
  f1: number;
  beta: number;
  mid: Vec3;
}

export interface Face {
  readonly idx: readonly number[];
  readonly c: Vec3;
  readonly n: Vec3;
  readonly up: Vec3;
  readonly right: Vec3;
  /** The radius of the circle inside the face. */
  readonly inr: number;
  readonly edges: Edge[];
}

/** A d4 prints its number at each corner of each face. */
export interface Corner {
  readonly vi: number;
  readonly at: Vec3;
  readonly up: Vec3;
  readonly right: Vec3;
}

export interface Poly {
  readonly kind: Kind;
  readonly verts: readonly Vec3[];
  readonly faces: readonly Face[];
  readonly edges: readonly Edge[];
  /** Scale that gives the silhouette its radius at rest. */
  readonly size: number;
  /** Number size against the face's inradius. */
  readonly digits: number;
  readonly corners: readonly (readonly Corner[])[] | null;
}

export interface Layout {
  readonly byCorner: boolean;
  readonly labels: readonly string[];
}

/** One stretch of a tumble in hindsight: a real tip over a real edge. */
export interface Segment {
  first: number;
  last: number;
  readonly k: number;
  readonly tdx: number;
  readonly tdy: number;
  u: Vec3;
  beta: number;
  P: Mat3;
}

/** How a body is oriented over the whole roll. */
export interface Orientation {
  readonly poly: Poly;
  readonly layout: Layout;
  readonly segs: readonly Segment[];
  readonly segOf: Int32Array | null;
  readonly angleEnd: number;
  readonly rest: Mat3;
  /** The angle the final pose is turned from upright. */
  readonly tilt: number;
}

/** An RGB colour, 0 to 255. */
export type Rgb = readonly [number, number, number];

/** The colours and fonts the tray draws with, read from the page's custom properties. */
export interface Theme {
  readonly floor: Rgb;
  readonly rim: Rgb;
  readonly rimHi: Rgb;
  readonly rimLo: Rgb;
  readonly shadow: Rgb;
  readonly paper: Rgb;
  readonly paperInk: Rgb;
  readonly paperShade: Rgb;
  readonly garnet: Rgb;
  readonly garnetInk: Rgb;
  readonly garnetShade: Rgb;
  readonly edge: Rgb;
  readonly gold: Rgb;
  readonly goldOn: Rgb;
  readonly surface: Rgb;
  readonly ink: Rgb;
  readonly line: Rgb;
  readonly accent: Rgb;
  readonly muted: Rgb;
  readonly display: string;
  readonly text: string;
}

export interface Spark {
  readonly ang: number;
  readonly speed: number;
  readonly life: number;
  readonly size: number;
  readonly rot: number;
}

/** A body ready to draw: its orientation over the roll, the die radius and the sparks of a critical. */
export interface SceneBody extends Body, Orientation {
  readonly R: number;
  readonly sparks: readonly Spark[] | null;
}

/** Where a die is on screen now, and the pose it is in. */
export interface View {
  x: number;
  y: number;
  z: number;
  /** The flat spin, relative to the final angle. */
  psi: number;
  /** The tumble progress and the stretch it is in. */
  p: number;
  seg: number;
  sq: number;
  sqAng: number;
  shape: Shape | null;
}

/** A die's pose on screen: the world rotation, its vertices (px, y down) and its face normals. */
export interface Shape {
  readonly M: Mat3;
  readonly S: number;
  readonly pts: readonly (readonly [number, number])[];
  readonly normals: readonly Vec3[];
}

export type LabelLook = 'crit' | 'vale' | 'fumble' | 'quiet' | 'part';

export interface PlacedLabel {
  readonly i: number;
  readonly x: number;
  readonly y: number;
  readonly size: { readonly w: number; readonly h: number };
  readonly text: string;
  readonly look: LabelLook;
}

export interface Scene {
  readonly bodies: readonly SceneBody[];
  readonly views: View[];
  readonly sim: Sim | null;
  readonly order: number[];
  readonly R: number;
  labels: PlacedLabel[] | null;
}

/** What is going on around a die while it is drawn. */
export interface Effects {
  readonly settle: number;
  readonly alpha: number;
  readonly crit: number;
  readonly ash: number;
  readonly dx: number;
}

/** What `roll` resolves with. */
export interface RollSummary {
  readonly cancelled: boolean;
  readonly seed: number;
  readonly durationMs: number;
  readonly dice: readonly {
    readonly sides: number;
    readonly result: number;
    readonly discarded: boolean;
  }[];
}

export interface RollOptions {
  /** A uint32; random when omitted. */
  readonly seed?: number;
  /** Playback rate, 1 is normal; it does not change the physics. */
  readonly speed?: number;
}

export interface TrayOptions {
  readonly width?: number;
  readonly height?: number;
  readonly reducedMotion?: boolean;
  readonly sound?: boolean;
}
