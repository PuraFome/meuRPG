import { NgTemplateOutlet } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  ElementRef,
  Injector,
  afterNextRender,
  effect,
  inject,
  input,
  output,
  signal,
  viewChild,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

/**
 * The pair of buttons at the bottom of a sheet, an in-place form or a dialog (E8-05, E9-08, E9-09): two
 * equal buttons, the outlined one ("Cancelar", "Voltar") and the one filled button that says what
 * happens. Right-aligned and 176 px wide from a tablet up; on a phone they share the width, 48 px high,
 * and when either label needs more than half of it (the component measures) they stack, each as wide as
 * the room, the filled one on top. With `ready` false the filled button is the app's dashed, disabled one
 * (`aria-disabled`, "⊘"): the reason belongs to a sentence above it, never to colour alone.
 */
@Component({
  selector: 'app-pair-foot',
  imports: [MatButtonModule, MatIconModule, NgTemplateOutlet],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div #foot class="pf" [class.pf--stacked]="stacked()">
      @if (stacked()) {
        <ng-container *ngTemplateOutlet="confirmBtn" />
        <ng-container *ngTemplateOutlet="cancelBtn" />
      } @else {
        <ng-container *ngTemplateOutlet="cancelBtn" />
        <ng-container *ngTemplateOutlet="confirmBtn" />
      }
    </div>
    <ng-template #cancelBtn>
      <button matButton="outlined" type="button" class="pf__cancel" [attr.data-initial-focus]="safe() ? '' : null" (click)="cancel.emit()">
        {{ cancelLabel() }}
      </button>
    </ng-template>
    <ng-template #confirmBtn>
      @if (!ready()) {
        <button matButton="outlined" type="button" class="pf__off" aria-disabled="true">
          <mat-icon aria-hidden="true">block</mat-icon>{{ confirmLabel() }}
        </button>
      } @else {
        <button matButton="filled" type="button" class="pf__go" [attr.aria-disabled]="busy()" (click)="!busy() && confirm.emit()">
          @if (confirmIcon()) {
            <mat-icon aria-hidden="true">{{ confirmIcon() }}</mat-icon>
          }{{ confirmLabel() }}
        </button>
      }
    </ng-template>
  `,
  styleUrl: './pair-foot.scss',
})
export class PairFoot {
  readonly cancelLabel = input('Cancelar');
  readonly confirmLabel = input.required<string>();
  readonly confirmIcon = input('');
  /** The filled button can be pressed; false draws the dashed, disabled one. */
  readonly ready = input(true);
  readonly busy = input(false);
  /** The cancel button takes the focus when the container opens (an alert dialog). */
  readonly safe = input(false);
  readonly cancel = output<void>();
  readonly confirm = output<void>();

  protected readonly stacked = signal(false);
  private readonly foot = viewChild<ElementRef<HTMLElement>>('foot');
  private readonly injector = inject(Injector);

  constructor() {
    const destroyRef = inject(DestroyRef);
    afterNextRender(() => {
      const foot = this.foot()?.nativeElement;
      if (!foot) {
        return;
      }
      const check = () => this.stacked.set(needsStack(foot));
      check();
      const observer = typeof ResizeObserver === 'function' ? new ResizeObserver(check) : null;
      observer?.observe(foot);
      // The room it has is the host's: a narrow column (the map editor's side panel) shrinks it without changing the pair's own size.
      if (foot.parentElement) {
        observer?.observe(foot.parentElement);
      }
      destroyRef.onDestroy(() => observer?.disconnect());
      // A new label may fit or not.
      effect(
        () => {
          this.confirmLabel();
          this.cancelLabel();
          this.ready();
          afterNextRender(check, { injector: this.injector });
        },
        { injector: this.injector },
      );
    });
  }
}

/** Whether the two buttons must stack. On a phone, when either button's words need more than half of the footer;
 * from a tablet up, when the room the pair has is less than two equal buttons need (a narrow column). Measured on the
 * buttons' own content (icon and label), which has the same width side by side or one over the other. */
export function needsStack(foot: HTMLElement): boolean {
  const buttons = Array.from(foot.querySelectorAll<HTMLElement>('button'));
  if (buttons.length < 2) {
    return false;
  }
  const gap = parseFloat(getComputedStyle(foot).columnGap) || 12;
  const need = (button: HTMLElement) => {
    const style = getComputedStyle(button);
    const padding = parseFloat(style.paddingLeft) + parseFloat(style.paddingRight);
    return Array.from(button.children).reduce((sum, child) => sum + child.getBoundingClientRect().width, 0) + padding;
  };
  if (foot.ownerDocument.defaultView?.matchMedia?.('(min-width: 768px)').matches) {
    // From a tablet up the pair is right-aligned and equal (176 px at least each); it stacks only when the room it has
    // (a narrow column) is less than the two need side by side.
    const room = foot.parentElement?.clientWidth ?? 0;
    return room > 0 && 2 * Math.max(176, ...buttons.map(need)) + gap > room;
  }
  const half = (foot.clientWidth - gap) / 2;
  return buttons.some((button) => need(button) > half);
}
