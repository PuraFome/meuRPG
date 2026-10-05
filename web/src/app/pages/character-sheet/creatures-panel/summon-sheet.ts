import { Component, computed, effect, inject, signal, untracked } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { FormsModule } from '@angular/forms';
import type { Creature, CreatureSummary } from '../../../../gen/meurpg/rules/v1/rules_pb';
import { circleLabel } from '../../../core/combat/combat-grid';
import { type SlotRow, freeText } from '../../../core/combat/cast-flow';
import { newKey } from '../../../core/connect/idempotency';
import { CreaturesClient } from '../../../core/creatures/creatures-client';
import { creatureErrorMessage } from '../../../core/creatures/creature-errors';
import {
  CREATURE_NAME_MAX,
  formSubtitle,
  nameCounter,
  summarySubtitle,
} from '../../../core/creatures/creature-format';
import type { SummonCastVm } from '../../../core/creatures/summon-access';
import { beastOptions, summonSpell, undeadCount } from '../../../core/creatures/summon-spells';
import { tight } from '../../../core/format/text';
import { SlotPicker } from '../../live-session/combat/cast-sheet/slot-picker';
import { SheetFrame } from '../../live-session/combat/sheet-frame/sheet-frame';
import { injectSheet } from '../../live-session/combat/sheet-host';
import { type ChoiceRow, CreatureChoiceList } from '../../../shared/creatures/creature-choice-list';

/** What the panel hands the sheet. */
export interface SummonSheetData {
  readonly campaignId: string;
  readonly characterId: string;
  readonly cast: SummonCastVm;
  /** The name of the familiar the character has now, which a new one replaces. */
  readonly replaces: string;
}

/** What the sheet answers when it cast: for the panel's live notice. */
export interface SummonSheetResult {
  readonly spellName: string;
  readonly ritual: boolean;
  readonly names: readonly string[];
  readonly replaced: boolean;
}

/**
 * Casting a summoning spell outside a combat (E9-10, quadro 2): Encontrar
 * Familiar as a ritual (no slot: the name, then the 15 forms of the book, and
 * the Pact of the Chain's four), Animar os Mortos (the slot decides how many
 * undead, one kind for them all) and Conjurar Animais (the slot, one of the
 * four options with the count the slot makes, and the beast). The same frame
 * as the combat's sheets: the title and the footer (what it costs, the filled
 * button, "Cancelar") never scroll away, the list scrolls between them.
 *
 * The browser only offers what the table of `summon-spells.ts` lists; every
 * choice is checked by `CastSummon`, and a refusal stays here, in words, with
 * the sheet open. The key of the cast is made once and made anew whenever a
 * choice changes, so a tap repeated after a lost answer never casts twice.
 */
@Component({
  selector: 'app-summon-sheet',
  imports: [CreatureChoiceList, FormsModule, MatButtonModule, MatFormFieldModule, MatIconModule, MatInputModule, SheetFrame, SlotPicker],
  templateUrl: './summon-sheet.html',
  styleUrl: './summon-sheet.scss',
})
export class SummonSheet {
  private readonly client = inject(CreaturesClient);
  private readonly sheet = injectSheet<SummonSheetData, SummonSheetResult>();
  protected readonly data = this.sheet.data;
  protected readonly inSheet = this.sheet.inSheet;
  protected readonly def = summonSpell(this.data.cast.key)!;
  protected readonly max = CREATURE_NAME_MAX;
  protected readonly nameCounter = nameCounter;

  protected readonly name = signal(this.data.replaces);
  protected readonly slotLevel = signal(0);
  protected readonly slotRows = signal<readonly SlotRow[] | null>(null);
  protected readonly option = signal(0);
  protected readonly chosen = signal('');
  protected readonly forms = signal<readonly Creature[] | null>(null);
  protected readonly beasts = signal<readonly CreatureSummary[] | null>(null);
  protected readonly busy = signal(false);
  protected readonly error = signal('');
  private key = newKey();

  /** A ritual spends no slot; otherwise the slot picker opens the sheet. */
  protected readonly ritual = this.data.cast.ritual;
  protected readonly kind = this.def.kind;

  protected readonly subtitle = computed(() => {
    const ritual = this.ritual && this.def.kind === 'familiar' ? ' · ritual' : '';
    return `Magia de ${circleLabel(this.def.level)}${ritual} · ${this.def.time}`;
  });

  protected readonly slot = computed(() => this.slotRows()?.find((r) => r.level === this.slotLevel()) ?? null);
  /** The circle the cast is made at: the slot's, or the spell's own for a ritual. */
  protected readonly circle = computed(() => (this.ritual ? this.def.level : this.slotLevel()));

  protected readonly options = computed(() => beastOptions(this.circle() || this.def.level));
  protected readonly count = computed(() => {
    if (this.kind === 'undead') {
      return undeadCount(this.circle() || this.def.level);
    }
    if (this.kind === 'beasts') {
      return this.options()[this.option()]?.count ?? 1;
    }
    return 1;
  });

  protected readonly rows = computed<readonly ChoiceRow[]>(() => {
    if (this.kind === 'beasts') {
      return (this.beasts() ?? []).map((s) => ({ key: s.key, title: s.namePt, subtitle: summarySubtitle(s) }));
    }
    const list = (this.forms() ?? []).filter((c) => c.summary);
    return [...list]
      .sort((a, b) => (a.summary!.namePt).localeCompare(b.summary!.namePt, 'pt-BR'))
      .map((c) => ({ key: c.summary!.key, title: c.summary!.namePt, subtitle: formSubtitle(c) }));
  });
  protected readonly loadingRows = computed(() => (this.kind === 'beasts' ? this.beasts() === null : this.forms() === null));

  protected readonly chosenRow = computed(() => this.rows().find((r) => r.key === this.chosen()) ?? null);

  /** What is still missing, in words; empty when the cast is ready. */
  protected readonly missing = computed(() => {
    if (!this.ritual && this.slotLevel() === 0) {
      return 'Falta escolher o espaço de magia.';
    }
    if (this.kind === 'familiar' && this.name().trim() === '') {
      return 'Falta dar um nome ao familiar.';
    }
    if (!this.chosenRow()) {
      return this.kind === 'beasts' ? 'Falta escolher o animal.' : this.kind === 'undead' ? 'Falta escolher o morto-vivo.' : 'Falta escolher a forma.';
    }
    return '';
  });
  protected readonly ready = computed(() => this.missing() === '' && !this.busy());

  /** "Conjurar como ritual · 1 hora · sem gastar espaço", or what the slot costs. */
  protected readonly costLine = computed(() => {
    if (this.ritual) {
      return `Conjurar como ritual · ${this.def.time} · sem gastar espaço`;
    }
    const slot = this.slot();
    const spend = slot ? `gasta um espaço de ${circleLabel(slot.level)}` : 'gasta um espaço de magia';
    const made = this.kind === 'familiar' ? '' : `${this.count() === 1 ? '1 criatura' : this.count() + ' criaturas'} · `;
    return tight(`${made}${this.def.time} · ${spend}`);
  });

  protected readonly buttonLabel = computed(() => {
    const n = this.name().trim();
    if (this.kind === 'familiar') {
      return n ? `Convocar ${n}` : 'Convocar o familiar';
    }
    return this.kind === 'undead' ? `Animar ${this.count() === 1 ? '1 morto-vivo' : this.count() + ' mortos-vivos'}` : 'Conjurar os animais';
  });

  protected readonly beastCr = computed(() => this.options()[this.option()]?.maxCr ?? '');

  constructor() {
    // The forms, and the slots, are read once when the sheet opens.
    if (this.kind === 'beasts') {
      effect(() => {
        const cr = this.beastCr();
        untracked(() => this.loadBeasts(cr));
      });
    } else {
      void this.loadForms();
    }
    if (!this.ritual) {
      void this.loadSlots();
    }
  }

  private async loadForms(): Promise<void> {
    const keys = [...this.def.forms, ...(this.data.cast.chain ? this.def.extraForms : [])];
    try {
      this.forms.set(await Promise.all(keys.map((k) => this.client.statBlock(this.data.campaignId, k))));
    } catch (err) {
      this.forms.set([]);
      this.error.set(creatureErrorMessage(err, 'read'));
    }
  }

  private async loadBeasts(maxCr: string): Promise<void> {
    this.beasts.set(null);
    this.chosen.set('');
    try {
      this.beasts.set((await this.client.search(this.data.campaignId, { type: 'beast', maxCr: maxCr === '' ? undefined : maxCr })).creatures.filter((s) => s.challengeRating !== '' && withinCr(s.challengeRating, maxCr)));
    } catch (err) {
      this.beasts.set([]);
      this.error.set(creatureErrorMessage(err, 'read'));
    }
  }

  private async loadSlots(): Promise<void> {
    try {
      const vitals = await this.client.vitalsOf(this.data.campaignId, this.data.characterId);
      const rows: SlotRow[] = (vitals?.spellSlots ?? [])
        .filter((s) => s.level >= this.def.level && s.total > 0)
        .map((s) => {
          const free = s.total - s.used;
          return { level: s.level, pact: false, free, total: s.total, used: s.used, enabled: free > 0, title: circleLabel(s.level), count: freeText(free, s.total) };
        });
      this.slotRows.set(rows);
      const first = rows.find((r) => r.enabled);
      if (first) {
        this.slotLevel.set(first.level);
      }
    } catch (err) {
      this.slotRows.set([]);
      this.error.set(creatureErrorMessage(err, 'cast'));
    }
  }

  protected pickSlot(row: SlotRow): void {
    this.slotLevel.set(row.level);
    this.changed();
  }

  protected pickOption(index: number): void {
    this.option.set(index);
    this.changed();
  }

  protected pick(key: string): void {
    this.chosen.set(key);
    this.changed();
  }

  protected setName(value: string): void {
    this.name.set(value);
    this.changed();
  }

  /** A choice changed: the next cast is a new one (a new key). */
  protected changed(): void {
    this.key = newKey();
    this.error.set('');
  }

  protected async cast(): Promise<void> {
    if (!this.ready()) {
      return;
    }
    this.busy.set(true);
    this.error.set('');
    const count = this.count();
    const names = this.kind === 'familiar' ? [this.name().trim()] : [];
    try {
      const res = await this.client.castSummon({
        campaignId: this.data.campaignId,
        characterId: this.data.characterId,
        spellKey: this.def.key,
        ritual: this.ritual,
        slot: this.ritual ? undefined : { level: this.slotLevel(), pact: false },
        summon: {
          option: this.kind === 'beasts' ? this.option() : 0,
          creatureKeys: Array.from({ length: count }, () => this.chosen()),
          names,
        },
        idempotencyKey: this.key,
      });
      this.sheet.close({ spellName: this.def.name, ritual: this.ritual, names, replaced: res.replacedIds.length > 0 });
    } catch (err) {
      this.error.set(creatureErrorMessage(err, 'cast'));
    } finally {
      this.busy.set(false);
    }
  }

  protected close(): void {
    this.sheet.close();
  }
}

/** Whether a challenge rating ("1/4", "2") is at most `max`: the list is already cut by the server, this only keeps
 * a stale answer from showing a stronger beast. */
function withinCr(cr: string, max: string): boolean {
  const value = (s: string): number => {
    const [a, b] = s.split('/');
    return b ? Number(a) / Number(b) : Number(a);
  };
  return max === '' || value(cr) <= value(max);
}
