import { computed, signal } from '@angular/core';

import {
  LevelUpHitPointsMethod,
  LevelUpSpellsKind,
  type LevelUpOptions,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import type { Skill, Spell } from '../../../gen/meurpg/rules/v1/rules_pb';
import { ABILITY_KEYS, type AbilityKey } from '../characters/characters.types';
import type { LevelUpChoicesInit } from './levelup-client';
import {
  type Missing,
  type PickItem,
  type SheetKeys,
  type StepKey,
  cantripOptions,
  expertiseOptions,
  needText,
  preparedMore,
  preparedOptions,
  skillOptions,
  spellOptions,
  stepsFor,
  totalsFor,
} from './levelup-flow';

/** The two cards of "Pontos de vida": the average, or a die. */
export type HpCard = 'average' | 'roll';

/** A die result: the server's kept roll, or the one typed from a physical die. */
export interface Rolled {
  readonly kind: 'app' | 'physical';
  readonly value: number;
}

const WIRE_ABILITY: Record<AbilityKey, string> = {
  str: 'strength',
  dex: 'dexterity',
  con: 'constitution',
  int: 'intelligence',
  wis: 'wisdom',
  cha: 'charisma',
};

type PickSet = ReturnType<typeof signal<ReadonlySet<string>>>;

/**
 * Everything the player has chosen so far in one guided level-up (MR-040):
 * the ability increase, the hit points, the subclass and the new cantrips,
 * spells, feature options, skills and expertise, with the steps they make,
 * what is still missing and the `LevelUpChoices` to send. Nothing is saved
 * until "Confirmar o nível N" (the server checks every choice with the rules
 * engine); this only counts the picks against the counts the server gave.
 */
export class LevelUpDraft {
  readonly abilityMode = signal<'one' | 'two'>('one');
  readonly abilityKeys = signal<readonly AbilityKey[]>([]);
  readonly hpCard = signal<HpCard>('average');
  readonly rolled = signal<Rolled | null>(null);
  readonly subclassKey = signal('');
  readonly cantrips: PickSet = signal<ReadonlySet<string>>(new Set());
  readonly spells: PickSet = signal<ReadonlySet<string>>(new Set());
  readonly prepared: PickSet = signal<ReadonlySet<string>>(new Set());
  /** The options of every feature choice, together: each option key is unique. */
  readonly features: PickSet = signal<ReadonlySet<string>>(new Set());
  readonly skills: PickSet = signal<ReadonlySet<string>>(new Set());
  readonly expertise: PickSet = signal<ReadonlySet<string>>(new Set());
  /** The new maximum of prepared spells, as `PreviewLevelUp` derives it (an ability increase moves it). */
  readonly preparedMaxAfter = signal(0);

  constructor(
    readonly options: LevelUpOptions,
    readonly have: SheetKeys,
    readonly catalog: { readonly spells: readonly Spell[]; readonly skills: readonly Skill[] },
  ) {
    this.preparedMaxAfter.set(options.preparedMaxAfter);
  }

  readonly totals = computed(() => totalsFor(this.options, this.subclassKey()));
  readonly more = computed(() => preparedMore(this.options, this.preparedMaxAfter(), this.have.prepared.length));
  readonly steps = computed<StepKey[]>(() => stepsFor(this.options, this.totals(), this.more()));

  readonly cantripItems = computed<PickItem[]>(() =>
    cantripOptions(this.options, this.catalog.spells, this.have),
  );
  private readonly spellItemsBase = computed<PickItem[]>(() => spellOptions(this.options, this.catalog.spells, this.have));
  /** The new spells, with the rows from outside the class list turned off once the cap is reached. */
  readonly spellItems = computed<PickItem[]>(() => {
    const base = this.spellItemsBase();
    const full = base.filter((i) => i.outside && this.spells().has(i.key)).length >= this.options.anyClassSpells;
    return full
      ? base.map((i) =>
          i.outside && !this.spells().has(i.key) ? { ...i, disabled: `Limite de ${this.options.anyClassSpells} de outra classe` } : i,
        )
      : base;
  });
  readonly preparedItems = computed<PickItem[]>(() =>
    preparedOptions(this.options, this.catalog.spells, this.have, this.spells()),
  );
  readonly skillItems = computed<PickItem[]>(() => skillOptions(this.catalog.skills, this.have));
  readonly expertiseItems = computed<PickItem[]>(() =>
    expertiseOptions(this.catalog.skills, this.have, this.skills()),
  );

  /** The Portuguese name of every key the screen may show. */
  readonly names = computed(() => {
    const names = new Map<string, string>();
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
    () => this.spellItems().filter((i) => i.outside && this.spells().has(i.key)).length,
  );
  readonly outsideMax = computed(() => this.options.anyClassSpells);

  readonly abilityAsked = computed(() => (this.abilityMode() === 'one' ? 1 : 2));
  readonly abilityIncrease = computed<Partial<Record<AbilityKey, number>>>(() => {
    const keys = this.abilityKeys();
    if (!this.options.abilityScoreImprovement || keys.length !== this.abilityAsked()) {
      return {};
    }
    const amount = this.abilityMode() === 'one' ? 2 : 1;
    return Object.fromEntries(keys.map((k) => [k, amount]));
  });

  /** Every choice still to make, in step order, each with the picker that holds it. */
  readonly missing = computed<Missing[]>(() => {
    const o = this.options;
    const t = this.totals();
    const out: Missing[] = [];
    if (o.abilityScoreImprovement && this.abilityKeys().length < this.abilityAsked()) {
      const n = this.abilityAsked() - this.abilityKeys().length;
      out.push({ step: 'abilities', id: 'abilities', text: needText(n, 'atributo', 'atributos') });
    }
    if (this.hpCard() === 'roll' && this.rolled() === null) {
      out.push({ step: 'hp', id: 'hp', text: 'Falta rolar o dado de vida.' });
    }
    if (o.subclassDue && this.subclassKey() === '') {
      out.push({ step: 'picks', id: 'subclass', text: 'Falta escolher a subclasse.' });
    }
    t.featureChoices.forEach((f, i) => {
      const picked = f.options.filter((opt) => this.features().has(opt.key)).length;
      if (picked < f.choose) {
        const n = f.choose - picked;
        out.push({
          step: 'picks',
          id: `feature-${i}`,
          text: needText(n, `opção de ${f.feature?.namePt ?? 'característica'}`, `opções de ${f.feature?.namePt ?? 'característica'}`),
        });
      }
    });
    const short = (picked: ReadonlySet<string>, asked: number) => Math.max(0, asked - picked.size);
    // A list with fewer rows than the level asks cannot be completed: the message says so (the server would refuse less).
    const lack = (asked: number, items: readonly PickItem[], what: string): string =>
      items.length < asked ? `A lista só traz ${items.length} ${what} e o nível pede ${asked}. Peça ao mestre para ajustar a ficha.` : '';
    let n = short(this.skills(), this.skillsAsked());
    if (n > 0) out.push({ step: 'picks', id: 'skills', text: lack(this.skillsAsked(), this.skillItems(), 'perícias') || needText(n, 'perícia', 'perícias') });
    n = short(this.expertise(), this.expertiseAsked());
    if (n > 0) out.push({ step: 'picks', id: 'expertise', text: lack(this.expertiseAsked(), this.expertiseItems(), 'perícias para a especialização') || needText(n, 'especialização', 'especializações') });
    n = short(this.cantrips(), this.cantripsAsked());
    if (n > 0) out.push({ step: 'spells', id: 'cantrips', text: lack(this.cantripsAsked(), this.cantripItems(), 'truques') || needText(n, 'truque', 'truques') });
    n = short(this.spells(), this.spellsAsked());
    if (n > 0) {
      const where = o.spellsKind === LevelUpSpellsKind.SPELLBOOK ? 'para o livro' : 'para as magias conhecidas';
      out.push({ step: 'spells', id: 'spells', text: lack(this.spellsAsked(), this.spellItems(), 'magias') || needText(n, 'magia', 'magias').replace(/\.$/, ` ${where}.`) });
    }
    n = short(this.prepared(), this.preparedAsked());
    if (n > 0) out.push({ step: 'spells', id: 'prepared', text: needText(n, 'magia', 'magias', 'preparar') });
    return out;
  });

  /** What is missing in `step`, if anything: the step's "Próximo" waits for it. */
  missingIn(step: StepKey): readonly Missing[] {
    return this.missing().filter((m) => m.step === step);
  }

  /** Whether anything was chosen beyond the defaults: leaving then asks first. */
  readonly dirty = computed(
    () =>
      this.abilityKeys().length > 0 ||
      this.hpCard() !== 'average' ||
      this.subclassKey() !== '' ||
      [this.cantrips(), this.spells(), this.prepared(), this.features(), this.skills(), this.expertise()].some(
        (s) => s.size > 0,
      ),
  );

  private build(hp: Rolled | null): LevelUpChoicesInit {
    const increase = this.abilityIncrease();
    return {
      classKey: this.options.classKey,
      abilityIncrease: Object.fromEntries(
        ABILITY_KEYS.filter((k) => increase[k]).map((k) => [WIRE_ABILITY[k], increase[k]]),
      ),
      subclassKey: this.subclassKey(),
      cantripKeys: [...this.cantrips()],
      knownSpellKeys: [...this.spells()],
      preparedSpellKeys: [...this.prepared()],
      featureChoiceKeys: [...this.features()],
      skillProficiencyKeys: [...this.skills()],
      expertiseSkillKeys: [...this.expertise()],
      hitPoints: hp
        ? {
            method:
              hp.kind === 'app'
                ? LevelUpHitPointsMethod.ROLLED_IN_APP
                : LevelUpHitPointsMethod.ROLLED_PHYSICAL,
            value: hp.value,
          }
        : { method: LevelUpHitPointsMethod.AVERAGE },
    };
  }

  /** The choices to send. Incomplete is fine for a preview: the answer says what is missing. */
  readonly choices = computed<LevelUpChoicesInit>(() =>
    this.build(this.hpCard() === 'roll' ? this.rolled() : null),
  );

  /** The same choices with the average hit points: "Média: 4" shows its own gain whichever card is open. */
  readonly averageChoices = computed<LevelUpChoicesInit>(() => this.build(null));

  setAbilityMode(mode: 'one' | 'two'): void {
    if (mode !== this.abilityMode()) {
      this.abilityMode.set(mode);
      this.abilityKeys.set([]);
    }
  }

  /** One ability for +2, two for +1 each: a third replaces the oldest. */
  toggleAbility(key: AbilityKey): void {
    const keys = this.abilityKeys();
    if (keys.includes(key)) {
      this.abilityKeys.set(keys.filter((k) => k !== key));
      return;
    }
    this.abilityKeys.set([...keys, key].slice(-this.abilityAsked()));
  }

  setHpCard(card: HpCard): void {
    this.hpCard.set(card);
  }

  setSubclass(key: string): void {
    if (key === this.subclassKey()) {
      return;
    }
    const gone = new Set(
      (this.options.subclasses.find((c) => c.key === this.subclassKey())?.featureChoices ?? []).flatMap((f) =>
        f.options.map((o) => o.key),
      ),
    );
    this.subclassKey.set(key);
    // Only what belonged to the previous subclass goes: its feature options. The cantrips, skills and
    // expertise that the level itself asks for stay, trimmed to what the new subclass's counts allow.
    this.features.set(new Set([...this.features()].filter((k) => !gone.has(k))));
    this.trim(this.cantrips, this.cantripsAsked());
    this.trim(this.skills, this.skillsAsked());
    this.trim(this.expertise, this.expertiseAsked());
  }

  /** Takes over what `other` had picked (after the sheet was read again), keeping only what these
   * options and lists still allow, then trimmed to the counts. */
  adopt(other: LevelUpDraft): void {
    if (this.options.abilityScoreImprovement) {
      this.abilityMode.set(other.abilityMode());
      this.abilityKeys.set(other.abilityKeys().slice(0, this.abilityAsked()));
    }
    this.hpCard.set(other.hpCard());
    this.rolled.set(other.hpCard() === 'roll' ? other.rolled() : null);
    if (this.options.subclasses.some((c) => c.key === other.subclassKey())) {
      this.subclassKey.set(other.subclassKey());
    }
    const keep = (picked: ReadonlySet<string>, items: readonly PickItem[]) => {
      const ok = new Set(items.map((i) => i.key));
      return new Set([...picked].filter((k) => ok.has(k)));
    };
    this.cantrips.set(keep(other.cantrips(), this.cantripItems()));
    this.spells.set(keep(other.spells(), this.spellItems()));
    this.prepared.set(keep(other.prepared(), this.preparedItems()));
    const options = new Set(this.totals().featureChoices.flatMap((f) => f.options.map((o) => o.key)));
    this.features.set(new Set([...other.features()].filter((k) => options.has(k))));
    this.skills.set(keep(other.skills(), this.skillItems()));
    this.expertise.set(keep(other.expertise(), this.expertiseItems()));
    this.trim(this.cantrips, this.cantripsAsked());
    this.trim(this.spells, this.spellsAsked());
    this.trim(this.prepared, this.preparedAsked());
    this.trim(this.skills, this.skillsAsked());
    this.trim(this.expertise, this.expertiseAsked());
  }

  /** Keeps the first `max` picks of a list. */
  private trim(set: PickSet, max: number): void {
    if (set().size > max) {
      set.set(new Set([...set()].slice(0, max)));
    }
  }

  /** Picks or drops `key`: with a single place the pick replaces, otherwise a full list refuses. */
  private flip(set: PickSet, key: string, max: number): void {
    const now = set();
    if (now.has(key)) {
      const next = new Set(now);
      next.delete(key);
      set.set(next);
    } else if (max === 1) {
      set.set(new Set([key]));
    } else if (now.size < max) {
      set.set(new Set([...now, key]));
    }
  }

  toggleCantrip(key: string): void {
    this.flip(this.cantrips, key, this.cantripsAsked());
  }

  toggleSpell(key: string): void {
    // Magical Secrets: only so many of the new spells may come from outside the class list.
    const item = this.spellItems().find((i) => i.key === key);
    if (item?.disabled && !this.spells().has(key)) {
      return;
    }
    this.flip(this.spells, key, this.spellsAsked());
    // A spell dropped from the book cannot stay prepared.
    const book = new Set([...this.have.known, ...this.spells()]);
    if (this.options.spellsKind === LevelUpSpellsKind.SPELLBOOK) {
      this.prepared.set(new Set([...this.prepared()].filter((k) => book.has(k))));
    }
  }

  togglePrepared(key: string): void {
    this.flip(this.prepared, key, this.preparedAsked());
  }

  toggleSkill(key: string): void {
    this.flip(this.skills, key, this.skillsAsked());
    // Expertise only in what is trained.
    const trained = new Set([...this.have.skills, ...this.skills()]);
    this.expertise.set(new Set([...this.expertise()].filter((k) => trained.has(k))));
  }

  toggleExpertise(key: string): void {
    this.flip(this.expertise, key, this.expertiseAsked());
  }

  /** A feature option: each choice (a fighting style, the metamagic) takes its own `choose`. */
  toggleFeature(key: string, group: readonly string[], choose: number): void {
    const now = this.features();
    if (now.has(key)) {
      this.flip(this.features, key, choose);
      return;
    }
    const inGroup = [...now].filter((k) => group.includes(k));
    if (choose === 1) {
      this.features.set(new Set([...now].filter((k) => !group.includes(k)).concat(key)));
    } else if (inGroup.length < choose) {
      this.features.set(new Set([...now, key]));
    }
  }
}
