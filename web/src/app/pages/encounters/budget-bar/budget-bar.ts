import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

import { EncounterBand, type EncounterEvaluation } from '../../../../gen/meurpg/play/v1/encounters_pb';
import { bandLimit, bandWord, barGeometry } from '../../../core/encounters/encounter-text';
import { formatInt, tight } from '../../../core/format/text';

interface BandMark {
  readonly band: EncounterBand;
  readonly word: string;
  readonly limit: string;
  /** Where the label sits on the bar, in percent (the middle of its stretch). */
  readonly at: number;
  readonly on: boolean;
}

/**
 * The difficulty bar (E10-09 states 1 to 3 and 8): the three budgets are ticks above the bar, the encounter's total is the
 * filled part with its marker, and the band it falls in is a word in an outlined pill, never colour alone and never
 * "mortal" (RN-29): past the high budget the bar is full, the marker sits at the tip with "›" and the band reads "Acima
 * de alta". Every number is the server's (`EvaluateEncounter`); this draws them. A bar of 330 px cannot hold three
 * labels, so a phone gets the marker above the bar and the bands as a list below it ("Baixa  até 1.250 XP", the current
 * one in bold with a dot).
 */
@Component({
  selector: 'app-budget-bar',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (evaluation(); as ev) {
      <div class="bar" [class.bar--stale]="stale()">
        <div class="bar__figure" role="img" [attr.aria-label]="summary()">
          <div class="bar__top" aria-hidden="true">
            @for (t of ticks(); track t.xp) {
              <span class="bar__budget" [style.left.%]="t.at">{{ t.label }}</span>
            }
            <span class="bar__marker" [class.bar__marker--end]="geo().over" [style.left.%]="markerAt()">
              {{ geo().over ? '›' : '' }} {{ total() }}
              <svg viewBox="0 0 10 6" aria-hidden="true"><path d="M0 0h10L5 6z" /></svg>
            </span>
          </div>
          <div class="bar__track" aria-hidden="true">
            <span class="bar__fill" [style.width.%]="geo().fill"></span>
            @for (t of ticks(); track t.xp) {
              <span class="bar__tick" [style.left.%]="t.at"></span>
            }
          </div>
          <div class="bar__words" aria-hidden="true">
            @for (b of marks(); track b.band) {
              <span class="bar__word" [class.bar__word--on]="b.on" [style.left.%]="b.at">{{ b.word }}</span>
            }
          </div>
        </div>
        <ul class="bands">
          @for (b of marks(); track b.band) {
            <li class="band" [class.band--on]="b.on" [attr.aria-current]="b.on ? 'true' : null">
              <span class="band__dot" aria-hidden="true"></span>
              <span class="band__w">{{ b.word }}</span>
              <span class="band__l">{{ b.limit }}</span>
            </li>
          }
        </ul>
      </div>
    }
  `,
  styleUrl: './budget-bar.scss',
})
export class BudgetBar {
  readonly evaluation = input.required<EncounterEvaluation | null>();
  /** A newer measure is on its way: the bar dims a little. */
  readonly stale = input(false);

  protected readonly geo = computed(() => {
    const ev = this.evaluation();
    return ev ? barGeometry(ev) : { low: 0, moderate: 0, high: 0, fill: 0, over: false };
  });
  protected readonly total = computed(() => tight(`${formatInt(this.evaluation()?.totalXp ?? 0)} XP`));
  /** The marker keeps clear of the bar's two ends, so its label never leaves the card. */
  protected readonly markerAt = computed(() => Math.min(94, Math.max(6, this.geo().over ? 100 : this.geo().fill)));
  protected readonly ticks = computed(() => {
    const ev = this.evaluation();
    const g = this.geo();
    return ev
      ? [
          { xp: ev.budget?.low ?? 0, at: g.low, label: formatInt(ev.budget?.low ?? 0) },
          { xp: ev.budget?.moderate ?? 0, at: g.moderate, label: formatInt(ev.budget?.moderate ?? 0) },
          { xp: ev.budget?.high ?? 0, at: g.high, label: formatInt(ev.budget?.high ?? 0) },
        ]
      : [];
  });
  protected readonly marks = computed<BandMark[]>(() => {
    const ev = this.evaluation();
    if (!ev) {
      return [];
    }
    const g = this.geo();
    const stretch: [EncounterBand, number, number][] = [
      [EncounterBand.LOW, 0, g.low],
      [EncounterBand.MODERATE, g.low, g.moderate],
      [EncounterBand.HIGH, g.moderate, g.high],
      [EncounterBand.ABOVE_HIGH, g.high, 100],
    ];
    return stretch.map(([band, from, to]) => ({
      band,
      word: bandWord(band),
      limit: bandLimit(ev, band),
      at: (from + to) / 2,
      on: ev.band === band,
    }));
  });
  protected readonly summary = computed(() => {
    const ev = this.evaluation();
    if (!ev) {
      return '';
    }
    return `Dificuldade ${bandWord(ev.band)}: ${formatInt(ev.totalXp)} XP. Orçamentos: baixa até ${formatInt(ev.budget?.low ?? 0)}, moderada até ${formatInt(ev.budget?.moderate ?? 0)}, alta até ${formatInt(ev.budget?.high ?? 0)} XP.`;
  });
}
