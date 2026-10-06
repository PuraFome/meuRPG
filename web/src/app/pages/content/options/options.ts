import { Component, DestroyRef, ElementRef, Injector, afterNextRender, computed, inject, signal, viewChild } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { Code, ConnectError } from '@connectrpc/connect';

import { Role } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { type OptionSwitchEntry, TableContentKind } from '../../../../gen/meurpg/rules/v1/table_content_pb';
import { CampaignsService } from '../../../core/campaigns/campaigns.service';
import { TableContentClient, contentErrorText } from '../../../core/content/content-client';
import { CONTENT_NAV, type ContentNavKind, KIND_WORDS, navBySlug } from '../../../core/content/content-kinds';
import { ContentWatcher } from '../../../core/content/content-watcher';
import {
  OptionSwitchesState,
  counterText,
  groupRows,
  hiddenNote,
  mainCount,
  menuCount,
  searchRows,
  subraceCount,
  subraceCounterText,
  usingText,
} from '../../../core/content/option-switches';
import { SelectField, type SelectOption } from '../../../shared/form-fields/select-field';
import { SwitchField } from '../../../shared/form-fields/switch-field';
import { TextField } from '../../../shared/form-fields/text-field';
import { mediaQuery, PHONE_QUERY } from '../../../shared/map-view/media-query';
import { LiveSessionSourceLive } from '../../live-session/live-session-source.live';

type Access =
  | { status: 'loading' }
  | { status: 'not-found' }
  | { status: 'not-master'; campaignName: string }
  | { status: 'error'; message: string }
  | { status: 'ok'; campaignName: string };

/**
 * "Opções para os jogadores" (MR-025, RN-23; E10-01 state 3): the master's one switch per class, subclass, race, sub-race,
 * background and spell, the SRD's and the table's together, by kind. What is on the players read in full and may choose; what is
 * off never reaches them, and there is no switch per player. Each row says how many characters use the option (turning it off
 * touches no sheet), the group says "Raças: 9 de 10 ligadas" and has "Ligar todas" and "Desligar todas", the search narrows by
 * name, and every change is saved at once ("Tudo salvo"). A subclass or sub-race under an off class or race says that it is
 * hidden for that reason and keeps its own switch. The page reads again when the table changes (`content_changed`).
 */
@Component({
  selector: 'app-content-options',
  imports: [MatButtonModule, MatIconModule, MatProgressSpinnerModule, RouterLink, SelectField, SwitchField, TextField],
  providers: [ContentWatcher, LiveSessionSourceLive],
  templateUrl: './options.html',
  styleUrl: './options.scss',
})
export class ContentOptions {
  private readonly campaigns = inject(CampaignsService);
  private readonly client = inject(TableContentClient);
  private readonly route = inject(ActivatedRoute);
  private readonly injector = inject(Injector);
  private readonly watcher = inject(ContentWatcher);
  private readonly title = viewChild<ElementRef<HTMLElement>>('title');

  protected readonly nav = CONTENT_NAV;
  protected readonly phone = mediaQuery(PHONE_QUERY);
  protected readonly campaignId = signal('');
  protected readonly access = signal<Access>({ status: 'loading' });
  protected readonly slug = signal<ContentNavKind['slug']>('classes');
  protected readonly query = signal('');
  protected readonly kindWords = KIND_WORDS;
  protected readonly Kind = TableContentKind;

  private state: OptionSwitchesState | null = null;
  /** The state is made once the campaign is known; this signal carries its options to the template. */
  private readonly stateSignal = signal<OptionSwitchesState | null>(null);

  protected readonly options = computed(() => this.stateSignal()?.options() ?? []);
  protected readonly status = computed(() => this.stateSignal()?.status() ?? 'loading');
  protected readonly loadError = computed(() => this.stateSignal()?.loadError() ?? '');
  protected readonly save = computed(() => this.stateSignal()?.save() ?? { kind: 'idle' as const });
  protected readonly announce = computed(() => this.stateSignal()?.announce() ?? '');

  protected readonly current = computed(() => navBySlug(this.slug()) ?? CONTENT_NAV[0]);
  protected readonly byKey = computed(() => new Map(this.options().map((o) => [o.key, o])));
  protected readonly rowsAll = computed(() => groupRows(this.options(), this.current()));
  protected readonly rows = computed(() => searchRows(this.rowsAll(), this.query()));
  protected readonly counter = computed(() => counterText(this.current(), mainCount(this.options(), this.current())));
  protected readonly subraces = computed(() => {
    const c = subraceCount(this.options());
    return this.current().slug === 'racas' && c.total > 0 ? subraceCounterText(c) : '';
  });
  protected readonly menuCounts = computed(() => new Map(CONTENT_NAV.map((n) => [n.slug, menuCount(this.options(), n)])));
  protected readonly kindOptions = computed<SelectOption[]>(() => CONTENT_NAV.map((n) => ({ value: n.slug, label: `${n.plural} · ${this.menuCounts().get(n.slug)}` })));
  protected readonly searching = computed(() => this.query().trim() !== '');
  /** "Ligar todas" acts on what the list shows: with a search on, only on the rows found, and the buttons say so. */
  protected readonly bulkSuffix = computed(() => (this.searching() ? ` (${this.rows().length})` : ''));
  protected readonly bulkDisabled = computed(() => this.rows().length === 0);
  protected readonly onWord = computed(() => (this.current().slug === 'antecedentes' ? 'Ligado' : 'Ligada'));
  protected readonly offWord = computed(() => (this.current().slug === 'antecedentes' ? 'Desligado' : 'Desligada'));

  constructor() {
    this.route.paramMap.pipe(takeUntilDestroyed(inject(DestroyRef))).subscribe((params) => {
      const id = params.get('id');
      if (id) {
        this.campaignId.set(id);
        void this.start(id);
      }
    });
    this.route.queryParamMap.pipe(takeUntilDestroyed(inject(DestroyRef))).subscribe((params) => {
      const n = navBySlug(params.get('tipo'));
      if (n) {
        this.slug.set(n.slug);
        this.query.set('');
      }
    });
    // The table changed (an entry written, a switch turned): the list is read again, without a spinner.
    this.watcher.whileLive(this.campaignId, () => void this.state?.load(true));
  }

  protected async start(id: string): Promise<void> {
    this.access.set({ status: 'loading' });
    try {
      const { campaign } = await this.campaigns.getCampaign(id);
      if (!campaign || campaign.awaitingApproval) {
        this.access.set({ status: 'not-found' });
        return;
      }
      if (campaign.myRole !== Role.MASTER) {
        this.access.set({ status: 'not-master', campaignName: campaign.name });
        return;
      }
      this.state = new OptionSwitchesState(this.client, id, (err) => contentErrorText(err, 'salvar'));
      this.stateSignal.set(this.state);
      this.access.set({ status: 'ok', campaignName: campaign.name });
      await this.state.load();
      afterNextRender(() => this.title()?.nativeElement.focus({ preventScroll: true }), { injector: this.injector });
    } catch (err) {
      this.access.set(
        ConnectError.from(err, Code.Unavailable).code === Code.NotFound
          ? { status: 'not-found' }
          : { status: 'error', message: contentErrorText(err, 'abrir as opções') },
      );
    }
  }

  protected retry(): void {
    const id = this.campaignId();
    if (this.access().status === 'ok') {
      void this.state?.load();
    } else {
      void this.start(id);
    }
  }

  protected back(): string[] {
    return ['/campanhas', this.campaignId(), 'conteudo'];
  }

  protected setQuery(text: string): void {
    this.query.set(text);
  }

  protected pickKind(slug: string): void {
    const n = navBySlug(slug);
    if (n) {
      this.slug.set(n.slug);
      this.query.set('');
    }
  }

  protected toggle(o: OptionSwitchEntry, on: boolean): void {
    void this.state?.toggle(o.key, !on);
  }

  protected setAll(off: boolean): void {
    void this.state?.setAll(this.rows(), off, this.current().plural);
  }

  protected using(o: OptionSwitchEntry): string {
    return usingText(o.charactersUsing);
  }

  protected note(o: OptionSwitchEntry): string {
    const hidden = hiddenNote(o, this.byKey());
    if (hidden) {
      return hidden;
    }
    if (o.off && o.charactersUsing > 0) {
      return o.charactersUsing === 1 ? 'A ficha que usa continua funcionando.' : 'As fichas que usam continuam funcionando.';
    }
    if (o.archived) {
      return 'Arquivada: os jogadores não a recebem.';
    }
    return '';
  }

  protected kindLabel(o: OptionSwitchEntry): string {
    return o.kind === TableContentKind.SUBRACE || o.kind === TableContentKind.SUBCLASS ? this.parentName(o) : '';
  }

  private parentName(o: OptionSwitchEntry): string {
    const parent = o.parentKey ? this.byKey().get(o.parentKey) : undefined;
    if (!parent) {
      return '';
    }
    return o.kind === TableContentKind.SUBRACE ? `Sub-raça de ${parent.namePt}` : `Subclasse de ${parent.namePt}`;
  }
}
