import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';
import { RouterLink } from '@angular/router';

/**
 * "Esta ficha tem 2 escolhas pendentes. Completar" (PM-05): the notice on the sheet of a living player character
 * whose class or race left choices open (a Fighting Style the sheet never picked, a half-elf's two +1). The count is
 * the server's (the master's and the owner's to know); "Completar" opens the page that makes them without the
 * editor. Nothing shows for zero.
 */
@Component({
  selector: 'app-pending-choices-banner',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule, RouterLink],
  template: `
    @if (count() > 0) {
      <section class="mr-notice mr-notice--warning pending" aria-label="Escolhas pendentes">
        <mat-icon aria-hidden="true">pending_actions</mat-icon>
        <p>
          Esta ficha tem {{ words() }}.
          <a
            class="pending__go"
            aria-label="Completar escolhas pendentes"
            [routerLink]="['/campaigns', campaignId(), 'characters', characterId(), 'choices']"
            >Completar</a
          >
        </p>
      </section>
    }
  `,
  styles: `
    :host {
      display: block;
    }

    .pending {
      margin-bottom: var(--mr-space-4);
    }

    // A 44px target in the middle of a sentence.
    .pending__go {
      display: inline-flex;
      align-items: center;
      min-height: 44px;
      padding: 0 4px;
      font-weight: 700;
      color: inherit;
    }
  `,
})
export class PendingChoicesBanner {
  readonly campaignId = input.required<string>();
  readonly characterId = input.required<string>();
  /** How many selections are open. */
  readonly count = input.required<number>();

  protected readonly words = computed(() => {
    const n = this.count();
    return n === 1 ? '1 escolha pendente' : `${n} escolhas pendentes`;
  });
}
