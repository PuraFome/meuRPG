import { NgTemplateOutlet } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  effect,
  inject,
  input,
  signal,
  untracked,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import { ItemLoss } from '../../../../gen/meurpg/characters/v1/characters_pb';
import { ActionKey } from '../../../core/connect/idempotency';
import { focusWithRing } from '../../../core/creatures/focus-ring';
import { ChargesClient } from '../../../core/play/charges-client';
import { chargesErrorMessage } from '../../../core/play/charges-errors';
import type { VitalsVm } from '../../../core/play/live-session.types';
import {
  dawnResultOf,
  quietText,
  type AmmunitionRow,
  type ChargesItemRow,
  type DawnResult,
} from '../../../core/play/party-charges';
import { PartyCharges } from '../../../core/play/party-charges-state';
import { initialOf } from '../../../core/play/scene-view';
import { CountStepper } from '../../../shared/count-stepper/count-stepper';
import { HiddenSwitch } from '../../../shared/hidden-switch/hidden-switch';

/** The row being corrected: an item (with the "Destruída" switch) or one kind of ammunition. */
interface Editing {
  readonly kind: 'item' | 'ammo';
  readonly characterId: string;
  /** The entry's id, or the ammunition's catalog key. */
  readonly id: string;
  readonly name: string;
  readonly was: number;
  readonly max: number;
  readonly count: number;
  readonly destroyed: boolean;
}

/** The most pieces of one kind the server takes. */
const AMMUNITION_MAX = 9999;

/**
 * "Cargas e munição" in the master's Grupo tab, above "Descanso": for each character the items with charges and the counted
 * ammunition, a "Corrigir" stepper on each row ("eram N", "Salvar", "Cancelar") and, for an item, a "Destruída" switch. The
 * correction is `SetCharges` / `SetAmmunition`. "Amanhecer" in the head is `ChargesService.Dawn`: the items of the whole
 * table get their dawn when the table does not rest, and the answer lists what each one regained. The sheets are read
 * again after each change. The refusals are Portuguese sentences, never the server's words.
 */
@Component({
  selector: 'app-party-charges',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [CountStepper, HiddenSwitch, MatButtonModule, MatIconModule, NgTemplateOutlet],
  templateUrl: './party-charges.html',
  styleUrl: './party-charges.scss',
})
export class PartyChargesPanel {
  readonly campaignId = input.required<string>();
  readonly party = input.required<readonly VitalsVm[]>();

  private readonly client = inject(ChargesClient);
  protected readonly store = inject(PartyCharges);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);

  protected readonly initial = initialOf;
  protected readonly editing = signal<Editing | null>(null);
  protected readonly saving = signal(false);
  protected readonly editError = signal('');
  protected readonly dawning = signal(false);
  protected readonly dawn = signal<DawnResult | null>(null);
  protected readonly dawnError = signal('');

  protected readonly quiet = computed(() => quietText(this.store.view().quiet));
  protected readonly nothing = computed(
    () => this.store.status() === 'ready' && this.store.view().characters.length === 0,
  );
  /** Who is in the party, for the read: a change in who plays reads the sheets again. */
  private readonly who = computed(() =>
    this.party()
      .map((v) => `${v.characterId}:${v.name}`)
      .join('|'),
  );

  private readonly key = new ActionKey();
  private readonly dawnKey = new ActionKey();

  constructor() {
    effect(() => {
      this.who();
      untracked(() => void this.reload());
    });
  }

  private reload(): Promise<void> {
    return this.store.load(
      this.campaignId(),
      this.party().map((v) => ({ characterId: v.characterId, name: v.name })),
    );
  }

  /** "Corrigir" reads the sheets again first: "eram N" and the stepper start from what the item holds now. */
  protected async editItem(characterId: string, shown: ChargesItemRow): Promise<void> {
    await this.reload();
    const fresh = this.store
      .view()
      .characters.find((c) => c.characterId === characterId)
      ?.items.find((i) => i.entryId === shown.entryId);
    if (this.store.status() !== 'ready' || !fresh) {
      return;
    }
    const item = fresh;
    this.editing.set({
      kind: 'item',
      characterId,
      id: item.entryId,
      name: item.name,
      was: item.left,
      max: item.max,
      count: item.left,
      destroyed: item.destroyed,
    });
    this.opened();
  }

  protected async editAmmunition(characterId: string, shown: AmmunitionRow): Promise<void> {
    await this.reload();
    const fresh = this.store
      .view()
      .characters.find((c) => c.characterId === characterId)
      ?.ammunition.find((a) => a.key === shown.key);
    if (this.store.status() !== 'ready' || !fresh) {
      return;
    }
    const ammo = fresh;
    this.editing.set({
      kind: 'ammo',
      characterId,
      id: ammo.key,
      name: ammo.name,
      was: ammo.quantity,
      max: AMMUNITION_MAX,
      count: ammo.quantity,
      destroyed: false,
    });
    this.opened();
  }

  private opened(): void {
    this.editError.set('');
    afterNextRender(
      () => focusWithRing(this.host.nativeElement.querySelector<HTMLElement>('.pc__edit .cs__btn')),
      { injector: this.injector },
    );
  }

  protected setCount(count: number): void {
    this.editing.update((e) => (e ? { ...e, count } : e));
  }

  protected setDestroyed(destroyed: boolean): void {
    this.editing.update((e) => (e ? { ...e, destroyed, count: destroyed ? 0 : e.count } : e));
  }

  protected cancel(): void {
    const e = this.editing();
    if (!e || this.saving()) {
      return;
    }
    this.close(e);
  }

  private close(e: Editing): void {
    this.editing.set(null);
    this.editError.set('');
    afterNextRender(
      () =>
        focusWithRing(
          this.host.nativeElement.querySelector<HTMLElement>(
            `[data-corrigir="${e.characterId}:${e.id}"]`,
          ),
        ),
      { injector: this.injector },
    );
  }

  protected async save(): Promise<void> {
    const e = this.editing();
    if (!e || this.saving()) {
      return;
    }
    this.saving.set(true);
    this.editError.set('');
    try {
      if (e.kind === 'ammo') {
        await this.client.setAmmunition(this.campaignId(), e.characterId, e.id, e.count);
      } else {
        const lost = e.destroyed ? ItemLoss.GONE : ItemLoss.UNSPECIFIED;
        const left = e.destroyed ? 0 : e.count;
        await this.client.setCharges(
          this.campaignId(),
          e.characterId,
          e.id,
          left,
          lost,
          this.key.keyFor({ e: e.id, left, lost }),
        );
      }
      this.key.renew();
      await this.reload();
      this.close(e);
    } catch (err) {
      this.editError.set(chargesErrorMessage(err, { itemName: e.name, left: e.was }, 'salvar'));
    } finally {
      this.saving.set(false);
    }
  }

  protected async amanhecer(): Promise<void> {
    if (this.dawning() || this.store.waiting().length === 0) {
      return;
    }
    this.dawning.set(true);
    this.dawnError.set('');
    try {
      const res = await this.client.dawn(this.campaignId(), this.dawnKey.keyFor({}));
      this.dawnKey.renew();
      this.dawn.set(dawnResultOf(res.restored, (id) => this.store.nameOf(id)));
      await this.reload();
    } catch (err) {
      this.dawn.set(null);
      this.dawnError.set(chargesErrorMessage(err, { itemName: '', left: 0 }, 'amanhecer'));
    } finally {
      this.dawning.set(false);
    }
  }
}
