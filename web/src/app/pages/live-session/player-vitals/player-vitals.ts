import {
  Component,
  DestroyRef,
  computed,
  effect,
  inject,
  input,
  signal,
  untracked,
} from '@angular/core';
import { MatIconModule } from '@angular/material/icon';
import { RouterLink } from '@angular/router';

import { article } from '../../../core/combat/combat-log';
import {
  SHIELD_ENDED_MS,
  aidBar,
  aidLabel,
  aidTag,
  armorClassWithShield,
  shieldEndedParts,
  shieldLabelForOwner,
  shieldSum,
  sheetMaximum,
  vitalsSpeech,
} from '../../../core/combat/combat-effects';
import { WildPools } from '../../../shared/wild-shape/wild-pools';
import { PlayerSheetVm, VitalsVm } from '../live-session.types';
import { SlotDots } from '../slot-dots/slot-dots';
import { freeWords, hitPointsPercent, slotLevelLabel, slotRowLabel, usedWords } from '../vitals';

interface SlotRowVm {
  readonly key: string;
  readonly label: string;
  readonly total: number;
  readonly used: number;
}

/**
 * The player's own vitals on the session page (RN-02, artboards E5-02 and
 * E5-03): the sheet's combat block with the current values filled in. The
 * PV box first (the number they came for), then the CA shield, temporary
 * HP and hit dice, then the spell slots, a link to the full sheet and who
 * changes these numbers. A player only ever sees their own character
 * (decision row 28): the server sends nothing else.
 */
@Component({
  selector: 'app-player-vitals',
  imports: [MatIconModule, RouterLink, SlotDots, WildPools],
  templateUrl: './player-vitals.html',
  styleUrl: './player-vitals.scss',
})
export class PlayerVitals {
  readonly vitals = input.required<VitalsVm>();
  /** From the character's sheet; `null` until it loads. */
  readonly sheet = input<PlayerSheetVm | null>(null);
  readonly campaignId = input.required<string>();
  /** While the druid is a beast: the armor class of the beast's book, in place of the character's own (the server sends none). */
  readonly beastAc = input<number | null>(null);
  /** The combat's version (E6-05): the PV box and the shield side by side,
   * then the slots; no temporary HP, hit dice or footer. */
  readonly compact = input(false);
  /** The Escudo Arcano's bonus on the character's own combatant while a combat runs (0 without the spell); `null`
   * when there is no combat or no combatant, which is not the shield ending. */
  readonly armorClassBonus = input<number | null>(null);

  /** The two reserves of a druid in a beast form, the beast's first: they take the place of the hit points box. */
  protected readonly pools = computed(() => {
    const v = this.vitals();
    const w = v.wildShape;
    if (!w) {
      return null;
    }
    const of = (name: string) => `PV d${article(name) === 'a' ? 'a' : 'o'} ${name}`;
    return {
      beast: { label: of(w.beastNamePt), current: w.hitPointsCurrent, max: w.hitPointsMax },
      character: { label: of(v.name), current: v.hitPointsCurrent, max: v.hitPointsMax },
    };
  });
  /** The armor class on the shield: the beast's while it is one, else the sheet's. */
  private readonly baseArmorClass = computed(() =>
    this.vitals().wildShape ? this.beastAc() : (this.sheet()?.armorClass ?? null),
  );
  /** What the shield shows: the armor class already summed with the Escudo Arcano ("18"). */
  protected readonly armorClass = computed(() => {
    const base = this.baseArmorClass();
    return base === null ? null : armorClassWithShield(base, this.shieldBonus());
  });
  protected readonly shieldBonus = computed(() => this.armorClassBonus() ?? 0);
  /** The small sum under the shield: "13 + 5"; empty without the spell. */
  protected readonly shieldSum = computed(() => {
    const base = this.baseArmorClass();
    return base !== null && this.shieldBonus() > 0 ? shieldSum(base, this.shieldBonus()) : '';
  });
  protected readonly shieldLabel = shieldLabelForOwner;

  /** Ajuda's bonus: 0 without it. */
  protected readonly aid = computed(() => Math.max(0, this.vitals().hitPointsMaxBonus ?? 0));
  protected readonly aidTag = aidTag;
  protected readonly aidLabel = aidLabel;
  protected readonly sheetMaximum = computed(() =>
    sheetMaximum(this.vitals().hitPointsMax, this.aid()),
  );
  protected readonly speech = computed(() => {
    const v = this.vitals();
    return vitalsSpeech(v.hitPointsCurrent, v.hitPointsMax, this.aid());
  });
  protected readonly barLabel = computed(() => {
    const v = this.vitals();
    return `${v.hitPointsCurrent} de ${v.hitPointsMax} pontos de vida`;
  });
  protected readonly bar = computed(() => {
    const v = this.vitals();
    return aidBar(v.hitPointsCurrent, v.hitPointsMax, this.aid());
  });
  /** The sentence of a shield that just ended, for a few seconds. */
  protected readonly shieldEnded = signal<{ lead: string; rest: string } | null>(null);
  private endedTimer: ReturnType<typeof setTimeout> | undefined;
  private previousBonus: number | null = null;

  /** "Classe de Armadura", or "CA do Lobo" while a beast (the shield is narrow). */
  protected readonly acLabel = computed(() => {
    const w = this.vitals().wildShape;
    return w
      ? `CA d${article(w.beastNamePt) === 'a' ? 'a' : 'o'} ${w.beastNamePt}`
      : 'Classe de Armadura';
  });

  protected readonly usedWords = usedWords;
  protected readonly freeWords = freeWords;
  protected readonly percent = computed(() => hitPointsPercent(this.vitals()));

  protected readonly slotRows = computed<SlotRowVm[]>(() => {
    const v = this.vitals();
    const rows: SlotRowVm[] = v.spellSlots.map((s) => ({
      key: `level-${s.level}`,
      label: slotLevelLabel(s.level),
      total: s.total,
      used: s.used,
    }));
    if (v.pactSlots) {
      rows.push({
        key: 'pact',
        label: `Pacto, ${slotLevelLabel(v.pactSlots.slotLevel)}`,
        total: v.pactSlots.total,
        used: v.pactSlots.used,
      });
    }
    return rows;
  });

  protected readonly slotRowLabel = slotRowLabel;

  /** Said once by screen readers when the master changes a number: the
   * screen changes under the player's eyes, and this says what changed. */
  protected readonly announcement = signal('');
  private previous: VitalsVm | null = null;

  constructor() {
    inject(DestroyRef).onDestroy(() => clearTimeout(this.endedTimer));
    // The shield that the combat takes away (the bonus goes from above 0 to 0 on this player's own combatant) is said once,
    // politely, and goes away by itself; a combat that ended or a combatant that is gone is not "the shield ended".
    effect(() => {
      const bonus = this.armorClassBonus();
      const before = this.previousBonus;
      this.previousBonus = bonus;
      if (before !== null && before > 0 && bonus === 0) {
        this.shieldEnded.set(shieldEndedParts(untracked(() => this.baseArmorClass())));
        clearTimeout(this.endedTimer);
        this.endedTimer = setTimeout(() => this.shieldEnded.set(null), SHIELD_ENDED_MS);
      }
    });
    effect(() => {
      const v = this.vitals();
      const before = this.previous;
      this.previous = v;
      if (!before || before.characterId !== v.characterId || before.revision === v.revision) {
        return;
      }
      this.announcement.set(
        `O mestre ajustou: ${v.hitPointsCurrent} de ${v.hitPointsMax} pontos de vida, ` +
          `${v.hitPointsTemporary} temporários.`,
      );
    });
  }
}
