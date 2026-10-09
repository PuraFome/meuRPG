import {
  Component,
  DOCUMENT,
  DestroyRef,
  computed,
  effect,
  inject,
  input,
  output,
  signal,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { RouterLink } from '@angular/router';

import { DiceMode, DicePreference } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { article } from '../../../core/combat/combat-log';
import { focusWithRing } from '../../../core/creatures/focus-ring';
import { hitDiceLeftWords, totalDiceLeft } from '../../../core/resources/hit-dice-text';
import { openHitDice } from '../hit-dice-sheet/hit-dice-sheet';
import { WildPools } from '../../../shared/wild-shape/wild-pools';
import { PlayerSheetVm, VitalsVm } from '../live-session.types';
import { SlotDots } from '../slot-dots/slot-dots';
import { freeWords, hitPointsPercent, slotLevelLabel, slotRowLabel } from '../vitals';

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
  imports: [MatButtonModule, MatIconModule, RouterLink, SlotDots, WildPools],
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
  /** How the campaign has the players roll their dice and the player's own choice: the hit die sheet follows them (RN-18). */
  readonly diceMode = input<DiceMode>(DiceMode.PLAYERS_CHOOSE);
  readonly dicePreference = input<DicePreference>(DicePreference.APP);
  /** The vitals the server answered after a hit die was spent: the page takes them in like any other change. */
  readonly vitalsChange = output<VitalsVm>();

  private readonly document = inject(DOCUMENT);
  private readonly destroyRef = inject(DestroyRef);
  private readonly dialog = inject(MatDialog);
  private readonly bottomSheet = inject(MatBottomSheet);

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
  protected readonly armorClass = computed(() =>
    this.vitals().wildShape ? this.beastAc() : (this.sheet()?.armorClass ?? null),
  );

  /** "Classe de Armadura", or "CA do Lobo" while a beast (the shield is narrow). */
  protected readonly acLabel = computed(() => {
    const w = this.vitals().wildShape;
    return w
      ? `CA d${article(w.beastNamePt) === 'a' ? 'a' : 'o'} ${w.beastNamePt}`
      : 'Classe de Armadura';
  });

  /** "3 de 5d10 e 1 de 1d6": the dice left by size. */
  protected readonly diceLeft = computed(() => hitDiceLeftWords(this.vitals().hitDiceSizes));
  protected readonly freeWords = freeWords;
  /** "Gastar dados de vida" has nothing to spend when every die is used. */
  protected readonly noDiceLeft = computed(() => totalDiceLeft(this.vitals().hitDiceSizes) === 0);

  /** "Gastar dados de vida": the sheet where a short rest's dice are spent one by one. */
  protected spendHitDice(): void {
    if (this.noDiceLeft()) {
      return;
    }
    const opener = this.document.activeElement as HTMLElement | null;
    openHitDice(this.dialog, this.bottomSheet, {
      campaignId: this.campaignId(),
      vitals: this.vitals,
      diceMode: this.diceMode(),
      preference: this.dicePreference(),
      apply: (v) => this.vitalsChange.emit(v),
    })
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe(() => focusWithRing(opener));
  }

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
