import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  Injector,
  OnInit,
  afterNextRender,
  computed,
  inject,
  input,
  signal,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { RouterLink } from '@angular/router';

import { formatDuration } from '../../../core/play/session-summary';
import { dayAndMonth } from '../campaign-detail.copy';
import { CampaignSessions } from '../game-session/campaign-sessions';
import { PHONE_QUERY, mediaQuery } from '../../../shared/map-view/media-query';
import { PanelHead } from '../panel-head/panel-head';

/** How many ended sessions show before "Mostrar as outras N". */
const SHOWN = 5;

interface PastSession {
  readonly id: string;
  readonly number: number;
  readonly span: string;
  readonly duration: string;
}

/**
 * "Sessões anteriores" on `/campaigns/:id` (PM-01): the campaign's ended
 * sessions, newest first, each with its number, the day and hours it ran, how
 * long it lasted and "Resumo" (named "Ver resumo da Sessão N"), which opens `/campaigns/:id/sessions/:n`.
 *
 * It reads the page's list of sessions (`CampaignSessions`, the same answer
 * the "Sessão" panel uses), so it makes no call of its own. The open session
 * is not here: it is the live notice above. With nothing ended, the master
 * sees the panel say so (the summary of the first one will be kept here) and a
 * player sees no panel. The list comes whole from the server; only the five
 * newest show until "Mostrar as outras N".
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-past-sessions-panel',
  imports: [MatButtonModule, MatIconModule, PanelHead, RouterLink],
  templateUrl: './past-sessions-panel.html',
  styleUrl: './past-sessions-panel.scss',
  host: { '[class.is-empty]': '!visible()' },
})
export class PastSessionsPanel implements OnInit {
  protected readonly sessions = inject(CampaignSessions);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);

  readonly campaignId = input.required<string>();
  /** The master sees the panel even with nothing ended; a player does not. */
  readonly isMaster = input(true);

  protected readonly expanded = signal(false);
  /** A phone folds the panel to one row ("Sessões anteriores · 3"); it opens in place. */
  private readonly phone = mediaQuery(PHONE_QUERY);
  /** Only the master's phone folds it: a player's page keeps its panels as they are. */
  protected readonly foldable = computed(() => this.phone() && this.isMaster());
  protected readonly open = signal(false);

  private readonly all = computed<PastSession[]>(() =>
    this.sessions.ended().map((s) => {
      const end = s.endedAt ?? s.startedAt;
      return {
        id: s.id,
        number: s.sessionNumber,
        span: dayAndMonth(s.startedAt),
        duration: formatDuration(Math.max(0, (end.getTime() - s.startedAt.getTime()) / 1000)),
      };
    }),
  );

  protected readonly rows = computed(() =>
    this.expanded() ? this.all() : this.all().slice(0, SHOWN),
  );
  protected readonly hidden = computed(() => this.all().length - this.rows().length);
  protected readonly count = computed(() => {
    const n = this.all().length;
    return this.foldable() ? `${n}` : `${n} ${n === 1 ? 'encerrada' : 'encerradas'}`;
  });

  protected readonly visible = computed(
    () => this.isMaster() || this.sessions.state().status !== 'ready' || this.all().length > 0,
  );

  ngOnInit(): void {
    void this.sessions.ensureLoaded(this.campaignId());
  }

  protected showAll(): void {
    this.expanded.set(true);
    // The button goes away: the focus lands on the first session that was not there.
    afterNextRender(
      () => this.host.nativeElement.querySelector<HTMLElement>('.js-first-extra')?.focus(),
      { injector: this.injector },
    );
  }

  protected retry(): void {
    void this.sessions.reload(this.campaignId());
  }

  protected readonly shown = SHOWN;
}
