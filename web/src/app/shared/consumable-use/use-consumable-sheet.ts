import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  effect,
  inject,
  signal,
  viewChild,
  viewChildren,
} from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';

import { parseSum } from '../../core/combat/combat-dice';
import {
  type ConsumableKind,
  type ConsumableTargetVm,
  costsAction,
  effectIcon,
  effectLine,
  healingDice,
  subtitleOf,
  type TurnBlock,
} from '../../core/characters/consumables';
import { ActionKey } from '../../core/connect/idempotency';
import type { ConsumableRoll } from '../../core/play/charges-client';
import { turnBlockMessage } from '../../core/play/charges-errors';
import { EconomyMark } from '../economy-shapes/economy-shapes';
import { SheetFrame } from '../sheet/sheet-frame/sheet-frame';
import { injectSheet, openSheet } from '../sheet/sheet-host';

/** Who may receive the consumable, or why the list could not be read. */
export interface TargetsOutcome {
  readonly targets: readonly ConsumableTargetVm[];
  /** A refusal in words; empty when the list was read. */
  readonly error: string;
}

/** What a tap on "Beber" or "Dar a …" asks: the receiver (`null` for the user) and how the healing dice come. */
export interface UseConsumableRequest {
  readonly target: ConsumableTargetVm | null;
  readonly roll: ConsumableRoll;
}

export interface UseConsumableData {
  readonly itemName: string;
  /** The catalog key of the item: which ones cost the action in a combat depends on it (`costsAction`). */
  readonly itemKey: string;
  readonly kind: ConsumableKind;
  /** What "Usar" does, as the server words it ("Cura 2d4 + 2"); empty for an item with no automatic effect. */
  readonly effectPt: string;
  /** The units of the item the sheet holds: "Restam 2". */
  readonly left: number;
  /** The master acts for this character: the title reads "Usar por Tor". `null` for the player's own use. */
  readonly actingFor: string | null;
  /** Whether a combat is running, when the page knows (the combat's "Itens" does); `null` on the sheet, which says both costs. */
  readonly inCombat: boolean | null;
  /** What the page knows keeps the use from the Action (it is not the turn, or the Action is spent): both buttons are dashed and the sheet says why, with no call to the server. Only for an item that costs the action. */
  readonly blockedBy: TurnBlock | null;
  /** The app may roll the dice (the table did not force real dice). */
  readonly canApp: boolean;
  /** Typing the faces of real dice is allowed (the table did not force the app). */
  readonly canType: boolean;
  /** Where the player rolls once the table's mode and their choice are put together (RN-18). */
  readonly preferApp: boolean;
  /** Reads who the item may be given to; never rejects. */
  readonly loadTargets: () => Promise<TargetsOutcome>;
  /** Uses the item under `key`; answers the refusal in words, or '' when it was used. */
  readonly use: (request: UseConsumableRequest, key: string) => Promise<string>;
}

/** Opens "Usar · <item>": a bottom sheet on a phone, a dialog from a tablet up. Answers `true` when the unit was used. */
export function openUseConsumable(
  dialog: MatDialog,
  bottomSheet: MatBottomSheet,
  data: UseConsumableData,
) {
  return openSheet<UseConsumableSheet, UseConsumableData, boolean>(
    dialog,
    bottomSheet,
    UseConsumableSheet,
    {
      data,
      ariaLabel: data.actingFor ? `Usar por ${data.actingFor}` : `Usar ${data.itemName}`,
      labelledBy: 'use-consumable-t',
      width: '500px',
      // The page puts the focus back on the item's row (with the focus ring) once it has drawn it again.
      restoreFocus: false,
    },
  );
}

/** The text a number added to the dice is said with: "+ 2", "− 1". */
function signed(modifier: number): string {
  return `${modifier < 0 ? '−' : '+'} ${Math.abs(modifier)}`;
}

/**
 * "Usar · Poção de cura": drink it ("Beber") or give it ("Dar a…", the receivers the server lists as radio rows). The dice of
 * a healing potion follow the table's rule (RN-18): the app rolls them, or the player types one face for each die ("Digitar
 * os dados"). The call has one idempotency key for the request it carries: the same use again after a failure is a retry and
 * never spends twice. Works on a locked sheet: the sheet only reads and sends.
 */
@Component({
  selector: 'app-use-consumable-sheet',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [EconomyMark, MatButtonModule, MatIconModule, SheetFrame],
  templateUrl: './use-consumable-sheet.html',
  styleUrl: './use-consumable-sheet.scss',
})
export class UseConsumableSheet {
  private readonly sheet = injectSheet<UseConsumableData, boolean>();
  protected readonly data = this.sheet.data;
  protected readonly inSheet = this.sheet.inSheet;
  private readonly injector = inject(Injector);
  private readonly radios = viewChildren<ElementRef<HTMLInputElement>>('radio');
  private readonly giveButton = viewChild('give', { read: ElementRef<HTMLButtonElement> });

  protected readonly title = this.data.actingFor
    ? `Usar por ${this.data.actingFor}`
    : `Usar · ${this.data.itemName}`;
  /** Every potion costs the action in a combat but the oils; any other item is spent at no cost and the master rules on its time. */
  private readonly costs = costsAction(this.data.kind, this.data.itemKey);
  protected readonly subtitle = subtitleOf({ ...this.data, costsAction: this.costs });
  protected readonly effectText = effectLine(this.data.effectPt);
  protected readonly effectIconName = effectIcon(this.data.effectPt);
  /** The dice of a healing potion; `null` for every other consumable, which rolls nothing. */
  protected readonly dice = healingDice(this.data.effectPt);
  protected readonly numbers = Array.from({ length: this.dice?.count ?? 0 }, (_, i) => i);
  protected readonly signedModifier = signed(this.dice?.modifier ?? 0);
  protected readonly drinkWord = this.data.kind === 'potion' ? 'Beber' : 'Usar';
  /** What the item does in words, for the items whose effect line says it only in short. */
  protected readonly explain =
    this.data.effectPt === ''
      ? 'O app só gasta o item e mostra o texto dele: o mestre e você seguem o texto.'
      : this.data.effectPt.startsWith('Resist')
        ? 'Quem bebe fica com um efeito que dura 1 hora: aparece em “Seus efeitos” e acaba sozinho.'
        : '';

  protected readonly phase = signal<'choose' | 'give'>('choose');
  /** `null` while the list is being read. */
  protected readonly targets = signal<readonly ConsumableTargetVm[] | null>(null);
  protected readonly targetsError = signal('');
  protected readonly chosen = signal('');
  protected readonly receiver = computed(
    () => this.targets()?.find((t) => t.id === this.chosen()) ?? null,
  );
  protected readonly receiverName = computed(() =>
    this.phase() === 'give' ? (this.receiver()?.name ?? '') : '',
  );
  /** What the page knows keeps the use from the Action: no list is read and nothing is offered. */
  protected readonly blocked = this.data.blockedBy;
  private readonly blockedText = this.blocked
    ? turnBlockMessage(this.blocked, this.data.actingFor)
    : '';
  /** The server lists only who is within reach for what costs the action in a running combat; any other use lists the party. */
  private readonly byReach = this.data.inCombat === true && this.costs;
  protected readonly whyId = 'use-consumable-why';
  protected readonly targetsErrorId = 'use-consumable-targets-error';
  /** Only a potion is given: a scroll has no "Dar a…", so nobody is listed for it. */
  protected readonly gives = this.data.kind === 'potion';
  protected readonly canGive = computed(
    () => this.gives && !this.blocked && (this.targets()?.length ?? 0) > 0,
  );
  /** "Dar a…" is dashed: the turn does not allow it, or the list is read and has nobody (or could not be read). */
  protected readonly giveOff = computed(
    () => !!this.blocked || (this.targets() !== null && !this.canGive()),
  );
  /** The block icon says there is nobody to give to; a use the turn does not allow keeps the open hand, like "Beber". */
  protected readonly giveIcon = computed(() =>
    this.giveOff() && !this.blocked ? 'block' : 'volunteer_activism',
  );
  /** What says why "Dar a…" is dashed: the line of the turn or of the empty list, or the error of the read. */
  protected readonly giveWhyId = computed(() => {
    if (!this.giveOff()) {
      return null;
    }
    if (this.reasonText()) {
      return this.whyId;
    }
    return this.targetsError() ? this.targetsErrorId : null;
  });
  protected readonly legend = this.byReach
    ? 'Dar a quem está ao seu lado'
    : this.data.inCombat === null
      ? 'Dar a quem pode receber'
      : 'Dar a alguém do grupo';

  /** The player rolls their own dice: the fields open first (the table's mode and the choice of the player, RN-18). */
  protected readonly typing = signal(
    this.data.canType && (!this.data.canApp || !this.data.preferApp),
  );
  protected readonly faces = signal<readonly string[]>(this.numbers.map(() => ''));
  private readonly parsed = computed(() =>
    this.faces().map((text) => parseSum(text, 1, this.dice?.sides ?? 1)),
  );
  protected readonly invalid = computed(() =>
    this.faces().map((text, i) => text.trim() !== '' && this.parsed()[i] === null),
  );
  /** Every die has its face (or the app rolls them). */
  private readonly rolled = computed(
    () => !this.dice || !this.typing() || this.parsed().every((v) => v !== null),
  );

  protected readonly busy = signal(false);
  protected readonly error = signal('');
  protected readonly canSend = computed(() => !this.blocked && !this.busy() && this.rolled());
  /** A request in the air: Esc and the backdrop do not close the sheet under it. */
  protected readonly lockWhileBusy = effect(() => this.sheet.lock(this.busy()));
  private readonly key = new ActionKey();

  /** Why the first state offers less than two buttons: the turn does not allow it, or the list is read and has nobody. */
  protected readonly reasonText = computed(() => {
    if (this.blockedText) {
      return this.blockedText;
    }
    if (!this.gives || this.targets() === null || this.canGive() || this.targetsError()) {
      return '';
    }
    return this.byReach
      ? 'Ninguém a 1,5 m de você para receber a poção.'
      : this.data.inCombat === null
        ? 'Ninguém por perto para receber a poção.'
        : 'Não há mais ninguém no grupo para receber a poção.';
  });

  /** What the use costs: the mark of the Ação and a bold lead where it spends it, an info icon where it says what it does not. */
  protected readonly cost = computed((): { mark: boolean; lead: string; rest: string } | null => {
    if (this.blocked) {
      return null;
    }
    if (this.phase() === 'choose' && this.data.inCombat === true && this.costs) {
      // The subtitle already says it costs the action; the first choice adds only the reason a button is dashed.
      return null;
    }
    const thing = this.data.kind === 'potion' ? 'A poção' : 'O pergaminho';
    const who = this.receiverName();
    const acts = who ? ` ${thing} age em ${who}.` : '';
    const combat = this.data.inCombat;
    if (!this.costs && combat !== false) {
      return {
        mark: false,
        lead: 'Não gasta a ação: o mestre decide o tempo que isso leva.',
        rest: acts,
      };
    }
    if (this.phase() === 'give') {
      return combat === false
        ? { mark: false, lead: 'Fora de combate, não gasta a vez.', rest: acts }
        : {
            mark: true,
            lead: combat === true ? 'Gasta a sua ação.' : 'Em combate, gasta a sua ação.',
            rest: acts,
          };
    }
    if (combat === false) {
      return {
        mark: false,
        lead: 'Fora de combate: não gasta a vez.',
        rest: this.canGive() ? ' “Dar a…” lista o grupo todo.' : '',
      };
    }
    return combat === true
      ? { mark: true, lead: 'Gasta a sua ação.', rest: '' }
      : { mark: false, lead: 'Em combate, gasta a ação; fora dele, não gasta a vez.', rest: '' };
  });

  constructor() {
    if (this.blocked || !this.gives) {
      // The turn does not allow the use, or the item is not given: nobody is listed and the server is not asked.
      this.targets.set([]);
      return;
    }
    void this.data.loadTargets().then((out) => {
      this.targets.set(out.targets);
      this.targetsError.set(out.error);
      this.chosen.set(out.targets[0]?.id ?? '');
    });
  }

  /** "Voltar": the first choice again, with the focus on "Dar a…". */
  protected back(): void {
    this.phase.set('choose');
    afterNextRender(() => this.giveButton()?.nativeElement.focus(), { injector: this.injector });
  }

  protected openGive(): void {
    this.phase.set('give');
    afterNextRender(
      () =>
        this.radios()
          .find((r) => r.nativeElement.checked)
          ?.nativeElement.focus(),
      {
        injector: this.injector,
      },
    );
  }

  protected onFace(index: number, event: Event): void {
    const text = (event.target as HTMLInputElement).value;
    this.faces.update((all) => all.map((old, i) => (i === index ? text : old)));
  }

  protected async send(target: ConsumableTargetVm | null): Promise<void> {
    if (!this.canSend()) {
      return;
    }
    const faces = this.parsed().filter((v): v is number => v !== null);
    const request: UseConsumableRequest = {
      target,
      roll: this.dice && this.typing() ? { faces } : { inApp: true },
    };
    this.busy.set(true);
    this.error.set('');
    try {
      const why = await this.data.use(
        request,
        this.key.keyFor({ target: target?.id ?? '', roll: request.roll }),
      );
      if (why) {
        this.error.set(why);
      } else {
        this.sheet.close(true);
      }
    } finally {
      this.busy.set(false);
    }
  }

  protected close(): void {
    if (!this.busy()) {
      this.sheet.close();
    }
  }
}
