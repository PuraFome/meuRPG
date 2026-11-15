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
  viewChildren,
} from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';

import { parseSum } from '../../../core/combat/combat-dice';
import {
  type ConsumableKind,
  type ConsumableTargetVm,
  effectIcon,
  effectLine,
  healingDice,
  subtitleOf,
} from '../../../core/characters/consumables';
import { ActionKey } from '../../../core/connect/idempotency';
import type { ConsumableRoll } from '../../../core/play/charges-client';
import { SheetFrame } from '../../../shared/sheet/sheet-frame/sheet-frame';
import { injectSheet, openSheet } from '../../../shared/sheet/sheet-host';

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
  readonly kind: ConsumableKind;
  /** What "Usar" does, as the server words it ("Cura 2d4 + 2"); empty for an item with no automatic effect. */
  readonly effectPt: string;
  /** The units of the item the sheet holds: "Restam 2". */
  readonly left: number;
  /** The master acts for this character: the title reads "Usar por Tor". `null` for the player's own use. */
  readonly actingFor: string | null;
  /** Whether a combat is running, when the page knows (the combat's "Itens" does); `null` on the sheet, which says both costs. */
  readonly inCombat: boolean | null;
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
  imports: [MatButtonModule, MatIconModule, SheetFrame],
  template: `
    <app-sheet-frame
      [title]="title"
      titleId="use-consumable-t"
      [subtitle]="subtitle"
      [phone]="inSheet"
      (closed)="close()"
    >
      <p class="effect">
        <mat-icon aria-hidden="true">{{ effectIconName }}</mat-icon><b>{{ effectText }}</b>
      </p>
      @if (explain) {
        <p class="explain">{{ explain }}</p>
      }
      @if (error()) {
        <p class="mr-notice mr-notice--danger" role="alert">
          <mat-icon aria-hidden="true">error</mat-icon><span>{{ error() }}</span>
        </p>
      }
      @if (targetsError()) {
        <p class="mr-notice mr-notice--danger" role="alert">
          <mat-icon aria-hidden="true">error</mat-icon><span>{{ targetsError() }}</span>
        </p>
      }
      @if (phase() === 'give') {
        <fieldset class="who">
          <legend class="who__legend">{{ legend }}</legend>
          @for (t of targets(); track t.id) {
            <label class="who__row" [class.who__row--on]="chosen() === t.id">
              <input
                #radio
                type="radio"
                name="use-consumable-target"
                class="who__radio"
                [value]="t.id"
                [checked]="chosen() === t.id"
                [disabled]="busy()"
                (change)="chosen.set(t.id)"
              />
              <span class="who__avatar" aria-hidden="true">{{ t.name.charAt(0) }}</span>
              <span class="who__text">
                <b>{{ t.name }}</b>
                @if (t.line) {
                  <span class="mr-muted">{{ t.line }}</span>
                }
              </span>
            </label>
          }
        </fieldset>
      }
      @if (dice; as d) {
        <div class="dice">
          @if (!typing()) {
            <p class="dice__line">
              O app rola {{ d.text }}{{ receiverName() ? ' para ' + receiverName() : '' }}.
              @if (data.canType) {
                <button type="button" class="dice__switch" (click)="typing.set(true)">Digitar os dados</button>
              }
            </p>
          } @else {
            <p class="dice__line">
              Digite o que saiu em cada dado.
              @if (d.modifier !== 0) {
                O {{ signedModifier }} o app soma.
              }
              @if (data.canApp) {
                <button type="button" class="dice__switch" (click)="typing.set(false)">Rolar no app</button>
              }
            </p>
            <div class="dice__fields">
              @for (n of numbers; track n) {
                <div class="dice__one">
                  <label class="dice__label" [for]="'use-consumable-face-' + n">Dado {{ n + 1 }} (d{{ d.sides }})</label>
                  <input
                    [id]="'use-consumable-face-' + n"
                    class="dice__field"
                    type="text"
                    inputmode="numeric"
                    maxlength="3"
                    autocomplete="off"
                    [value]="faces()[n]"
                    [attr.aria-invalid]="invalid()[n] ? 'true' : null"
                    [attr.aria-describedby]="invalid()[n] ? 'use-consumable-face-err-' + n : null"
                    (input)="onFace(n, $event)"
                  />
                  @if (invalid()[n]) {
                    <span class="dice__err" [id]="'use-consumable-face-err-' + n">
                      <mat-icon aria-hidden="true">error</mat-icon>Digite um número de 1 a {{ d.sides }}
                    </span>
                  }
                </div>
              }
            </div>
          }
        </div>
      }
      @if (phase() === 'choose' && noneText()) {
        <p class="none">
          <mat-icon aria-hidden="true">block</mat-icon>{{ noneText() }}
        </p>
      }
      <p class="cost mr-muted">{{ costText() }}</p>
      <div foot class="foot">
        @if (phase() === 'choose') {
          @if (data.kind === 'potion') {
            <button
              matButton="outlined"
              type="button"
              class="give"
              [class.give--off]="!canGive()"
              [disabled]="busy() || !canGive()"
              disabledInteractive
              (click)="canGive() && openGive()"
            >
              <mat-icon aria-hidden="true">volunteer_activism</mat-icon>Dar a…
            </button>
          }
          <button
            matButton="filled"
            type="button"
            class="go"
            [class.go--off]="!canSend()"
            [disabled]="!canSend()"
            disabledInteractive
            (click)="send(null)"
          >
            <mat-icon aria-hidden="true">{{ data.kind === 'potion' ? 'science' : 'description' }}</mat-icon>{{ busy() ? 'Usando...' : drinkWord }}
          </button>
        } @else {
          <button
            matButton="filled"
            type="button"
            class="go go--wide"
            [class.go--off]="!canSend()"
            [disabled]="!canSend()"
            disabledInteractive
            (click)="send(receiver())"
          >
            <mat-icon aria-hidden="true">volunteer_activism</mat-icon>{{ busy() ? 'Usando...' : 'Dar a ' + receiverName() }}
          </button>
          <button
            matButton="outlined"
            type="button"
            class="go--wide"
            [disabled]="!canSend()"
            disabledInteractive
            (click)="send(null)"
          >
            <mat-icon aria-hidden="true">science</mat-icon>Beber eu mesmo
          </button>
        }
      </div>
    </app-sheet-frame>
  `,
  styles: `
    .effect {
      display: flex;
      align-items: center;
      gap: var(--mr-space-2);
      margin: 0 0 var(--mr-space-2);
    }
    .effect .mat-icon {
      width: var(--mr-font-18);
      height: var(--mr-font-18);
      font-size: var(--mr-font-18);
    }
    .explain {
      margin: 0 0 var(--mr-space-3);
    }
    .who {
      min-width: 0;
      margin: 0 0 var(--mr-space-3);
      padding: 0;
      border: 0;
    }
    .who__legend {
      margin-bottom: var(--mr-space-2);
      padding: 0;
      font-size: var(--mr-font-14);
      font-weight: 600;
    }
    .who__row {
      display: flex;
      align-items: center;
      gap: var(--mr-space-3);
      min-height: 56px;
      margin-bottom: var(--mr-space-2);
      padding: var(--mr-space-2) var(--mr-space-3);
      border: 1px solid var(--mr-rule);
      border-radius: var(--mr-radius-md);
      cursor: pointer;
    }
    .who__row--on {
      border-color: var(--mr-accent);
      background: var(--mr-accent-soft);
    }
    .who__row:has(:focus-visible) {
      outline: 2px solid var(--mr-focus);
      outline-offset: 2px;
    }
    .who__radio {
      flex: none;
      width: 20px;
      height: 20px;
      margin: 0;
      accent-color: var(--mr-accent);
    }
    .who__avatar {
      display: grid;
      flex: none;
      place-items: center;
      width: 32px;
      height: 32px;
      border-radius: 50%;
      background: var(--mr-ink);
      color: var(--mr-surface);
      font-size: var(--mr-font-14);
      font-weight: 700;
    }
    .who__text {
      display: flex;
      flex-direction: column;
      min-width: 0;
      font-size: var(--mr-font-14);
    }
    .dice {
      margin: 0 0 var(--mr-space-2);
    }
    .dice__line {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      justify-content: space-between;
      gap: var(--mr-space-1) var(--mr-space-3);
      margin: 0 0 var(--mr-space-2);
      font-size: var(--mr-font-14);
    }
    .dice__switch {
      min-height: 44px;
      padding: 0 var(--mr-space-1);
      border: 0;
      background: none;
      color: var(--mr-accent-text);
      font: inherit;
      font-weight: 700;
      text-decoration: underline;
      cursor: pointer;
    }
    .dice__fields {
      display: flex;
      flex-wrap: wrap;
      gap: var(--mr-space-3);
    }
    .dice__one {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-1);
      width: 104px;
    }
    .dice__label {
      font-size: var(--mr-font-14);
    }
    .dice__field {
      box-sizing: border-box;
      width: 100%;
      min-height: 48px;
      padding: 0 var(--mr-space-3);
      border: 2px solid var(--mr-accent);
      border-radius: var(--mr-radius-md);
      background: var(--mr-surface);
      color: var(--mr-ink);
      font: inherit;
      font-size: var(--mr-font-18);
    }
    .dice__field[aria-invalid='true'] {
      border-color: var(--mr-danger-ink);
    }
    .dice__err {
      display: flex;
      align-items: flex-start;
      gap: var(--mr-space-1);
      color: var(--mr-danger-ink);
      font-size: var(--mr-font-13);
    }
    .dice__err .mat-icon {
      flex: none;
      width: var(--mr-font-14);
      height: var(--mr-font-14);
      font-size: var(--mr-font-14);
    }
    .none {
      display: flex;
      align-items: center;
      gap: var(--mr-space-2);
      margin: 0 0 var(--mr-space-2);
    }
    .none .mat-icon {
      flex: none;
      width: var(--mr-font-18);
      height: var(--mr-font-18);
      font-size: var(--mr-font-18);
    }
    .cost {
      margin: 0;
      font-size: var(--mr-font-14);
    }
    .foot {
      display: flex;
      flex-wrap: wrap;
      justify-content: flex-end;
      gap: var(--mr-space-2) var(--mr-space-3);
    }
    .foot button {
      min-height: 48px;
    }
    .give--off {
      border-style: dashed;
      opacity: 0.7;
      cursor: default;
    }
    .go--off {
      opacity: 0.6;
      cursor: default;
    }
    .go--wide {
      flex: 1 1 100%;
    }
  `,
})
export class UseConsumableSheet {
  private readonly sheet = injectSheet<UseConsumableData, boolean>();
  protected readonly data = this.sheet.data;
  protected readonly inSheet = this.sheet.inSheet;
  private readonly injector = inject(Injector);
  private readonly radios = viewChildren<ElementRef<HTMLInputElement>>('radio');

  protected readonly title = this.data.actingFor
    ? `Usar por ${this.data.actingFor}`
    : `Usar · ${this.data.itemName}`;
  protected readonly subtitle = subtitleOf(this.data);
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
  protected readonly canGive = computed(() => (this.targets()?.length ?? 0) > 0);
  protected readonly legend =
    this.data.inCombat === true
      ? 'Dar a quem está ao seu lado'
      : this.data.inCombat === false
        ? 'Dar a alguém do grupo'
        : 'Dar a quem pode receber';

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
  private readonly rolled = computed(() => !this.dice || !this.typing() || this.parsed().every((v) => v !== null));

  protected readonly busy = signal(false);
  protected readonly error = signal('');
  protected readonly canSend = computed(() => !this.busy() && this.rolled());
  /** A request in the air: Esc and the backdrop do not close the sheet under it. */
  protected readonly lockWhileBusy = effect(() => this.sheet.lock(this.busy()));
  private readonly key = new ActionKey();

  protected readonly noneText = computed(() => {
    if (this.targets() === null || this.canGive() || this.targetsError()) {
      return '';
    }
    return this.data.inCombat === true
      ? 'Ninguém a 1,5 m de você para receber a poção.'
      : this.data.inCombat === false
        ? 'Não há mais ninguém no grupo para receber a poção.'
        : 'Ninguém por perto para receber a poção.';
  });

  protected readonly costText = computed(() => {
    const thing = this.data.kind === 'potion' ? 'A poção' : 'O pergaminho';
    const who = this.receiverName();
    const acts = who ? ` ${thing} age em ${who}.` : '';
    const combat = this.data.inCombat;
    if (this.phase() === 'give') {
      const lead =
        combat === false
          ? 'Fora de combate, não gasta a vez.'
          : combat === true
            ? 'Gasta a sua ação.'
            : 'Em combate, gasta a sua ação.';
      return `${lead}${acts}`;
    }
    return combat === false
      ? `Fora de combate: não gasta a vez.${this.canGive() ? ' “Dar a…” lista o grupo todo.' : ''}`
      : combat === true
        ? 'Gasta a sua ação.'
        : 'Em combate, gasta a ação; fora dele, não gasta a vez.';
  });

  constructor() {
    void this.data.loadTargets().then((out) => {
      this.targets.set(out.targets);
      this.targetsError.set(out.error);
      this.chosen.set(out.targets[0]?.id ?? '');
    });
  }

  protected openGive(): void {
    this.phase.set('give');
    afterNextRender(() => this.radios().find((r) => r.nativeElement.checked)?.nativeElement.focus(), {
      injector: this.injector,
    });
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
      const why = await this.data.use(request, this.key.keyFor({ target: target?.id ?? '', roll: request.roll }));
      if (why) {
        this.error.set(why);
      } else {
        this.key.renew();
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
