import { type Signal, computed } from '@angular/core';

import {
  LevelUpProficiencyKind,
  type LevelUpOptions,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import type { LevelUpCatalog } from './levelup-draft-types';
import {
  type PickItem,
  type SheetKeys,
  type StepKey,
  cantripOptions,
  expertiseOptions,
  preparedMore,
  preparedOptions,
  proficiencyPick,
  skillOptions,
  spellOptions,
  stepsFor,
  totalsFor,
  withSubclass,
} from './levelup-flow';

/** The picks the lists below are read against. */
export interface ListPicks {
  readonly subclassKey: Signal<string>;
  readonly spells: Signal<ReadonlySet<string>>;
  readonly skills: Signal<ReadonlySet<string>>;
  /** The new maximum of prepared spells. */
  readonly preparedMax: Signal<number>;
}

/**
 * What a guided level-up offers to pick from and how many it asks of each, from the server's options, the sheet's keys
 * and the rules' lists, with the picks so far (the subclass changes the counts, the spells change what can be prepared).
 */
export class LevelUpLists {
  /** The server's options with the picked subclass's casting folded in: what the spell steps read. */
  readonly effective = computed(() => withSubclass(this.options, this.picks.subclassKey()));
  /** Whose spell list the new spells come from, by name: the class's own, the one a table class reuses, a third caster's. */
  readonly listName = computed(
    () =>
      this.catalog.classes?.find((c) => c.key === this.effective().spellListClassKey)?.namePt ??
      this.options.classNamePt,
  );
  readonly totals = computed(() => totalsFor(this.options, this.picks.subclassKey()));
  readonly more = computed(() =>
    preparedMore(this.effective(), this.picks.preparedMax(), this.have.prepared.length),
  );
  readonly steps = computed<StepKey[]>(() => stepsFor(this.options, this.totals(), this.more()));

  readonly cantripItems = computed<PickItem[]>(() =>
    cantripOptions(this.effective(), this.catalog.spells, this.have),
  );
  private readonly spellItemsBase = computed<PickItem[]>(() =>
    spellOptions(this.effective(), this.catalog.spells, this.have),
  );
  /** The new spells, with the rows from outside the class list turned off once the cap is reached. */
  readonly spellItems = computed<PickItem[]>(() => {
    const base = this.spellItemsBase();
    const full =
      base.filter((i) => i.outside && this.picks.spells().has(i.key)).length >=
      this.options.anyClassSpells;
    return full
      ? base.map((i) =>
          i.outside && !this.picks.spells().has(i.key)
            ? { ...i, disabled: `Limite de ${this.options.anyClassSpells} de outra classe` }
            : i,
        )
      : base;
  });
  readonly preparedItems = computed<PickItem[]>(() =>
    preparedOptions(this.effective(), this.catalog.spells, this.have, this.picks.spells()),
  );
  readonly skillItems = computed<PickItem[]>(() =>
    skillOptions(this.catalog.skills, this.have, this.options),
  );
  /** The instruments of a new Bard, with the ones the character has turned off. */
  readonly instrumentItems = computed<PickItem[]>(() =>
    (
      this.options.proficiencyChoices.find((c) => c.kind === LevelUpProficiencyKind.INSTRUMENT)
        ?.from ?? []
    ).map((f) => ({
      key: f.key,
      name: f.namePt,
      sub: '',
      ...(f.alreadyHave ? { disabled: 'Você já tem' } : {}),
    })),
  );
  readonly instrumentAsked = computed(() =>
    proficiencyPick(this.options, LevelUpProficiencyKind.INSTRUMENT),
  );
  readonly expertiseItems = computed<PickItem[]>(() =>
    expertiseOptions(this.catalog.skills, this.have, this.picks.skills()),
  );

  /** The Portuguese name of every key the screen may show. */
  readonly names = computed(() => {
    const names = new Map<string, string>(this.catalog.names);
    for (const s of this.catalog.spells) names.set(s.key, s.namePt);
    for (const s of this.catalog.skills) names.set(s.key, s.namePt);
    for (const s of this.options.subclasses) names.set(s.key, s.namePt);
    for (const f of this.totals().featureChoices) {
      names.set(f.feature?.key ?? '', f.feature?.namePt ?? '');
      for (const o of f.options) names.set(o.key, o.namePt);
    }
    return names;
  });

  /** What each count asks: the server's own number, never less. A list with fewer rows than that is
   * said so (`short`), never a way through: the server refuses fewer. Only the spells to prepare are
   * capped by their list, since the server asks for no more than the new maximum, not exactly that. */
  readonly cantripsAsked = computed(() => this.totals().cantrips);
  readonly spellsAsked = computed(() => this.totals().spells);
  readonly preparedAsked = computed(() => Math.min(this.more(), this.preparedItems().length));
  readonly skillsAsked = computed(() => this.totals().skills);
  readonly expertiseAsked = computed(() => this.totals().expertise);

  /** How many spells from outside the class list were picked, and how many may be (Magical Secrets). */
  readonly outsidePicked = computed(
    () => this.spellItems().filter((i) => i.outside && this.picks.spells().has(i.key)).length,
  );
  readonly outsideMax = computed(() => this.options.anyClassSpells);

  constructor(
    private readonly options: LevelUpOptions,
    private readonly have: SheetKeys,
    private readonly catalog: LevelUpCatalog,
    private readonly picks: ListPicks,
  ) {}
}
