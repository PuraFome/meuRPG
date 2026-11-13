import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';

import { AttunementClient, type MagicItemOptionVm } from '../../../core/characters/attunement';
import { SheetFrame } from '../../../shared/sheet/sheet-frame/sheet-frame';
import { injectSheet, openSheet } from '../../../shared/sheet/sheet-host';

export interface MagicItemPickerData {
  readonly campaignId: string;
  /** Adds the item (a catalog key, never a family) as a full-sheet save; answers the refusal in words, or '' when added. */
  readonly add: (key: string) => Promise<string>;
}

/** Opens "Adicionar item mágico": a bottom sheet under 768 px, a 480 px dialog from there; it answers `true` when added. */
export function openMagicItemPicker(
  dialog: MatDialog,
  bottomSheet: MatBottomSheet,
  data: MagicItemPickerData,
) {
  return openSheet<MagicItemPickerSheet, MagicItemPickerData, boolean>(
    dialog,
    bottomSheet,
    MagicItemPickerSheet,
    {
      data,
      ariaLabel: 'Adicionar item mágico',
      labelledBy: 'sheet-t',
      width: '480px',
      tall: true,
      focus: '#magic-item-search',
    },
  );
}

/** The search waits for the typing to pause. */
const SEARCH_DELAY_MS = 250;

const RARITY: Record<string, string> = {
  common: 'comum',
  uncommon: 'incomum',
  rare: 'raro',
  very_rare: 'muito raro',
  legendary: 'lendário',
  artifact: 'artefato',
  varies: 'varia',
};
const CATEGORY: Record<string, string> = {
  armor: 'Armadura',
  weapon: 'Arma',
  ammunition: 'Munição',
  potion: 'Poção',
  scroll: 'Pergaminho',
  ring: 'Anel',
  rod: 'Bastão',
  staff: 'Cajado',
  wand: 'Varinha',
  'wondrous-item': 'Item maravilhoso',
};

/**
 * "Adicionar item mágico": the search asks the server (`ListMagicItems`), each row is a radio card (name, category,
 * rarity, "N variantes", "Pede sintonização · Só para …"), a family asks for its variant in a select, and "Adicionar
 * item" stays off until the choice names one item. The +N armor and weapons are not here: they come from the piece's menu.
 */
@Component({
  selector: 'app-magic-item-picker-sheet',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatButtonModule, MatIconModule, SheetFrame],
  templateUrl: './magic-item-picker-sheet.html',
  styleUrl: './magic-item-picker-sheet.scss',
})
export class MagicItemPickerSheet {
  private readonly client = inject(AttunementClient);
  private readonly sheet = injectSheet<MagicItemPickerData, boolean>();
  protected readonly data = this.sheet.data;
  protected readonly inSheet = this.sheet.inSheet;

  protected readonly query = signal('');
  protected readonly items = signal<readonly MagicItemOptionVm[]>([]);
  protected readonly loading = signal(true);
  protected readonly loadError = signal(false);
  protected readonly chosen = signal<string | null>(null);
  protected readonly variant = signal('');
  protected readonly busy = signal(false);
  protected readonly error = signal('');
  private timer: ReturnType<typeof setTimeout> | undefined;
  private asked = 0;

  protected readonly chosenItem = computed(
    () => this.items().find((i) => i.key === this.chosen()) ?? null,
  );
  /** The catalog key the save adds: the item itself, or the family's chosen variant; null until it names one item. */
  protected readonly key = computed(() => {
    const item = this.chosenItem();
    if (!item) {
      return null;
    }
    if (item.variants.length === 0) {
      return item.key;
    }
    return item.variants.some((v) => v.key === this.variant()) ? this.variant() : null;
  });

  constructor() {
    void this.load('');
  }

  protected words(i: MagicItemOptionVm): string {
    return `${CATEGORY[i.category] ?? i.category} · ${RARITY[i.rarity] ?? i.rarity}`;
  }

  protected asksLine(i: MagicItemOptionVm): string {
    const restriction = i.variants.find((v) => v.restrictionPt)?.restrictionPt ?? i.restrictionPt;
    return restriction ? `Pede sintonização · ${restriction}` : 'Pede sintonização';
  }

  /** A family asks for attunement when any of its variants does; the line is the same under the card. */
  protected asks(i: MagicItemOptionVm): boolean {
    return i.asks || i.variants.some((v) => v.asks);
  }

  protected typed(text: string): void {
    this.query.set(text);
    clearTimeout(this.timer);
    this.timer = setTimeout(() => void this.load(text), SEARCH_DELAY_MS);
  }

  protected choose(key: string): void {
    if (this.chosen() !== key) {
      this.variant.set('');
    }
    this.chosen.set(key);
    this.error.set('');
  }

  private async load(query: string): Promise<void> {
    const mine = ++this.asked;
    this.loading.set(true);
    this.loadError.set(false);
    try {
      const page = await this.client.listMagicItems(this.data.campaignId, query.trim());
      if (mine === this.asked) {
        this.items.set(page.items);
      }
    } catch {
      if (mine === this.asked) {
        this.loadError.set(true);
      }
    } finally {
      if (mine === this.asked) {
        this.loading.set(false);
      }
    }
  }

  protected retry(): void {
    void this.load(this.query());
  }

  protected async add(): Promise<void> {
    const key = this.key();
    if (this.busy() || key === null) {
      return;
    }
    this.busy.set(true);
    this.error.set('');
    try {
      const why = await this.data.add(key);
      if (why) {
        this.error.set(why);
      } else {
        this.sheet.close(true);
      }
    } finally {
      this.busy.set(false);
    }
  }

  protected cancel(): void {
    if (!this.busy()) {
      this.sheet.close();
    }
  }
}
