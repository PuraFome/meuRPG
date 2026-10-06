import { Component, ElementRef, afterNextRender, computed, inject, Injector, signal, viewChild } from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { ActivatedRoute, RouterLink } from '@angular/router';

import { XpMode } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { type Treasure, TreasureMode } from '../../../gen/meurpg/maps/v1/treasure_pb';
import { focusWithRing } from '../../core/creatures/focus-ring';
import { type TreasureAccess, TreasureAccessCheck } from '../../core/treasure/treasure-access';
import { TreasureClient } from '../../core/treasure/treasure-client';
import { generateFailure } from '../../core/treasure/treasure-errors';
import { goldLine, modeText, partyHelp, placedSummary, placedXpLine } from '../../core/treasure/treasure-format';
import { NumberStepper } from '../../shared/form-fields/number-stepper';
import { Segmented, type Segment } from '../../shared/segmented/segmented';
import { openSheet } from '../../shared/sheet/sheet-host';
import { ItemSheet, type ItemSheetData } from './item-sheet/item-sheet';
import { PlaceSheet, type PlaceSheetData, type PlaceSheetResult } from './place-sheet/place-sheet';
import { type ItemOpen, TreasureResult } from './treasure-result';

type Party = { livingCount: number; lowestLevel: number; highestLevel: number };
type Generation = { status: 'idle' } | { status: 'busy' } | { status: 'error'; message: string } | { status: 'ready'; treasure: Treasure };

/** What was put on a map, for the confirmation under the result. */
interface Placed {
  readonly mapId: string;
  readonly mapName: string;
  readonly place: string;
  readonly goldPo: number;
  readonly itemCount: number;
}

const MODES: readonly Segment<'individual' | 'hoard'>[] = [
  { value: 'individual', label: 'Individual' },
  { value: 'hoard', label: 'De covil' },
];

const MIN_LEVEL = 1;
const MAX_LEVEL = 20;

/**
 * "/campanhas/:id/tesouro" (MR-044, MR-041, RN-09, RN-10, E10-10, states 1, 2 and 5): the master's treasure generator. The mode
 * (individual or de covil), the party level (from `GetTreasureParty`, the lowest level of the living characters, and editable), then
 * "Gerar tesouro": the server rolls and the page draws it (`app-treasure-result`), with the seed shown. "Gerar outro" asks again
 * without a seed; the same seed, mode and level give the same treasure. "Ver descrição" opens an item (`ItemSheet`) and "Pôr no mapa"
 * makes a hidden treasure point (`PlaceSheet`); the confirmation says what the campaign does with the gold, from the campaign's XP
 * mode. Only the master has the page; a player is told so, and the server answers `not_found` to anyone else.
 */
@Component({
  selector: 'app-treasure',
  imports: [MatButtonModule, MatIconModule, MatProgressSpinnerModule, NumberStepper, RouterLink, Segmented, TreasureResult],
  templateUrl: './treasure.html',
  styleUrl: './treasure.scss',
})
export class TreasurePage {
  private readonly route = inject(ActivatedRoute);
  private readonly api = inject(TreasureClient);
  private readonly accessCheck = inject(TreasureAccessCheck);
  private readonly dialog = inject(MatDialog);
  private readonly bottomSheet = inject(MatBottomSheet);
  private readonly injector = inject(Injector);

  protected readonly campaignId = this.route.snapshot.paramMap.get('id') ?? '';
  protected readonly access = signal<TreasureAccess | { status: 'loading' }>({ status: 'loading' });
  protected readonly party = signal<Party | null>(null);
  protected readonly modes = MODES;
  protected readonly mode = signal<'individual' | 'hoard'>('hoard');
  protected readonly level = signal(MIN_LEVEL);
  protected readonly generation = signal<Generation>({ status: 'idle' });
  protected readonly placed = signal<Placed | null>(null);
  /** A treasure is being rolled: the old result stays on screen with its buttons resting, and the new title takes the focus when it lands. */
  protected readonly rolling = signal(false);
  /** The party could not be read: the page says so, with a way to try again, instead of saying the party is empty. */
  protected readonly partyFailed = signal(false);
  protected readonly minLevel = MIN_LEVEL;
  protected readonly maxLevel = MAX_LEVEL;

  private readonly placedBox = viewChild<ElementRef<HTMLElement>>('placedBox');
  private readonly resultBox = viewChild<ElementRef<HTMLElement>>('resultHost');
  private generationId = 0;

  protected readonly campaignName = computed(() => {
    const a = this.access();
    return a.status === 'master' ? a.campaignName : '';
  });
  protected readonly xpMode = computed(() => {
    const a = this.access();
    return a.status === 'master' ? a.xpMode : XpMode.ENEMIES;
  });
  protected readonly subtitle = computed(() => {
    const p = this.party();
    const who = this.campaignName();
    if (!p || p.livingCount === 0) {
      return who;
    }
    return `${who} · para o grupo de nível ${p.lowestLevel === p.highestLevel ? p.lowestLevel : `${p.lowestLevel} e ${p.highestLevel}`}`;
  });
  protected readonly help = computed(() => (this.partyFailed() ? '' : partyHelp(this.party())));
  protected readonly modeHint = computed(() =>
    this.mode() === 'hoard' ? 'De covil: o tesouro guardado num esconderijo.' : 'Individual: o que uma criatura carrega.',
  );
  protected readonly generated = computed(() => {
    const g = this.generation();
    return g.status === 'ready' ? g.treasure : null;
  });
  protected readonly announce = computed(() => {
    const g = this.generation();
    return g.status === 'ready' ? `${modeText(g.treasure.mode)}: tesouro gerado, semente ${g.treasure.seed}.` : '';
  });
  protected readonly placedSummary = computed(() => {
    const p = this.placed();
    return p ? placedSummary(p.goldPo, p.itemCount, this.generated()?.mode === TreasureMode.HOARD) : '';
  });
  protected readonly placedXp = computed(() => {
    const p = this.placed();
    return p ? placedXpLine(this.xpMode(), this.campaignName(), p.goldPo, p.itemCount) : '';
  });
  protected readonly placedLink = computed(() => {
    const p = this.placed();
    return p ? ['/campanhas', this.campaignId, 'mapas', p.mapId] : null;
  });
  protected readonly goldSentence = computed(() => goldLine(this.xpMode(), this.campaignName()));

  constructor() {
    void this.start();
  }

  protected async start(): Promise<void> {
    this.access.set({ status: 'loading' });
    const access = await this.accessCheck.check(this.campaignId);
    this.access.set(access);
    if (access.status !== 'master') {
      return;
    }
    await this.loadParty();
  }

  protected async loadParty(): Promise<void> {
    this.partyFailed.set(false);
    try {
      const p = await this.api.party(this.campaignId);
      this.party.set({ livingCount: p.livingCount, lowestLevel: p.lowestLevel, highestLevel: p.highestLevel });
      if (p.livingCount > 0 && p.lowestLevel >= MIN_LEVEL) {
        this.level.set(Math.min(MAX_LEVEL, p.lowestLevel));
      }
    } catch {
      // The level stays where it is and the master sets it; the page says the party could not be read.
      this.party.set(null);
      this.partyFailed.set(true);
    }
  }

  protected setMode(value: 'individual' | 'hoard'): void {
    this.mode.set(value);
  }

  protected setLevel(level: number): void {
    this.level.set(Math.min(MAX_LEVEL, Math.max(MIN_LEVEL, level)));
  }

  /** "Gerar tesouro", "Gerar outro" and "Gerar de novo": a new call without a seed, so the server draws one. Generating again replaces the result (the old one stays on screen while the server rolls) and clears the confirmation: nothing is put on a map by it. */
  protected async generate(): Promise<void> {
    if (this.rolling()) {
      return;
    }
    const id = ++this.generationId;
    const again = this.generation().status === 'ready';
    this.rolling.set(true);
    if (!again) {
      this.generation.set({ status: 'busy' });
    }
    try {
      const res = await this.api.generate(this.campaignId, this.mode() === 'hoard' ? TreasureMode.HOARD : TreasureMode.INDIVIDUAL, this.level());
      if (id !== this.generationId) {
        return;
      }
      if (!res.treasure) {
        throw new Error('GenerateTreasure answered without a treasure');
      }
      this.placed.set(null);
      this.generation.set({ status: 'ready', treasure: res.treasure });
      if (again) {
        // The new title is where the person looks next.
        afterNextRender(() => focusWithRing(this.resultBox()?.nativeElement.querySelector<HTMLElement>('.result__title')), { injector: this.injector });
      }
    } catch (err) {
      if (id === this.generationId) {
        this.generation.set({ status: 'error', message: generateFailure(err) });
      }
    } finally {
      if (id === this.generationId) {
        this.rolling.set(false);
      }
    }
  }

  protected openItem(item: ItemOpen): void {
    openSheet<ItemSheet, ItemSheetData, void>(this.dialog, this.bottomSheet, ItemSheet, {
      data: { campaignId: this.campaignId, key: item.key, namePt: item.namePt },
      ariaLabel: item.namePt,
      labelledBy: 'item-t',
      width: '560px',
      tall: true,
      focus: '[data-initial-focus]',
      restoreFocus: false,
    }).subscribe(() => focusWithRing(item.button.isConnected ? item.button : null));
  }

  protected openPlace(): void {
    const treasure = this.generated();
    if (!treasure) {
      return;
    }
    openSheet<PlaceSheet, PlaceSheetData, PlaceSheetResult>(this.dialog, this.bottomSheet, PlaceSheet, {
      data: { campaignId: this.campaignId, treasure, xpMode: this.xpMode(), campaignName: this.campaignName() },
      ariaLabel: 'Pôr no mapa',
      labelledBy: 'place-t',
      width: '1140px',
      tall: true,
      restoreFocus: false,
    }).subscribe((result) => {
      if (result?.kind === 'again') {
        void this.generate();
      } else if (result?.kind === 'placed') {
        this.placed.set({ mapId: result.mapId, mapName: result.mapName, place: result.place, goldPo: result.goldPo, itemCount: result.itemCount });
        // The confirmation is where the person looks next: it comes into view and takes the focus.
        afterNextRender(
          () => {
            const box = this.placedBox()?.nativeElement;
            box?.scrollIntoView?.({ block: 'nearest' });
            focusWithRing(box);
          },
          { injector: this.injector },
        );
      } else {
        focusWithRing(this.resultBox()?.nativeElement.querySelector<HTMLElement>('.act--go'));
      }
    });
  }
}
