import { NgTemplateOutlet } from '@angular/common';
import { MatButtonModule } from '@angular/material/button';
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
  output,
  untracked,
} from '@angular/core';

import type { GearVm } from '../../core/characters/gear';

/** What a tap on a row's button asks: this piece, equipped or stowed. */
export interface GearToggle {
  readonly key: string;
  readonly equipped: boolean;
}

/**
 * The gear of a sheet in two groups, "Equipado" and "Carregado" (SRD 5.1, "Equipment"): each piece with the button
 * that moves it to the other group. The names, the groups and the armor class a change would give all come from the
 * server (`DerivedSheet.gear`); the list only shows them and tells the host what was asked. The same list is on the
 * sheet and in the combat's "Sacar ou guardar" fold: `cost` is the line a row carries there ("usa a interação
 * gratuita do turno", "usa a ação").
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-gear-list',
  imports: [MatButtonModule, NgTemplateOutlet],
  templateUrl: './gear-list.html',
  styleUrl: './gear-list.scss',
})
export class GearList {
  private static next = 0;
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);
  private askedKey: string | null = null;
  private wasBusy = false;
  protected readonly idBase = `gl${GearList.next++}`;

  readonly gear = input.required<readonly GearVm[]>();
  /** Whether the buttons exist: false for a reader who may not change the gear. */
  readonly canChange = input(true);
  /** The piece being saved: its button waits, the others stay. */
  readonly busyKey = input<string | null>(null);
  /** A line a row carries under its name (the cost of the change in a combat); empty for none. */
  readonly cost = input<(piece: GearVm) => string>(() => '');
  /** The reason a piece cannot change now (a refusal the host knows beforehand); empty when it can. */
  readonly blocked = input<(piece: GearVm) => string>(() => '');

  /** The armor class now (`DerivedSheet.armor_class`): a row shows the server's number only when it differs. */
  readonly armorClass = input<number | null>(null);

  readonly toggled = output<GearToggle>();

  constructor() {
    // The call ended (the list is the new one): the focus follows the piece that moved to its button in the other group.
    effect(() => {
      const busy = this.busyKey() !== null;
      untracked(() => {
        if (this.wasBusy && !busy && this.askedKey) {
          const key = this.askedKey;
          this.askedKey = null;
          afterNextRender(() => this.focusRow(key), { injector: this.injector });
        }
        this.wasBusy = busy;
      });
    });
  }

  protected readonly equipped = computed(() => this.gear().filter((g) => g.equipped));
  protected readonly carried = computed(() => this.gear().filter((g) => !g.equipped));

  protected readonly kindLabel = (g: GearVm): string =>
    g.kind === 'armor'
      ? 'Armadura'
      : g.kind === 'shield'
        ? 'Escudo'
        : g.twoHanded
          ? 'Arma de duas mãos'
          : 'Arma';

  /** What the button promises about the armor class: the server's number, only when it differs from the current. */
  protected armorLine(g: GearVm, current: number | null): string {
    if (g.kind === 'weapon' || g.armorClassIfToggled <= 0 || g.armorClassIfToggled === current) {
      return '';
    }
    return `CA ${g.armorClassIfToggled}`;
  }

  /** What a screen reader hears: the action, the piece and the armor class the server says it would give. */
  protected buttonLabel(g: GearVm): string {
    const ac = this.armorLine(g, this.armorClass());
    return `${g.equipped ? 'Guardar' : 'Equipar'} ${g.namePt}${ac ? `: ${ac}` : ''}`;
  }

  protected ask(g: GearVm): void {
    this.askedKey = g.key;
    this.toggled.emit({ key: g.key, equipped: !g.equipped });
  }

  /** After a change the focus goes to the moved row's button, which is now in the other group. */
  focusRow(key: string): void {
    this.host.nativeElement
      .querySelector<HTMLElement>(`[data-gear-key="${CSS.escape(key)}"]`)
      ?.focus();
  }
}
