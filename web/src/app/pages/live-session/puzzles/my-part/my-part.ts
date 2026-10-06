import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/**
 * "A sua parte da pista" (MR-038, RN-27, E10-12 state 9): the split information on a player's phone. The player reads **only their own part**,
 * labelled as only theirs ("Só você vê esta parte."), and learns only the names of the characters that have another one, never the text:
 * the table has to talk to put the clue together. The server sends nothing of anyone else's part.
 */
@Component({
  selector: 'app-my-part',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule],
  template: `
    @if (part()) {
      <section class="mr-panel mine" aria-labelledby="mp-title">
        <h2 class="mine__title" id="mp-title">A sua parte da pista</h2>
        <blockquote class="mine__text">“{{ part() }}”</blockquote>
        <p class="mine__only"><mat-icon aria-hidden="true">visibility_off</mat-icon>Só você vê esta parte.</p>
      </section>
    }
    @if (holders().length > 0) {
      <section class="mr-panel others" aria-labelledby="mo-title">
        <h2 class="mine__title" id="mo-title">Quem mais tem uma parte</h2>
        <ul class="others__list">
          @for (name of holders(); track name) {
            <li class="others__row">
              <span class="others__badge" aria-hidden="true">{{ name.charAt(0).toUpperCase() }}</span>
              <span class="others__name">{{ name }}</span>
              <span class="others__has">tem uma parte</span>
            </li>
          }
        </ul>
        <p class="others__help">Vocês precisam conversar para juntar as partes. O texto delas não aparece aqui.</p>
      </section>
    }
  `,
  styles: `
    :host {
      display: contents;
    }

    .mine__title {
      margin: 0 0 var(--mr-space-2);
      font-size: 16px;
      font-weight: 700;
    }

    .mine__text {
      margin: 0;
      font-family: var(--mr-font-display);
      font-weight: 700;
      font-size: 20px;
      line-height: 28px;
    }

    .mine__only {
      display: flex;
      align-items: center;
      gap: 6px;
      margin: var(--mr-space-2) 0 0;
      font-size: 14px;
      color: var(--mr-ink-muted);

      .mat-icon {
        width: 18px;
        height: 18px;
        font-size: 18px;
      }
    }

    .others__list {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-2);
      margin: 0;
      padding: 0;
      list-style: none;
    }

    .others__row {
      display: flex;
      align-items: center;
      gap: var(--mr-space-3);
      min-height: 44px;
    }

    .others__badge {
      flex: none;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      box-sizing: border-box;
      width: 32px;
      height: 32px;
      border: 1.5px solid var(--mr-control-line);
      border-radius: 50%;
      font-family: var(--mr-font-display);
      font-weight: 700;
    }

    .others__name {
      flex: 1;
      min-width: 0;
      font-size: 17px;
      font-weight: 700;
    }

    .others__has {
      font-size: 14px;
      color: var(--mr-ink-muted);
    }

    .others__help {
      margin: var(--mr-space-3) 0 0;
      font-size: 14px;
      line-height: 20px;
      color: var(--mr-ink-muted);
    }
  `,
})
export class MyPart {
  /** This player's own part, or `''`. */
  readonly part = input('');
  /** The names of the other characters that have a part. */
  readonly holders = input<readonly string[]>([]);
}
