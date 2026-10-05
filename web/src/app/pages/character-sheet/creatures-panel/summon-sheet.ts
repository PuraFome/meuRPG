import { create } from '@bufbuild/protobuf';
import { Component, computed, effect, inject, signal, untracked, viewChild } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';

import { type GetSummonOptionsResponse, GetSummonOptionsResponseSchema, type SummonOption, SummonSlot, SummonSpellOptions } from '../../../../gen/meurpg/characters/v1/characters_pb';
import type { Creature, CreatureSummary } from '../../../../gen/meurpg/rules/v1/rules_pb';
import { type SlotRow, freeText } from '../../../core/combat/cast-flow';
import { circleLabel } from '../../../core/combat/combat-grid';
import { newKey } from '../../../core/connect/idempotency';
import { CreaturesClient } from '../../../core/creatures/creatures-client';
import { creatureErrorMessage } from '../../../core/creatures/creature-errors';
import { CREATURE_NAME_MAX, formSubtitle, nameCounter, summarySubtitle } from '../../../core/creatures/creature-format';
import { FIND_FAMILIAR, castVerb, creaturesText, replacesText } from '../../../core/creatures/summon-labels';
import { tight } from '../../../core/format/text';
import { SlotPicker } from '../../live-session/combat/cast-sheet/slot-picker';
import { SheetFrame } from '../../live-session/combat/sheet-frame/sheet-frame';
import { injectSheet } from '../../live-session/combat/sheet-host';
import { type ChoiceRow, CreatureChoiceList } from '../../../shared/creatures/creature-choice-list';

/** What the panel hands the sheet. */
export interface SummonSheetData {
  readonly campaignId: string;
  readonly characterId: string;
  readonly spellKey: string;
}

/** What the sheet answers when it cast: for the panel's live notice. */
export interface SummonSheetResult {
  readonly spellName: string;
  readonly ritual: boolean;
  readonly castingTime: string;
  readonly names: readonly string[];
  readonly count: number;
  readonly dismissed: number;
}

/**
 * Casting a summoning spell outside a combat (E9-10, quadro 2). The sheet reads what the character
 * can do from the server (`GetSummonOptions`: the slots, what each circle may bring, what a casting
 * would send away) and keeps no rule of its own: a ritual when the server says the character can
 * cast it as one, otherwise the slot picker; the options and the creatures of the circle; a mix
 * of kinds when the option counts several creatures ("−" 1 "+" for each kind, the total must equal the
 * count). The same frame as the combat's sheets: the title, the name and search fields and the
 * footer stay put, and only the list scrolls.
 *
 * Before the cast it says what the casting would send away ("Isso encerra Conjurar Animais e dispensa 8
 * criaturas"); a refusal by the server stays here, in words, with the sheet open. The key of the cast is
 * made anew whenever a choice changes, so a tap repeated after a lost answer never casts twice.
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
  protected readonly max = CREATURE_NAME_MAX;
  protected readonly nameCounter = nameCounter;

  protected readonly options = signal<GetSummonOptionsResponse | null>(null);
  protected readonly name = signal('');
  protected readonly slotKey = signal('');
  protected readonly option = signal(0);
  /** How many of each creature kind: one kind with 1 for a single creature. */
  protected readonly counts = signal<Readonly<Record<string, number>>>({});
  protected readonly forms = signal<readonly Creature[] | null>(null);
  protected readonly beasts = signal<readonly CreatureSummary[] | null>(null);
  protected readonly busy = signal(false);
  protected readonly error = signal('');
  private readonly frame = viewChild.required(SheetFrame);
  private key = newKey();
  private beastSeq = 0;

  protected readonly spell = computed<SummonSpellOptions | null>(() => this.options()?.spells.find((s) => s.spellKey === this.data.spellKey) ?? null);
  /** A ritual spends no slot: the server says whether the character can cast this one as one. */
  protected readonly ritual = computed(() => this.spell()?.canRitual ?? false);
  protected readonly familiar = computed(() => this.data.spellKey === FIND_FAMILIAR);

  protected readonly subtitle = computed(() => {
    const s = this.spell();
    return s ? tight(`Magia de ${circleLabel(s.level)}${this.ritual() ? ' · ritual' : ''} · ${s.castingTimePt}`) : '';
  });

  /** The slots this spell can use: from its circle up, only the circles the server lists for it. */
  protected readonly slotRows = computed<readonly SlotRow[]>(() => {
    const s = this.spell();
    if (!s) {
      return [];
    }
    return (this.options()?.slots ?? [])
      .filter((sl) => sl.level >= s.level && sl.total > 0 && s.circles.some((c) => c.circle === sl.level))
      .map((sl) => slotRow(sl));
  });
  protected readonly slot = computed(() => this.slotRows().find((r) => slotId(r) === this.slotKey()) ?? null);
  /** The circle the cast is made at: the slot's, or the spell's own for a ritual. */
  protected readonly circle = computed(() => (this.ritual() ? (this.spell()?.level ?? 0) : (this.slot()?.level ?? 0)));
  protected readonly circleOptions = computed<readonly SummonOption[]>(() => this.spell()?.circles.find((c) => c.circle === this.circle())?.options ?? []);
  protected readonly opt = computed<SummonOption | null>(() => this.circleOptions()[this.option()] ?? null);
  protected readonly count = computed(() => this.opt()?.count ?? 0);
  protected readonly total = computed(() => Object.values(this.counts()).reduce((a, b) => a + b, 0));
  protected readonly several = computed(() => this.count() > 1);
  /** The one creature chosen, for the radios. */
  protected readonly single = computed(() => (this.several() ? '' : (Object.keys(this.counts())[0] ?? '')));

  protected readonly replacesNote = computed(() => {
    const s = this.spell();
    return s ? replacesText(s.namePt, s.concentration, s.replaces) : '';
  });

  protected readonly rows = computed<readonly ChoiceRow[]>(() => {
    const o = this.opt();
    if (!o) {
      return [];
    }
    if (o.forms.length === 0) {
      return (this.beasts() ?? []).map((b) => ({ key: b.key, title: b.namePt, subtitle: summarySubtitle(b) }));
    }
    const blocks = new Map((this.forms() ?? []).map((c) => [c.summary?.key ?? '', c]));
    return o.forms
      .map((f) => {
        const block = blocks.get(f.monsterKey);
        return { key: f.monsterKey, title: f.namePt, subtitle: block ? formSubtitle(block) : '' };
      })
      .sort((a, b) => a.title.localeCompare(b.title, 'pt-BR'));
  });
  protected readonly loadingRows = computed(() => {
    const o = this.opt();
    return !!o && (o.forms.length === 0 ? this.beasts() === null : this.forms() === null);
  });
  protected readonly listLabel = computed(() => {
    const n = this.rows().length;
    if (this.familiar()) {
      return `Forma · ${n} do livro`;
    }
    return this.several() ? `Criaturas · ${this.total()} de ${this.count()}` : 'Criatura';
  });

  /** What is still missing, in one sentence naming everything; empty when the cast is ready. */
  protected readonly missing = computed(() => {
    const parts: string[] = [];
    if (!this.ritual() && !this.slot()) {
      parts.push('escolha o espaço de magia');
    }
    if (this.familiar() && this.name().trim() === '') {
      parts.push('dê um nome ao familiar');
    }
    const need = this.count() - this.total();
    if (this.opt() && need > 0) {
      parts.push(this.several() ? `escolha mais ${creaturesText(need)}` : this.familiar() ? 'escolha a forma' : 'escolha a criatura');
    } else if (need < 0) {
      parts.push(`tire ${creaturesText(-need)}`);
    }
    if (parts.length === 0) {
      return '';
    }
    // "Escolha a forma e dê um nome ao familiar.": the choice first, then the name.
    parts.sort((a, b) => (a.startsWith('dê') ? 1 : 0) - (b.startsWith('dê') ? 1 : 0));
    const text = parts.length === 1 ? parts[0] : `${parts.slice(0, -1).join(', ')} e ${parts[parts.length - 1]}`;
    return `${text[0].toUpperCase()}${text.slice(1)}.`;
  });
  protected readonly ready = computed(() => this.missing() === '' && !this.busy() && this.spell() !== null);

  /** "Conjurar como ritual · 1 hora · sem gastar espaço", or what the slot costs. */
  protected readonly costLine = computed(() => {
    const s = this.spell();
    if (!s) {
      return '';
    }
    if (this.ritual()) {
      return tight(`Conjurar como ritual · ${s.castingTimePt} · sem gastar espaço`);
    }
    const slot = this.slot();
    const spend = slot ? `gasta um espaço de ${circleLabel(slot.level)}${slot.pact ? ' (pacto)' : ''}` : 'gasta um espaço de magia';
    const made = this.count() > 1 ? `${creaturesText(this.count())} · ` : '';
    return tight(`${made}${s.castingTimePt} · ${spend}`);
  });

  protected readonly buttonLabel = computed(() => castVerb(this.data.spellKey, this.spell()?.namePt ?? ''));

  constructor() {
    void this.load();
    // The forms (their numbers) are read once the spell is known; the beasts again when the option changes.
    effect(() => {
      const o = this.opt();
      untracked(() => {
        if (!o) {
          return;
        }
        if (o.forms.length > 0) {
          void this.loadForms(o);
        } else {
          void this.loadBeasts(o);
        }
      });
    });
  }

  private async load(): Promise<void> {
    try {
      const options = await this.client.summonOptions(this.data.campaignId, this.data.characterId);
      this.options.set(options);
      const spell = options.spells.find((s) => s.spellKey === this.data.spellKey);
      if (!spell) {
        this.error.set('A ficha não conjura mais essa magia. Feche esta folha e olhe a ficha.');
        return;
      }
      if (this.familiar() && spell.replaces[0]) {
        this.name.set(spell.replaces[0].name);
      }
      const free = this.slotRows().find((r) => r.enabled);
      if (free) {
        this.slotKey.set(slotId(free));
      }
    } catch (err) {
      this.options.set(create(GetSummonOptionsResponseSchema, {}));
      this.error.set(creatureErrorMessage(err, 'read'));
    }
  }

  private async loadForms(o: SummonOption): Promise<void> {
    // A form whose numbers cannot be read still shows its name; only the line under it is missing.
    const read = await Promise.allSettled(o.forms.map((f) => this.client.statBlock(this.data.campaignId, f.monsterKey)));
    this.forms.set(read.flatMap((r) => (r.status === 'fulfilled' ? [r.value] : [])));
  }

  private async loadBeasts(o: SummonOption): Promise<void> {
    const seq = ++this.beastSeq;
    this.beasts.set(null);
    try {
      const found = (await this.client.search(this.data.campaignId, { type: o.type, maxCr: o.maxCr })).creatures;
      // An answer that arrives after another option was picked is for a list nobody looks at.
      if (seq === this.beastSeq) {
        this.beasts.set(found);
      }
    } catch (err) {
      if (seq === this.beastSeq) {
        this.beasts.set([]);
        this.error.set(creatureErrorMessage(err, 'read'));
      }
    }
  }

  protected pickSlot(row: SlotRow): void {
    this.slotKey.set(slotId(row));
    this.option.set(0);
    this.counts.set({});
    this.changed();
  }

  protected pickOption(index: number): void {
    this.option.set(index);
    this.counts.set({});
    this.changed();
  }

  /** One creature: the radio's choice. */
  protected pick(key: string): void {
    this.counts.set({ [key]: 1 });
    this.changed();
  }

  /** Several creatures: "−" and "+" of one kind. */
  protected step(change: { key: string; delta: number }): void {
    const next = Math.max(0, (this.counts()[change.key] ?? 0) + change.delta);
    if (change.delta > 0 && this.total() >= this.count()) {
      return;
    }
    const counts = { ...this.counts(), [change.key]: next };
    if (next === 0) {
      delete counts[change.key];
    }
    this.counts.set(counts);
    this.changed();
  }

  protected setName(value: string): void {
    this.name.set(value);
    this.changed();
  }

  /** The chosen option's label: "4 criaturas de ND 1/2 ou menos". */
  protected optionLabel(o: SummonOption): string {
    return tight(`${creaturesText(o.count)} de ND ${o.maxCr} ou menos`);
  }

  /** A choice changed: the next cast is a new one (a new key). */
  protected changed(): void {
    this.key = newKey();
    this.error.set('');
  }

  protected async cast(): Promise<void> {
    const spell = this.spell();
    if (!spell || !this.ready()) {
      return;
    }
    this.busy.set(true);
    this.error.set('');
    // The kinds in the list's order, each as many times as it was chosen.
    const keys = this.rows().flatMap((r) => Array.from({ length: this.counts()[r.key] ?? 0 }, () => r.key));
    // Only a familiar is named by the table; any other creature takes the book's name, numbered when several.
    const names = this.familiar() && this.name().trim() !== '' ? [this.name().trim()] : [];
    const slot = this.slot();
    try {
      const res = await this.client.castSummon({
        campaignId: this.data.campaignId,
        characterId: this.data.characterId,
        spellKey: spell.spellKey,
        ritual: this.ritual(),
        slot: this.ritual() || !slot ? undefined : { level: slot.level, pact: slot.pact },
        summon: { option: this.option(), creatureKeys: keys, names },
        idempotencyKey: this.key,
      });
      this.sheet.close({
        spellName: spell.namePt,
        ritual: this.ritual(),
        castingTime: spell.castingTimePt,
        names,
        count: keys.length,
        dismissed: res.replacedIds.length,
      });
    } catch (err) {
      this.error.set(creatureErrorMessage(err, 'cast'));
      // The refusal is at the top of the body: scroll there, where it is seen.
      setTimeout(() => this.frame().scrollToTop());
    } finally {
      this.busy.set(false);
    }
  }

  protected close(): void {
    this.sheet.close();
  }
}

function slotId(r: SlotRow): string {
  return `${r.level}${r.pact ? 'p' : ''}`;
}

function slotRow(sl: SummonSlot): SlotRow {
  return {
    level: sl.level,
    pact: sl.pact,
    free: sl.free,
    total: sl.total,
    used: sl.total - sl.free,
    enabled: sl.free > 0,
    title: sl.pact ? `${circleLabel(sl.level)} (pacto)` : circleLabel(sl.level),
    count: freeText(sl.free, sl.total),
  };
}
