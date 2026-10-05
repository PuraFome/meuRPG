import { Component, computed, inject, signal, viewChild } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { XpMode } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import type { TreasureToConvert } from '../../../gen/meurpg/progression/v1/progression_pb';
import { newKey } from '../../core/connect/idempotency';
import { formatInt } from '../../core/format/text';
import type { ExperienceRow } from '../../core/progression/experience-store';
import { ProgressionClient } from '../../core/progression/progression-client';
import { foundLine, po, totalPo, townCalc, treasureCount } from '../../core/progression/treasure';
import { townErrorMessage, xpBlocked } from '../../core/progression/xp-errors';
import { eachLine } from '../../core/progression/xp-math';
import { SheetFrame } from '../../pages/live-session/combat/sheet-frame/sheet-frame';
import { injectSheet } from '../../pages/live-session/combat/sheet-host';
import { CheckBox } from '../check-box/check-box';
import type { AwardXpResult } from './award-xp-sheet';
import { XpActions } from './xp-actions';

/** What the caller hands "Voltar à cidade": the treasures it already knows
 * (the sheet reads them again as it opens) and who is alive. */
export interface TownData {
  readonly campaignId: string;
  readonly xpMode: XpMode;
  readonly rows: readonly ExperienceRow[];
  readonly treasures: readonly TreasureToConvert[];
  readonly total: number;
}

/** What the award is called in the history when the master does not write a
 * reason (the dialog has no field for it): the title line is made from the
 * treasures, so this is only what the server stores. */
const TOWN_REASON = 'Voltar à cidade';

/**
 * "Voltar à cidade" (E9-09, MR-041, RN-09, RN-10): converts the found, not
 * converted treasures into one XP award of a campaign by gold, 1 XP per PO.
 * A dialog on a desktop and a bottom sheet on a phone (`openSheet`), in the
 * shared `sheet-frame`, so "Cancelar" and the one filled button, which says the
 * number ("Dar 105 XP para cada"), are always in reach.
 *
 * - Every treasure and every living character comes checked; unchecking
 *   changes the calculation, written out and read by a polite status ("420 XP
 *   ÷ 4 = 105 XP para cada", and what is left over).
 * - The server sums the PO and splits; the line here is only the preview.
 * - A refusal stays in the sheet, in words, and the list is read again: a
 *   treasure that was converted or unmarked meanwhile leaves it, the rest of
 *   the choice stays.
 * - One idempotency key per set of choices, as "Dar XP".
 */
@Component({
  selector: 'app-town-sheet',
  imports: [CheckBox, MatIconModule, SheetFrame, XpActions],
  templateUrl: './town-sheet.html',
  styleUrl: './town-sheet.scss',
})
export class TownSheet {
  private readonly api = inject(ProgressionClient);
  private readonly sheet = injectSheet<TownData, AwardXpResult | undefined>();
  protected readonly data = this.sheet.data;
  protected readonly inSheet = this.sheet.inSheet;

  protected readonly treasures = signal<readonly TreasureToConvert[]>(this.data.treasures);
  private readonly totalFound = signal(this.data.total);
  private known = new Set(this.data.treasures.map((t) => t.pointId));
  protected readonly checkedTreasures = signal<ReadonlySet<string>>(new Set(this.known));
  protected readonly checkedPeople = signal<ReadonlySet<string>>(new Set(this.data.rows.map((r) => r.id)));
  protected readonly busy = signal(false);
  protected readonly error = signal('');
  private readonly frame = viewChild.required(SheetFrame);

  protected readonly chosen = computed(() => this.treasures().filter((t) => this.checkedTreasures().has(t.pointId)));
  protected readonly calc = computed(() => townCalc(this.chosen().length, totalPo(this.chosen()), this.checkedPeople().size));
  /** Treasures the list does not hold (it stops at 100). */
  protected readonly more = computed(() => Math.max(0, this.totalFound() - this.treasures().length));

  /** Why the filled button waits, in words, or empty when it can go. */
  protected readonly waiting = computed(() => {
    if (this.chosen().length === 0 || this.checkedPeople().size === 0) {
      return 'Marque pelo menos um tesouro e um personagem.';
    }
    if (this.calc().split.each === 0) {
      return 'O total é pequeno demais: cada um precisa receber pelo menos 1 XP.';
    }
    return '';
  });
  protected readonly primaryLabel = computed(() =>
    this.waiting() ? 'Dar XP' : `Dar ${eachLine(this.calc().split)}`,
  );
  protected readonly announcement = computed(() => this.waiting() || `${eachLine(this.calc().split)}.`);

  protected readonly foundLine = (t: TreasureToConvert) => foundLine(t);
  protected readonly po = po;
  protected readonly count = treasureCount;
  protected readonly formatInt = formatInt;

  private key = newKey();
  private keyFor = '';

  constructor() {
    // The master may have marked or converted treasures since the host read them.
    void this.reload();
  }

  protected toggleTreasure(id: string): void {
    this.checkedTreasures.update((set) => flip(set, id));
  }

  protected togglePerson(id: string): void {
    this.checkedPeople.update((set) => flip(set, id));
  }

  /** Reads the list again: what left it is unchecked for good, what is new comes checked. */
  private async reload(): Promise<void> {
    try {
      const res = await this.api.listTreasures(this.data.campaignId);
      const now = new Set(res.treasures.map((t) => t.pointId));
      const fresh = res.treasures.filter((t) => !this.known.has(t.pointId)).map((t) => t.pointId);
      this.checkedTreasures.update((set) => new Set([...[...set].filter((id) => now.has(id)), ...fresh]));
      this.known = new Set([...this.known, ...now]);
      this.treasures.set(res.treasures);
      this.totalFound.set(res.total);
    } catch {
      // The list the host gave stays: the server refuses a stale choice anyway.
    }
  }

  protected async give(): Promise<void> {
    if (this.busy() || this.waiting()) {
      return;
    }
    const ids = this.chosen().map((t) => t.pointId);
    const people = this.data.rows.filter((r) => this.checkedPeople().has(r.id)).map((r) => r.id);
    // New choices are a new award; the same ones again are a retry.
    const signature = JSON.stringify([ids, people]);
    if (signature !== this.keyFor) {
      this.keyFor = signature;
      this.key = newKey();
    }
    this.busy.set(true);
    this.error.set('');
    try {
      const res = await this.api.award(
        this.data.campaignId,
        { mode: 'town', treasurePointIds: ids },
        TOWN_REASON,
        people,
        this.key,
      );
      this.sheet.close(res.award ? { award: res.award, xpEach: res.xpEach, lostXp: res.lostXp } : undefined);
    } catch (err) {
      this.error.set(townErrorMessage(err));
      this.frame().scrollToTop();
      if (xpBlocked(err)) {
        await this.reload();
      }
    } finally {
      this.busy.set(false);
    }
  }

  protected cancel(): void {
    this.sheet.close(undefined);
  }
}

function flip(set: ReadonlySet<string>, id: string): ReadonlySet<string> {
  const next = new Set(set);
  if (!next.delete(id)) {
    next.add(id);
  }
  return next;
}
