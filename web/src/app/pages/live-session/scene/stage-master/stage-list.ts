import {
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  inject,
  input,
  output,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { StageController } from '../../../../core/play/stage-controller';
import type { SceneState } from '../../../../core/play/scene-state';
import type { StageCandidate } from '../../../../core/play/stage-roster';
import { STAGE_FULL_REASON } from '../../../../core/play/stage-view';
import { Portrait } from '../../../../shared/portrait/portrait';

/**
 * The campaign's NPCs, one per line (E8-09 state 2): portrait, name, kind and
 * "Pôr em cena", or the green "Em cena" tag for one that is on the stage
 * already. Every tap acts at once, so the list stays open for the next one.
 * With the stage full, the lines that are not on it carry the same dashed
 * button as the page, off (`aria-disabled`, still focusable) with the reason
 * linked by `aria-describedby`. It sits inline on a desktop and in the
 * "Pôr em cena" sheet on a phone, over the same `StageController`.
 *
 * Focus: the button that was pressed turns into the tag, so focus goes to the
 * next line that has a button; with none left, `exhausted` tells the host to
 * move it (to the list's title or the sheet's "Fechar").
 */
@Component({
  selector: 'app-stage-list',
  imports: [MatButtonModule, MatIconModule, Portrait],
  template: `
    @if (full()) {
      <p class="sl__reason" [id]="reasonId">{{ reason }}</p>
    }
    <ul class="sl">
      @for (c of candidates(); track c.characterId) {
        <li class="sl__row">
          <app-portrait [src]="c.portraitUrl" [name]="c.name" [size]="48" fit="cover" [decorative]="false" />
          <div class="sl__text">
            <span class="sl__name">{{ c.name }}</span>
            <span class="sl__meta">
              <span class="sl__kind">{{ kindLine(c) }}</span>
              @if (onStage(c)) {
                <span class="sl__tag"><mat-icon aria-hidden="true">check</mat-icon>Em cena</span>
              }
            </span>
          </div>
          @if (onStage(c)) {
            <!-- the tag is in the meta line above -->
          } @else if (full()) {
            <button
              matButton="outlined"
              type="button"
              class="sl__put sl__put--off"
              aria-disabled="true"
              [attr.aria-describedby]="reasonId"
              [attr.aria-label]="'Pôr ' + c.name + ' em cena'"
            >
              <mat-icon aria-hidden="true">block</mat-icon>Pôr em cena
            </button>
          } @else {
            <button
              matButton="outlined"
              type="button"
              class="sl__put"
              data-put
              [aria-disabled]="ctl().busy(c.characterId)"
              [attr.aria-label]="'Pôr ' + c.name + ' em cena'"
              (click)="put(c)"
            >
              <mat-icon aria-hidden="true">login</mat-icon>Pôr em cena
            </button>
          }
        </li>
      }
    </ul>
  `,
  styleUrl: './stage-list.scss',
})
export class StageList {
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);

  readonly candidates = input.required<readonly StageCandidate[]>();
  readonly state = input.required<SceneState>();
  readonly ctl = input.required<StageController>();
  /** No button is left to press: the host moves focus. */
  readonly exhausted = output<void>();

  protected readonly reason = STAGE_FULL_REASON;
  protected readonly reasonId = 'stage-full-reason-' + Math.random().toString(36).slice(2, 8);
  protected readonly full = computed(() => !this.ctl().hasRoom());
  private readonly onStageIds = computed(
    () =>
      new Set(
        this.state()
          .stage()
          .map((n) => n.characterId),
      ),
  );

  protected onStage(c: StageCandidate): boolean {
    return this.onStageIds().has(c.characterId);
  }

  /** "Aliado · sem retrato": the kind, and a note when there is no portrait. */
  protected kindLine(c: StageCandidate): string {
    return c.portraitUrl ? c.kindLabel : `${c.kindLabel} · sem\u00a0retrato`;
  }

  protected async put(c: StageCandidate): Promise<void> {
    if (this.ctl().busy(c.characterId)) {
      return;
    }
    if (await this.ctl().put(c.characterId)) {
      afterNextRender(
        () => {
          const next = this.host.nativeElement.querySelector<HTMLElement>(
            '[data-put]:not([aria-disabled="true"])',
          );
          if (next) {
            next.focus();
          } else {
            this.exhausted.emit();
          }
        },
        { injector: this.injector },
      );
    }
  }
}
