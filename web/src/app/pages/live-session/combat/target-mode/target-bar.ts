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
  untracked,
  viewChild,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import { clearBelowBar, keepClear } from '../../../../shared/scroll-clearance/scroll-clearance';
import type { TargetMode } from './target-mode';

/**
 * What waits for a tap on the map (C7-04 on a phone, C7-05 from 1024px): the ask ("Escolha o alvo · Raio de Fogo", "Mísseis
 * Mágicos · 2 de 3 dardos"), then the target named before the roll, "Escolher na lista" (the sheet's own list: the keyboard and
 * screen-reader path, and the fallback), "Desfazer" for the darts, and "Cancelar" | "Confirmar". "Confirmar" is the one filled
 * button once the target is chosen (every dart placed). On a phone it is the bar above the pinned nav, in the place of the
 * turn's bar; on a laptop a card under the rail's action tabs. A region with a polite status ("2 de 3 dardos"); the focus
 * comes to it when the choice starts.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-target-bar',
  imports: [MatButtonModule, MatIconModule],
  template: `
    @if (mode().session(); as s) {
      <section class="box" [class.box--card]="variant() === 'card'" aria-label="Escolha do alvo" tabindex="-1" (keydown.escape)="mode().cancel()">
        <div class="head">
          <mat-icon class="head__icon" aria-hidden="true">{{ s.rule.kind === 'darts' ? 'auto_fix_high' : 'my_location' }}</mat-icon>
          <div class="head__text">
            <b #head class="head__title" tabindex="-1">{{ title() }}</b>
            <span class="head__hint">{{ hint() }}</span>
          </div>
        </div>
        <div class="links">
          @if (mode().several() && mode().picks().length > 0) {
            <button type="button" class="link" (click)="mode().undo()">
              <mat-icon aria-hidden="true">history</mat-icon>Desfazer
            </button>
          }
          @if (variant() === 'bar' || mode().ready()) {
            <button type="button" class="link" (click)="mode().list()">
              <mat-icon aria-hidden="true">checklist</mat-icon>Escolher na lista
            </button>
          }
        </div>
        <div class="pair">
          <button mat-stroked-button type="button" class="pair__btn" (click)="mode().cancel()">
            <mat-icon aria-hidden="true">close</mat-icon>Cancelar
          </button>
          @if (mode().ready()) {
            <button mat-flat-button type="button" class="pair__btn" (click)="mode().confirm()">
              <mat-icon aria-hidden="true">check</mat-icon>Confirmar
            </button>
          } @else if (variant() === 'bar') {
            <button mat-stroked-button type="button" class="pair__btn mr-button--off" disabledInteractive [disabled]="true" aria-disabled="true">
              <mat-icon aria-hidden="true">check</mat-icon>Confirmar
            </button>
          } @else {
            <button type="button" class="link link--inline" (click)="mode().list()">
              <mat-icon aria-hidden="true">checklist</mat-icon>Escolher na lista
            </button>
          }
        </div>
        <span class="mr-visually-hidden" role="status" aria-live="polite">{{ mode().status() }}</span>
      </section>
    }
  `,
  styles: `
    :host {
      display: block;
    }

    :host(.bar) {
      position: sticky;
      bottom: var(--mr-session-nav, 0px);
      z-index: 5;
      margin: 0 calc(var(--mr-gutter) * -1);
      padding: 10px var(--mr-gutter);
      border-top: 2px solid var(--mr-accent);
      background: var(--mr-surface);

      @media (min-width: 1024px) {
        display: none;
      }
    }

    .box {
      display: flex;
      flex-direction: column;
      gap: 2px;
    }

    // The rail's card: bordered in the accent, under the tabs.
    .box--card {
      gap: 8px;
      margin-block: 12px;
      padding: 12px 16px;
      border: 2px solid var(--mr-accent);
      border-radius: var(--mr-radius-lg, 12px);
    }

    .head {
      display: flex;
      align-items: center;
      gap: 10px;
    }

    .head__icon {
      flex: none;
      color: var(--mr-accent-text);
    }

    .head__text {
      display: flex;
      flex: 1;
      flex-direction: column;
      min-width: 0;
    }

    .head__title {
      font-size: 16px;
      line-height: 21px;
      outline-offset: 2px;
    }

    // Two lines at most: the bar leaves the map card its room above it.
    .head__hint {
      display: -webkit-box;
      overflow: hidden;
      -webkit-box-orient: vertical;
      -webkit-line-clamp: 2;
      font-size: 13px;
      line-height: 18px;
      color: var(--mr-ink-muted);
    }

    .links {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: 0 12px;
    }

    .links:empty {
      display: none;
    }

    .link {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      min-height: 44px;
      padding: 0 4px;
      border: 0;
      background: transparent;
      color: var(--mr-accent-text);
      font: inherit;
      font-weight: 700;
      cursor: pointer;
    }

    .link:focus-visible {
      outline: 2px solid var(--mr-focus);
      outline-offset: 2px;
    }

    .pair {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 8px;
    }

    .pair__btn {
      --mat-button-outlined-container-height: 48px;
      --mat-button-filled-container-height: 48px;
      --mat-button-outlined-outline-color: var(--mr-control-line);
      --mat-button-outlined-label-text-color: var(--mr-ink);
      --mat-button-outlined-disabled-label-text-color: var(--mr-ink-muted);
      --mat-button-outlined-disabled-outline-color: var(--mr-control-line);
      width: 100%;
      min-width: 0;
    }

    .pair__btn.mr-button--off {
      border-style: dashed;
      cursor: default;
    }
  `,
  host: {
    '[class.bar]': "variant() === 'bar'",
    '(document:focusin)': 'keepFocusClear($event)',
  },
})
export class TargetBar {
  readonly mode = input.required<TargetMode>();
  /** "bar": above the phone's nav; "card": under the action tabs of the rail. */
  readonly variant = input<'bar' | 'card'>('bar');
  /** The master acts for this character (C8-01): the ask says "por Pensantus". */
  readonly by = input('');

  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);
  private readonly heading = viewChild<ElementRef<HTMLElement>>('head');

  protected readonly title = computed(() => {
    const m = this.mode();
    const words = m.words();
    const single = m.session()?.rule.kind === 'single';
    return this.variant() === 'card' && single && m.picks().length === 0
      ? 'Escolha o alvo no mapa'
      : (words?.title ?? '');
  });
  protected readonly hint = computed(() => {
    const m = this.mode();
    const words = m.words();
    const single = m.session()?.rule.kind === 'single';
    // Why a tapped creature cannot be the target: in the bar's own line, since a chip over a narrow map would cover a token.
    const why = this.variant() === 'bar' ? m.note()?.text : undefined;
    if (why) {
      return why;
    }
    if (single && m.picks().length === 0 && words) {
      const hint = `${words.hint.charAt(0).toLowerCase()}${words.hint.slice(1)}`;
      const by = this.by();
      if (this.variant() === 'card') {
        return `${m.session()?.name}${by ? `, por ${by}` : ''} · ${hint}`;
      }
      if (by) {
        return `Por ${by} · ${hint}`;
      }
    }
    return words?.hint ?? '';
  });

  /** Keyboard focus on a row under the open bar moves the page up to it (the bar's measured top, plus a gap). */
  protected keepFocusClear(event: FocusEvent): void {
    if (this.variant() === 'bar' && this.mode().session() && event.target instanceof Element) {
      clearBelowBar(this.host.nativeElement, event.target);
    }
  }

  constructor() {
    // The bar covers the foot of the page: what is focused or scrolled into view stops above it.
    keepClear(() => (this.variant() === 'bar' ? this.host.nativeElement : undefined), 'bottom');
    // The focus comes to the bar when the choice starts (the keyboard and the screen reader are told where they are).
    effect(() => {
      if (this.mode().session()) {
        untracked(() =>
          afterNextRender(() => this.heading()?.nativeElement.focus({ preventScroll: true }), {
            injector: this.injector,
          }),
        );
      }
    });
  }
}
