import type { TrapState } from '../../../../gen/meurpg/maps/v1/maps_pb';
import type {
  Ability,
  TrapPassOutcome,
  TrapSaveApplies,
  TrapTargets,
  TrapTrigger,
} from '../../../../gen/meurpg/rules/v1/rules_pb';

/** The shapes of the trap form's text and choices; the functions that build and check them are in `trap-draft` and `trap-errors`. */
export interface DamageDraft {
  readonly dice: string;
  readonly typeKey: string;
}

export interface ConditionDraft {
  readonly key: string;
  /** "1 hora": how long it lasts, for the master's table (0 to 60 characters). */
  readonly duration: string;
}

export interface AttackDraft {
  readonly bonus: string;
  readonly count: string;
  readonly damage: DamageDraft;
}

export interface SaveDraft {
  readonly ability: Ability;
  readonly dc: string;
  readonly appliesTo: TrapSaveApplies;
  readonly failDamage: readonly DamageDraft[];
  readonly failCondition: ConditionDraft | null;
  readonly onPass: TrapPassOutcome;
}

export interface TrapDraft {
  readonly presetKey: string;
  readonly name: string;
  readonly description: string;
  /** Empty: nobody notices it alone (the SRD's poison needle). */
  readonly noticeDc: string;
  readonly findDc: string;
  /** "Também acham com": the skill keys (`skill:arcana`) besides Percepção and Investigação that find it, against `findDc`; none by default. */
  readonly alsoFind: readonly string[];
  // eslint-disable-next-line @typescript-eslint/no-magic-numbers -- the SRD's trap areas, 1×1 to 4×4
  readonly areaSize: 1 | 2 | 3 | 4;
  readonly trigger: TrapTrigger;
  readonly state: TrapState;
  readonly targets: TrapTargets;
  readonly attack: AttackDraft | null;
  /** "Dano que sempre acontece": damage that lands on every creature caught. */
  readonly damage: readonly DamageDraft[];
  /** "Condição que sempre acontece". */
  readonly conditions: readonly ConditionDraft[];
  readonly save: SaveDraft | null;
}
