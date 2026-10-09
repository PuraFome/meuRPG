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
import { Code, ConnectError } from '@connectrpc/connect';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { ActivatedRoute, RouterLink } from '@angular/router';

import { Role } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  type AffectedCharacter,
  type ImportTableContentResponse,
  TableImportStatus,
  type TableImportEntry,
} from '../../../../gen/meurpg/rules/v1/table_content_pb';
import { CampaignsService } from '../../../core/campaigns/campaigns.service';
import { ActionKey } from '../../../core/connect/idempotency';
import {
  TableContentClient,
  contentErrorText,
  refusalOf,
} from '../../../core/content/content-client';
import { KIND_NOUNS } from '../../../core/content/content-kinds';
import {
  changedText,
  countsByKind,
  packViolationText,
  parsePack,
} from '../../../core/content/content-pack';

type Access =
  | { status: 'loading' }
  | { status: 'not-found' }
  | { status: 'not-master'; campaignName: string }
  | { status: 'error'; message: string }
  | { status: 'ok'; campaignName: string };

/** The page after a file is chosen: the preview, the apply, the result. */
type Step =
  | { step: 'choose' }
  | { step: 'checking'; fileName: string }
  | { step: 'preview'; fileName: string; json: Uint8Array; res: ImportTableContentResponse }
  | { step: 'applying'; fileName: string; json: Uint8Array; res: ImportTableContentResponse }
  | { step: 'done'; res: ImportTableContentResponse };

/** One group of the preview: its heading, the word that says what happens and the entries. */
interface Group {
  readonly key: string;
  readonly title: string;
  readonly rows: readonly TableImportEntry[];
}

/**
 * "Importar pacote" (MR-025): the master chooses a content pack file, the page shows what the import would do with every
 * entry (new, updated and what changes, unchanged, refused and why), and only then "Importar N entradas" writes it, all or
 * nothing. The server judges each entry with the rules of the editors; a refused entry blocks the import until the file is
 * fixed and chosen again. The characters left with issues are listed like after an edit.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-content-import',
  imports: [MatButtonModule, MatIconModule, MatProgressSpinnerModule, RouterLink],
  templateUrl: './content-import.html',
  styleUrl: './content-import.scss',
})
export class ContentImport {
  private readonly campaigns = inject(CampaignsService);
  private readonly client = inject(TableContentClient);
  private readonly route = inject(ActivatedRoute);
  private readonly injector = inject(Injector);
  private readonly title = viewChild<ElementRef<HTMLElement>>('title');
  private readonly applyKey = new ActionKey();

  protected readonly campaignId = signal('');
  protected readonly access = signal<Access>({ status: 'loading' });
  protected readonly state = signal<Step>({ step: 'choose' });
  /** What is wrong with the file or the call, in words. */
  protected readonly problem = signal('');
  /** The pack-level reasons the server gave (the file's size, format, version, keys). */
  protected readonly packProblems = signal<readonly string[]>([]);

  protected readonly back = computed(() => ['/campaigns', this.campaignId(), 'content']);
  protected readonly preview = computed(() => {
    const s = this.state();
    return s.step === 'preview' || s.step === 'applying' ? s : null;
  });
  protected readonly done = computed(() => {
    const s = this.state();
    return s.step === 'done' ? s.res : null;
  });
  protected readonly groups = computed<Group[]>(() => {
    const p = this.preview();
    if (!p) {
      return [];
    }
    const by = (status: TableImportStatus) => p.res.entries.filter((e) => e.status === status);
    return [
      { key: 'refused', title: 'Recusadas', rows: by(TableImportStatus.REFUSED) },
      { key: 'new', title: 'Novas', rows: by(TableImportStatus.NEW) },
      { key: 'updated', title: 'Atualizadas', rows: by(TableImportStatus.UPDATED) },
      { key: 'unchanged', title: 'Sem mudança', rows: by(TableImportStatus.UNCHANGED) },
    ].filter((g) => g.rows.length > 0);
  });
  protected readonly kinds = computed(() => countsByKind(this.preview()?.res.entries ?? []));
  protected readonly refused = computed(() => this.preview()?.res.totals?.refused ?? 0);
  /** What "Importar N entradas" writes: the new and the updated ones (the unchanged write nothing). */
  protected readonly writes = computed(() => {
    const t = this.preview()?.res.totals;
    return (t?.new ?? 0) + (t?.updated ?? 0);
  });
  protected readonly applyLabel = computed(() => {
    const n = this.writes();
    return `Importar ${n} ${n === 1 ? 'entrada' : 'entradas'}`;
  });
  protected readonly busy = computed(() => {
    const s = this.state().step;
    return s === 'checking' || s === 'applying';
  });

  constructor() {
    this.route.paramMap.pipe(takeUntilDestroyed(inject(DestroyRef))).subscribe((params) => {
      const id = params.get('id');
      if (id) {
        this.campaignId.set(id);
        void this.start(id);
      }
    });
  }

  protected async start(id: string): Promise<void> {
    this.access.set({ status: 'loading' });
    try {
      const { campaign } = await this.campaigns.getCampaign(id);
      if (!campaign || campaign.awaitingApproval) {
        this.access.set({ status: 'not-found' });
        return;
      }
      this.access.set(
        campaign.myRole === Role.MASTER
          ? { status: 'ok', campaignName: campaign.name }
          : { status: 'not-master', campaignName: campaign.name },
      );
      afterNextRender(() => this.title()?.nativeElement.focus({ preventScroll: true }), {
        injector: this.injector,
      });
    } catch (err) {
      this.access.set(
        ConnectError.from(err, Code.Unavailable).code === Code.NotFound
          ? { status: 'not-found' }
          : { status: 'error', message: contentErrorText(err, 'abrir a importação') },
      );
    }
  }

  protected retry(): void {
    void this.start(this.campaignId());
  }

  /** The file the master chose: checked here, then the server previews every entry. */
  protected async choose(input: HTMLInputElement): Promise<void> {
    const file = input.files?.[0];
    input.value = '';
    if (!file) {
      return;
    }
    this.problem.set('');
    this.packProblems.set([]);
    this.state.set({ step: 'checking', fileName: file.name });
    const parsed = parsePack(await file.text(), file.size);
    if (!parsed.ok) {
      this.problem.set(parsed.message);
      this.state.set({ step: 'choose' });
      return;
    }
    try {
      // The file's own bytes go to the server, which reads them strictly.
      const json = new TextEncoder().encode(parsed.text);
      const res = await this.client.importPack(this.campaignId(), json, 'preview');
      this.state.set({ step: 'preview', fileName: file.name, json, res });
    } catch (err) {
      this.fail(err, 'ler o pacote');
      this.state.set({ step: 'choose' });
    }
  }

  protected async apply(): Promise<void> {
    const s = this.state();
    if (s.step !== 'preview' || this.refused() > 0) {
      return;
    }
    this.problem.set('');
    this.state.set({ ...s, step: 'applying' });
    const key = this.applyKey.keyFor({
      campaignId: this.campaignId(),
      pack: new TextDecoder().decode(s.json),
    });
    try {
      const res = await this.client.importPack(this.campaignId(), s.json, 'apply', key);
      this.applyKey.renew();
      this.state.set({ step: 'done', res });
      afterNextRender(() => this.title()?.nativeElement.focus({ preventScroll: true }), {
        injector: this.injector,
      });
    } catch (err) {
      this.fail(err, 'importar o pacote');
      this.state.set({ ...s, step: 'preview' });
    }
  }

  /** Back to the first step, to choose another file. */
  protected again(): void {
    this.problem.set('');
    this.packProblems.set([]);
    this.state.set({ step: 'choose' });
  }

  private fail(err: unknown, what: string): void {
    const violations = refusalOf(err);
    if (violations) {
      const texts = new Set(violations.map((v) => packViolationText(v)));
      this.packProblems.set([...texts]);
      this.problem.set(
        'Não foi possível importar este arquivo. Corrija o que está abaixo e escolha o arquivo de novo.',
      );
      return;
    }
    this.problem.set(contentErrorText(err, what));
  }

  protected kindWord(e: TableImportEntry): string {
    const n = KIND_NOUNS[e.kind];
    return n ? n.noun.charAt(0).toUpperCase() + n.noun.slice(1) : '';
  }

  protected changed(e: TableImportEntry): string {
    return `Muda ${changedText(e.changedFields)}.`;
  }

  protected reason(v: { field: string; reason: string }, e: TableImportEntry): string {
    return packViolationText(
      v,
      `${KIND_NOUNS[e.kind]?.article === 'O' ? 'um' : 'uma'} ${KIND_NOUNS[e.kind]?.noun ?? 'entrada'}`,
    );
  }

  protected characterLink(a: AffectedCharacter): string[] {
    return ['/campaigns', this.campaignId(), 'characters', a.characterId];
  }
}
