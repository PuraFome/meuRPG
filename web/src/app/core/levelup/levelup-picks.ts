import {
  LevelUpHitPointsRule,
  type LevelUpOptions,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { toggledIn } from '../format/sets';
import type { HpCard, PickSet, Rolled } from './levelup-draft-types';

/** Drops the picks `keep` refuses, leaving the set itself alone when nothing goes (a new set would wake every reader). */
export function keepOnly(set: PickSet, keep: (key: string) => boolean): void {
  const kept = [...set()].filter(keep);
  if (kept.length !== set().size) {
    set.set(new Set(kept));
  }
}

/** Keeps the first `max` picks of a list. */
export function trim(set: PickSet, max: number): void {
  if (set().size > max) {
    set.set(new Set([...set()].slice(0, max)));
  }
}

/** Picks or drops `key`: with a single place the pick replaces, otherwise a full list refuses. */
export function flip(set: PickSet, key: string, max: number): void {
  const now = set();
  if (!now.has(key) && max === 1) {
    set.set(new Set([key]));
  } else if (now.has(key) || now.size < max) {
    set.set(toggledIn(now, key));
  }
}

/** The picks of `picked` that are among the `items` offered. */
export function offeredAmong(
  picked: ReadonlySet<string>,
  items: readonly { readonly key: string }[],
): Set<string> {
  const ok = new Set(items.map((i) => i.key));
  return new Set([...picked].filter((k) => ok.has(k)));
}

/** The feature options picked once `key` is tapped: each choice (a fighting style, the metamagic) takes its own `choose`. */
export function nextFeatures(
  now: ReadonlySet<string>,
  key: string,
  group: readonly string[],
  choose: number,
): ReadonlySet<string> {
  if (now.has(key)) {
    const next = new Set(now);
    next.delete(key);
    return next;
  }
  if (choose === 1) {
    return new Set([...now].filter((k) => !group.includes(k)).concat(key));
  }
  const inGroup = [...now].filter((k) => group.includes(k));
  return inGroup.length < choose ? new Set([...now, key]) : now;
}

/** The hit points card of the table: its rule decides when it has one, else what the player had open. */
function adoptedCard(o: LevelUpOptions, was: HpCard): HpCard {
  if (o.hitPointsRule === LevelUpHitPointsRule.AVERAGE_ONLY) {
    return 'average';
  }
  return o.hitPointsRule === LevelUpHitPointsRule.ROLL_ONLY ? 'roll' : was;
}

/** Whether a die result still stands for these options: an in-app roll is the server's own kept roll for this class
 * and level (a roll belongs to the level it was made for), and a typed one stays only for the same level and a die it fits. */
function rollStands(o: LevelUpOptions, was: Rolled, wasOptions: LevelUpOptions): boolean {
  if (was.kind === 'app') {
    const keptClass = o.keptHitPointRollClassKey;
    return o.keptHitPointRoll === was.value && (keptClass === '' || keptClass === o.classKey);
  }
  return (
    o.classKey === wasOptions.classKey &&
    o.toLevel === wasOptions.toLevel &&
    Number.isInteger(was.value) &&
    was.value >= 1 &&
    was.value <= o.hitDie
  );
}

/** The hit points card and die result of an earlier draft, as far as these options still allow them. */
export function adoptedHitPoints(
  o: LevelUpOptions,
  wasOptions: LevelUpOptions,
  wasCard: HpCard,
  wasRolled: Rolled | null,
): { readonly card: HpCard; readonly rolled: Rolled | null } {
  const card = adoptedCard(o, wasCard);
  const keep = card === 'roll' && wasRolled !== null && rollStands(o, wasRolled, wasOptions);
  return { card, rolled: keep ? wasRolled : null };
}

/** The feature options a subclass brings: what goes when the player changes it. */
export function featureOptionKeys(o: LevelUpOptions, subclassKey: string): Set<string> {
  const choices = o.subclasses.find((c) => c.key === subclassKey)?.featureChoices ?? [];
  return new Set(choices.flatMap((f) => f.options.map((opt) => opt.key)));
}

/** Whether the subclass prepares spells itself (a third caster): it then says its own maximum until the preview does. */
export function subclassPrepares(o: LevelUpOptions, subclassKey: string): boolean {
  return o.subclasses.find((c) => c.key === subclassKey)?.prepares === true;
}

/** What the draft is asked whether anything was picked: every pick, as a signal or a plain value. */
export interface PickedSource {
  readonly abilityKeys: () => readonly unknown[];
  readonly featKey: () => string;
  readonly hpCard: () => HpCard;
  readonly subclassKey: () => string;
  readonly lateDrafts: () => ReadonlyMap<string, unknown>;
  readonly newDrafts: () => ReadonlyMap<string, unknown>;
  readonly instrument: () => string;
  readonly cantrips: () => ReadonlySet<string>;
  readonly spells: () => ReadonlySet<string>;
  readonly prepared: () => ReadonlySet<string>;
  readonly features: () => ReadonlySet<string>;
  readonly skills: () => ReadonlySet<string>;
  readonly expertise: () => ReadonlySet<string>;
}

/** Whether anything was chosen beyond the defaults (`startCard` being the card the hit points start on). */
export function anyPicked(d: PickedSource, startCard: HpCard): boolean {
  return (
    d.abilityKeys().length > 0 ||
    d.featKey() !== '' ||
    d.hpCard() !== startCard ||
    d.subclassKey() !== '' ||
    d.lateDrafts().size > 0 ||
    d.newDrafts().size > 0 ||
    d.instrument() !== '' ||
    [d.cantrips(), d.spells(), d.prepared(), d.features(), d.skills(), d.expertise()].some(
      (s) => s.size > 0,
    )
  );
}
