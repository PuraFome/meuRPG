import {
  ChangeDetectionStrategy,
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

import { article } from '../../core/format/article';

/**
 * "Reviver Toren?": the master's question in place of the button that asked it, on the dead character's page and
 * in the list of the dead of a combat. It says in four lines what comes back and what does not. The focus opens on
 * "Reviver Toren" (outlined: marking the character dead again undoes it, so this is not an act without a way back)
 * and Escape cancels; the parent sends the call, hands back `busy`, and puts the focus back on the button that
 * asked.
 */
@Component({
  selector: 'app-revive-confirm',
  imports: [MatButtonModule, MatIconModule],
  templateUrl: './revive-confirm.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
  styleUrl: './revive-confirm.scss',
})
export class ReviveConfirm {
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);

  readonly characterName = input.required<string>();
  /** The call is on its way. */
  readonly busy = input(false);
  readonly confirmed = output<void>();
  readonly cancelled = output<void>();

  /** "ele" or "ela", by the name, as the log writes "o Goblin" and "a Brisa". */
  protected readonly pronoun = computed(() =>
    article(this.characterName()) === 'a' ? 'ela' : 'ele',
  );

  constructor() {
    afterNextRender(
      () => {
        this.host.nativeElement.scrollIntoView?.({ block: 'nearest' });
        this.host.nativeElement
          .querySelector<HTMLButtonElement>('.js-confirm-revive')
          ?.focus({ focusVisible: true, preventScroll: true } as FocusOptions);
      },
      { injector: this.injector },
    );
  }
}
