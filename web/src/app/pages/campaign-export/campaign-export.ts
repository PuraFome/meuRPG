import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  inject,
  signal,
  viewChild,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { Code, ConnectError } from '@connectrpc/connect';

import {
  type CampaignExport,
  CampaignExportFailure,
  CampaignExportState,
} from '../../../gen/meurpg/campaignpackage/v1/campaignpackage_pb';
import { Role } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CampaignPackageClient } from '../../core/campaign-package/campaign-package-client';
import {
  approxSize,
  exportPhase,
  expiryText,
  lastPackageText,
  suggestedFileName,
} from '../../core/campaign-package/package-copy';
import { exportErrorText } from '../../core/campaign-package/package-errors';
import { MAX_PACKAGE_BYTES } from '../../core/campaign-package/package-problems';
import { CampaignsService } from '../../core/campaigns/campaigns.service';
import { ActionKey } from '../../core/connect/idempotency';
import { formatBytes } from '../../core/images/image-format';

type PageState =
  | { status: 'loading' }
  | { status: 'not-found' }
  | { status: 'forbidden' }
  | { status: 'error'; message: string }
  | { status: 'ready' };

/** The board's two lists (decision 7): what goes into the package, and what never does. */
const GOES: readonly string[] = [
  'O documento, as anotações do mestre e a galeria (imagens e miniaturas)',
  'Os mapas: imagem, grade, camadas, portas, luzes, armadilhas e pontos',
  'Os NPCs e criaturas',
  'As cenas, suas ações, pistas e ganchos',
  'Os quebra-cabeças',
  'Os pontos de batalha e os encontros',
  'Os pontos de tesouro',
  'As regras da mesa e o conteúdo da mesa',
];

const NEVER: readonly string[] = [
  'As anotações privadas dos jogadores',
  'O histórico das sessões e os registros',
  'As contas: ids, e-mails e nomes',
  'Quem é dono e quem é membro',
  'Convites e links de personagem',
  'A memória de névoa de cada jogador',
];

/** While an export runs, the page asks the server where it is this often. */
export const EXPORT_POLL_MS = 2000;

/** The error codes that no new poll fixes: the page stops asking and says so. */
const STOP_POLLING: ReadonlySet<Code> = new Set([
  Code.Unauthenticated,
  Code.NotFound,
  Code.PermissionDenied,
]);

/**
 * "/campaigns/:id/export" (MR-050, PM-09 state 1), master only: what goes into the package and what never does,
 * the file's name with its size ("cerca de N MB"), "Exportar campanha", the running export with its progress
 * and "Cancelar", and the last package with "Baixar de novo" until it expires (24 hours).
 *
 * The export runs on the server, so the master may leave: coming back, `GetCampaignExport` shows where it is.
 * While it runs the page polls it every `EXPORT_POLL_MS`, with a timer that is cleared when the page goes (a
 * spec fakes the clock). The download is a plain link to the export's `download_path`: the server checks again
 * that the caller is the master and answers `Content-Disposition: attachment`.
 *
 * A player gets the calm "Só o mestre exporta a campanha." (the same as the gallery's "Só o mestre vê"), a
 * non-member and a campaign that does not exist both get "campanha não encontrada" (ADR-0011).
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-campaign-export',
  imports: [MatButtonModule, MatIconModule, MatProgressSpinnerModule, RouterLink],
  templateUrl: './campaign-export.html',
  styleUrl: './campaign-export.scss',
})
export class CampaignExportPage {
  private readonly campaigns = inject(CampaignsService);
  private readonly client = inject(CampaignPackageClient);
  private readonly route = inject(ActivatedRoute);
  private readonly injector = inject(Injector);
  private readonly startKey = new ActionKey();
  private pollTimer: ReturnType<typeof setTimeout> | undefined;
  private gone = false;

  protected readonly State = CampaignExportState;
  protected readonly Failure = CampaignExportFailure;
  protected readonly goes = GOES;
  protected readonly never = NEVER;

  protected readonly campaignId = signal('');
  protected readonly campaignName = signal('');
  protected readonly state = signal<PageState>({ status: 'loading' });
  protected readonly latest = signal<CampaignExport | null>(null);
  protected readonly estimatedBytes = signal(0);
  protected readonly limitBytes = signal(MAX_PACKAGE_BYTES);
  protected readonly starting = signal(false);
  protected readonly cancelling = signal(false);
  /** What a failed "Exportar campanha" or "Cancelar" said. */
  protected readonly actionError = signal('');
  /** Said under the progress when one poll could not reach the server; the next one clears it. */
  protected readonly pollNote = signal('');
  /** The polite live region for a change the cards do not announce themselves: done, failed, canceled. */
  protected readonly announcement = signal('');

  protected readonly running = computed(() => this.latest()?.state === CampaignExportState.RUNNING);
  protected readonly fileName = computed(() => suggestedFileName(this.campaignName()));
  protected readonly sizeWords = computed(() => {
    const bytes = this.estimatedBytes();
    return bytes > 0 ? approxSize(bytes) : '';
  });
  protected readonly limitText = computed(() => formatBytes(this.limitBytes()));
  protected readonly percent = computed(() => this.latest()?.percent ?? 0);
  protected readonly phase = computed(() => exportPhase(this.percent()));
  protected readonly lastDetails = computed(() => {
    const exp = this.latest();
    return exp ? lastPackageText(exp) : '';
  });
  protected readonly expiry = computed(() => expiryText(this.latest()?.expiresAt));

  private readonly primary = viewChild('primary', { read: ElementRef<HTMLElement> });

  constructor() {
    this.route.paramMap.pipe(takeUntilDestroyed()).subscribe((params) => {
      const id = params.get('id');
      if (id) {
        this.campaignId.set(id);
        void this.load(id);
      }
    });
    inject(DestroyRef).onDestroy(() => {
      this.gone = true;
      clearTimeout(this.pollTimer);
    });
  }

  protected async load(id = this.campaignId()): Promise<void> {
    clearTimeout(this.pollTimer);
    this.state.set({ status: 'loading' });
    try {
      const { campaign } = await this.campaigns.getCampaign(id);
      if (id !== this.campaignId()) {
        return;
      }
      if (!campaign || campaign.awaitingApproval) {
        this.state.set({ status: 'not-found' });
        return;
      }
      if (campaign.myRole !== Role.MASTER) {
        this.state.set({ status: 'forbidden' });
        return;
      }
      this.campaignName.set(campaign.name);
      const res = await this.client.getExport(id);
      if (id !== this.campaignId()) {
        return;
      }
      this.take(res.export ?? null, res.estimatedBytes, res.limitBytes);
      this.state.set({ status: 'ready' });
      this.focusPrimary();
      this.schedulePoll();
    } catch (err) {
      if (id === this.campaignId()) {
        this.state.set(this.stateFor(err));
      }
    }
  }

  private stateFor(err: unknown): PageState {
    const code = ConnectError.from(err, Code.Unavailable).code;
    if (code === Code.NotFound) {
      return { status: 'not-found' };
    }
    if (code === Code.PermissionDenied) {
      return { status: 'forbidden' };
    }
    return { status: 'error', message: exportErrorText(err) };
  }

  private take(exp: CampaignExport | null, estimated?: bigint, limit?: bigint): void {
    this.latest.set(exp);
    if (estimated !== undefined) {
      this.estimatedBytes.set(Number(estimated));
    }
    if (limit !== undefined && Number(limit) > 0) {
      this.limitBytes.set(Number(limit));
    }
  }

  private focusPrimary(): void {
    afterNextRender(() => this.primary()?.nativeElement.focus(), { injector: this.injector });
  }

  protected async start(): Promise<void> {
    if (this.starting() || this.running()) {
      return;
    }
    this.starting.set(true);
    this.actionError.set('');
    try {
      // One key per export: a second tap or a retry after a lost answer is the same export.
      const res = await this.client.startExport(
        this.campaignId(),
        this.startKey.keyFor([this.campaignId()]),
      );
      this.startKey.renew();
      this.take(res.export ?? null);
      this.pollNote.set('');
      this.schedulePoll();
    } catch (err) {
      this.actionError.set(exportErrorText(err));
    } finally {
      this.starting.set(false);
    }
  }

  protected async cancel(): Promise<void> {
    const exp = this.latest();
    if (!exp || this.cancelling()) {
      return;
    }
    this.cancelling.set(true);
    this.actionError.set('');
    try {
      const res = await this.client.cancelExport(this.campaignId(), exp.id);
      this.settle(res.export ?? null);
      this.focusPrimary();
    } catch (err) {
      this.actionError.set(exportErrorText(err));
    } finally {
      this.cancelling.set(false);
    }
  }

  private schedulePoll(): void {
    clearTimeout(this.pollTimer);
    if (this.gone || !this.running()) {
      return;
    }
    this.pollTimer = setTimeout(() => void this.poll(), EXPORT_POLL_MS);
  }

  private async poll(): Promise<void> {
    const id = this.campaignId();
    try {
      const res = await this.client.getExport(id);
      if (this.gone || id !== this.campaignId()) {
        return;
      }
      this.pollNote.set('');
      this.settle(res.export ?? null, res.estimatedBytes);
    } catch (err) {
      if (this.gone || id !== this.campaignId()) {
        return;
      }
      if (STOP_POLLING.has(ConnectError.from(err, Code.Unavailable).code)) {
        this.state.set(this.stateFor(err));
        return;
      }
      this.pollNote.set('Sem conexão com o servidor agora. Tentando de novo.');
    }
    this.schedulePoll();
  }

  /** Takes a newer reading of the export; a change of state is announced, and a running one keeps polling. */
  private settle(exp: CampaignExport | null, estimated?: bigint): void {
    const before = this.latest()?.state;
    this.take(exp, estimated);
    if (exp && exp.state !== before) {
      this.announcement.set(announcementFor(exp));
    }
    this.schedulePoll();
  }
}

function announcementFor(exp: CampaignExport): string {
  switch (exp.state) {
    case CampaignExportState.DONE:
      return 'O pacote está pronto para baixar.';
    case CampaignExportState.FAILED:
      return 'A exportação não terminou.';
    case CampaignExportState.CANCELED:
      return 'Exportação cancelada.';
    default:
      return '';
  }
}
