import type { signal } from '@angular/core';

import type { Skill, Spell } from '../../../gen/meurpg/rules/v1/rules_pb';

/** The two cards of "Pontos de vida": the average, or a die. */
export type HpCard = 'average' | 'roll';

/** A die result: the server's kept roll, or the one typed from a physical die. */
export interface Rolled {
  readonly kind: 'app' | 'physical';
  readonly value: number;
}

/** What the player takes at an Ability Score Improvement level: the increase of the SRD, or a feat. */
export type AsiMode = 'increase' | 'feat';

/** A set of picked keys, held in a signal. */
export type PickSet = ReturnType<typeof signal<ReadonlySet<string>>>;

/** What the draft reads from the rules content: the spell and skill lists, the names of other keys and the classes. */
export interface LevelUpCatalog {
  readonly spells: readonly Spell[];
  readonly skills: readonly Skill[];
  readonly names?: ReadonlyMap<string, string>;
  readonly classes?: readonly {
    readonly key: string;
    readonly namePt: string;
    readonly hitDie?: number;
  }[];
}
