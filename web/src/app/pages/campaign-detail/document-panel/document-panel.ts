import { Component, OnInit, computed, inject, input, signal } from '@angular/core';
import { timestampDate } from '@bufbuild/protobuf/wkt';
import { MatIconModule } from '@angular/material/icon';
import { RouterLink } from '@angular/router';

import type { CampaignDocument } from '../../../../gen/meurpg/campaigns/v1/campaign_document_pb';
import { describeConnectError } from '../../../core/connect/connect-errors';
import { DocumentClient } from '../../../shared/campaign-document/document-clients';
import { whenText } from '../../../shared/campaign-document/document-format';

type PanelState =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'ready'; doc: CampaignDocument };

/**
 * The master's "Documento da campanha" row on "Preparação" (E5-09,
 * MR-018; board C6-02): one row that is the link to the document, with "Editado ontem às 22:10 ·
 * só você lê" under the title. The tab renders it for the master only: the server refuses the
 * document to a player.
 */
@Component({
  selector: 'app-document-panel',
  imports: [MatIconModule, RouterLink],
  templateUrl: './document-panel.html',
  styleUrl: './document-panel.scss',
})
export class DocumentPanel implements OnInit {
  private readonly documents = inject(DocumentClient);

  readonly campaignId = input.required<string>();

  protected readonly state = signal<PanelState>({ status: 'loading' });
  /** "Editado ontem às 22:10 · só você lê": the board's line under the title. */
  protected readonly line = computed(() => {
    const s = this.state();
    if (s.status !== 'ready') {
      return '';
    }
    const when = s.doc.updatedAt
      ? `Editado ${whenText(timestampDate(s.doc.updatedAt))}`
      : 'Ainda sem texto';
    return `${when} · só você lê`;
  });

  ngOnInit(): void {
    this.documents.get(this.campaignId()).then(
      (doc) => this.state.set({ status: 'ready', doc }),
      (err: unknown) => this.state.set({ status: 'error', message: describeConnectError(err, {}) }),
    );
  }
}
