import { DOCUMENT } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  ElementRef,
  afterRenderEffect,
  effect,
  inject,
  untracked,
  viewChild,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import { DiceSoundSetting } from './dice-sound-setting';
import { RollAnimator, outcomeLine } from './roll-animator';
import { RollTray } from './roll-tray';

/**
 * The dice roll animation, mounted once in the shell (docs/design/session.md#rolls). It is not a dialog: focus never
 * moves into it, the picture is hidden from assistive technology and the live region announces one sentence when the
 * dice land. It exists only while a roll plays (the dice land in under 2 s; the result stays up to 10 s), so it never
 * blocks the page after that. A tap or Esc lands the dice at once, and a second one closes it. The dice are thrown on
 * a tray (`RollTray`); the words under it are the screen's own, shown when the dice have landed.
 *
 * Under reduced motion nothing moves and nothing dims or blocks the page: the dice sit at rest in a card with the
 * words, the rest of the screen stays usable, and the card closes with Esc, "Fechar" or after 10 s.
 */
@Component({
  selector: 'app-roll-overlay',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatButtonModule, MatIconModule, RollTray],
  host: {
    '(document:keydown.escape)': 'animator.tap()',
    '(document:fullscreenchange)': 'followFullscreen()',
  },
  template: `
    <p class="sr" role="status" aria-live="polite">{{ animator.announcement() }}</p>
    @if (animator.current(); as r) {
      <div #scrim class="scrim" [class.scrim--still]="animator.reduced()" popover="manual">
        <div class="veil" aria-hidden="true" (click)="animator.tap()"></div>
        <div class="stage">
          <app-roll-tray
            [request]="animator.request()"
            [reduced]="animator.reduced()"
            (landed)="animator.land()"
          />
          @if (animator.hidden() > 0) {
            <span class="more" aria-hidden="true">+{{ animator.hidden() }}</span>
          }
          @if (animator.done()) {
            <div
              class="result"
              aria-hidden="true"
              [class.result--crit]="!!r.critical"
              [class.result--fumble]="!!r.fumble"
            >
              <span class="result__label">{{ r.label }}</span>
              @if (r.line) {
                <b class="result__line">{{ r.line }}</b>
              }
              @if (r.note) {
                <span class="result__note">{{ r.note }}</span>
              }
              @if (outcome(r); as word) {
                <b
                  class="result__word"
                  [class.result__word--good]="r.outcome?.good && !r.fumble"
                  [class.result__word--bad]="(r.outcome && !r.outcome.good) || r.fumble"
                  >{{ word }}</b
                >
              }
            </div>
            @if (!animator.reduced()) {
              <span class="hint" aria-hidden="true">Toque para fechar</span>
            }
          }
          <div class="actions">
            <button
              matButton="outlined"
              type="button"
              class="action"
              [attr.aria-pressed]="sound.on()"
              (click)="toggleSound()"
            >
              <mat-icon aria-hidden="true">{{ sound.on() ? 'volume_up' : 'volume_off' }}</mat-icon>
              Som dos dados
            </button>
            @if (animator.reduced()) {
              <button matButton="outlined" type="button" class="action" (click)="animator.dismiss()">
                Fechar
              </button>
            }
          </div>
        </div>
      </div>
    }
  `,
  styleUrl: './roll-overlay.scss',
})
export class RollOverlay {
  protected readonly animator = inject(RollAnimator);
  protected readonly sound = inject(DiceSoundSetting);
  protected readonly outcome = outcomeLine;

  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef).nativeElement;
  private readonly doc = inject(DOCUMENT);
  /** Where the host sits in the app shell, to put it back when full screen ends. */
  private home: { parent: Node; next: Node | null } | null = null;

  /** The veil while a roll plays (none between rolls). */
  private readonly scrim = viewChild<ElementRef<HTMLElement>>('scrim');
  private readonly tray = viewChild(RollTray);

  constructor() {
    const destroyRef = inject(DestroyRef);
    destroyRef.onDestroy(this.animator.attach());
    destroyRef.onDestroy(() => this.restore());
    // A tap or Esc while the dice roll lands them at once.
    effect(() => {
      if (this.animator.skips() > 0) {
        untracked(() => this.tray()?.finish());
      }
    });
    // The app's dialogs and sheets are native popovers (the CDK's default), in the browser's top layer, which no
    // z-index reaches: the veil joins the top layer too, after them, so the dice show above an open sheet (the scene
    // roll's result, the attack sheet). A browser without popovers keeps the z-index above the CDK container.
    afterRenderEffect(() => {
      const el = this.scrim()?.nativeElement;
      if (el && typeof el.showPopover === 'function' && !popoverOpen(el)) {
        try {
          el.showPopover();
        } catch {
          // Not connected yet, or already shown: the z-index still applies.
        }
      }
    });
  }

  /**
   * The browser paints only the full-screen element's subtree, so while a map is on full screen the overlay's host
   * moves inside it (a fixed `.scrim` then covers the screen) and goes back to the shell when the mode ends.
   */
  protected followFullscreen(): void {
    const target = this.doc.fullscreenElement;
    if (target && target !== this.host && !target.contains(this.host)) {
      this.home ??= this.host.parentNode
        ? { parent: this.host.parentNode, next: this.host.nextSibling }
        : null;
      target.appendChild(this.host);
    } else if (!target) {
      this.restore();
    }
  }

  private restore(): void {
    if (this.home?.parent.isConnected) {
      this.home.parent.insertBefore(
        this.host,
        this.home.next?.parentNode === this.home.parent ? this.home.next : null,
      );
    }
    this.home = null;
  }

  /**
   * "Som dos dados": switches the sound on or off for this tab. The tray starts its audio in this very tap, because a
   * browser starts audio only after a gesture; the tap does not reach the veil, so it neither lands nor closes the roll.
   */
  protected toggleSound(): void {
    this.tray()?.setSound(this.sound.toggle());
  }
}

/** Whether the element is a popover on screen (`:popover-open`; a browser that does not know the selector says no). */
function popoverOpen(el: HTMLElement): boolean {
  try {
    return el.matches(':popover-open');
  } catch {
    return false;
  }
}
