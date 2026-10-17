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
  signal,
  untracked,
} from '@angular/core';
import { Code, ConnectError } from '@connectrpc/connect';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { timestampMs } from '@bufbuild/protobuf/wkt';

import {
  RevivifyRequestStatus,
  type RevivifyRequest,
} from '../../../../gen/meurpg/play/v1/revivify_pb';
import { livingRefusal, type LivingRefusal } from '../../../core/characters/character-errors';
import { describeConnectError } from '../../../core/connect/connect-errors';
import { ActionKey } from '../../../core/connect/idempotency';
import { article } from '../../../core/format/article';
import { RevivifyClient } from '../../../core/revivify/revivify-client';
import { ReviveBlocked } from '../../../shared/revive/revive-blocked';

const ASK_ERRORS = {
  [Code.NotFound]: 'Esse pedido não existe mais. Atualize a sessão.',
  [Code.PermissionDenied]: 'Só o mestre responde a este pedido.',
};

/**
 * "Ilaria quer conjurar Revivificar em Toren": the master's question when a player casts Revivify outside a
 * combat. The app does not count time out of combat, so the cast waits for him: nothing is spent until he answers
 * "Faz menos de 1 minuto"; "Já passou" ends it at no cost and the player reads only "O mestre disse que não dá".
 * An `alertdialog` in place of the page's top, with the focus on the first button, and no way to dismiss it: the
 * cast waits for an answer. The page bumps `reload` when the stream says the casts changed (and on each
 * connection), and this reads the list again, showing the oldest cast first.
 *
 * If the target's player made another living character since the death, the server refuses with nothing spent and
 * the card says whom to file first, as on the dead character's page.
 */
@Component({
  selector: 'app-revivify-ask',
  imports: [MatButtonModule, MatIconModule, ReviveBlocked],
  templateUrl: './revivify-ask.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
  styleUrl: './revivify-ask.scss',
})
export class RevivifyAsk {
  private readonly api = inject(RevivifyClient);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);

  readonly campaignId = input.required<string>();
  /** Goes up when the casts may have changed: the list is read again. */
  readonly reload = input(0);

  /** The casts waiting for the master, the oldest first. */
  private readonly waiting = signal<readonly RevivifyRequest[]>([]);
  protected readonly current = computed(() => this.waiting().at(0) ?? null);
  /** How many wait behind the one on screen. */
  protected readonly more = computed(() => Math.max(0, this.waiting().length - 1));
  protected readonly answering = signal<boolean | null>(null);
  protected readonly error = signal<string | null>(null);
  protected readonly refused = signal<LivingRefusal | null>(null);
  /** What a screen reader hears once it is answered. */
  protected readonly announcement = signal('');
  private readonly key = new ActionKey();
  private seq = 0;

  /** "ele" or "ela" for the target, as the other texts do. */
  protected pronoun(r: RevivifyRequest): string {
    return article(r.targetName) === 'a' ? 'ela' : 'ele';
  }

  constructor() {
    effect(() => {
      this.reload();
      const campaignId = this.campaignId();
      untracked(() => void this.read(campaignId));
    });
    // A new question takes the focus, on the first button.
    effect(() => {
      const id = this.current()?.id;
      if (id) {
        untracked(() =>
          afterNextRender(
            () =>
              this.host.nativeElement
                .querySelector<HTMLButtonElement>('.js-within')
                ?.focus({ focusVisible: true } as FocusOptions),
            { injector: this.injector },
          ),
        );
      }
    });
  }

  private async read(campaignId: string): Promise<void> {
    if (!campaignId) {
      return;
    }
    const seq = ++this.seq;
    try {
      const list = await this.api.list(campaignId);
      if (seq !== this.seq) {
        return;
      }
      this.waiting.set(
        list
          .filter((r) => r.status === RevivifyRequestStatus.PENDING)
          .sort((a, b) => ms(a) - ms(b)),
      );
    } catch {
      // Keep what is on screen: the next hint or connection reads again.
    }
  }

  protected async answer(request: RevivifyRequest, withinMinute: boolean): Promise<void> {
    if (this.answering() !== null) {
      return;
    }
    this.answering.set(withinMinute);
    this.error.set(null);
    try {
      const key = this.key.keyFor({ id: request.id, withinMinute });
      await this.api.confirmTime(this.campaignId(), request.id, withinMinute, key);
      this.key.renew();
      this.waiting.update((list) => list.filter((r) => r.id !== request.id));
      this.announcement.set(
        withinMinute
          ? `${request.targetName} voltou à vida`
          : `${request.casterName} lê que o mestre disse que não dá`,
      );
      void this.read(this.campaignId());
    } catch (err) {
      const living = livingRefusal(err);
      if (living) {
        this.refused.set(living);
      } else if (ConnectError.from(err, Code.Unavailable).code === Code.FailedPrecondition) {
        // Answered already (by the master on another screen): there is nothing left to answer.
        void this.read(this.campaignId());
      } else {
        this.error.set(describeConnectError(err, ASK_ERRORS));
      }
    } finally {
      this.answering.set(null);
    }
  }

  protected cancelRefused(): void {
    this.refused.set(null);
  }
}

function ms(r: RevivifyRequest): number {
  return r.createdAt ? timestampMs(r.createdAt) : 0;
}
