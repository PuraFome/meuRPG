import {
  ChangeDetectionStrategy,
  Component,
  OnInit,
  computed,
  inject,
  input,
  signal,
} from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { XpMode } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { TableRulesClient, type RulesDraft } from '../../../core/campaigns/table-rules';
import { describeConnectError } from '../../../core/connect/connect-errors';
import { PanelAction } from '../panel-action/panel-action';
import { PanelHead } from '../panel-head/panel-head';
import { ruleSummaryRows } from './rules-summary.copy';

type SummaryState =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'ready'; rules: RulesDraft };

/**
 * "Regras da mesa" on the master's "Regras" tab (board C6-03): six of the table's rules in words
 * (hit points, ability scores, critical hits, XP, fog, reminders) and "Abrir as regras", in the
 * title row, which opens the page where they are changed. It reads the rules itself
 * (`GetTableRules`); the XP mode comes from the campaign the page already has.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-rules-summary',
  imports: [MatIconModule, PanelAction, PanelHead],
  templateUrl: './rules-summary.html',
  styleUrl: './rules-summary.scss',
})
export class RulesSummary implements OnInit {
  private readonly client = inject(TableRulesClient);

  readonly campaignId = input.required<string>();
  readonly xpMode = input.required<XpMode>();

  protected readonly state = signal<SummaryState>({ status: 'loading' });
  protected readonly rows = computed(() => {
    const s = this.state();
    return s.status === 'ready' ? ruleSummaryRows(s.rules, this.xpMode()) : [];
  });

  ngOnInit(): void {
    this.client.get(this.campaignId()).then(
      (vm) => this.state.set({ status: 'ready', rules: vm.saved }),
      (err: unknown) => this.state.set({ status: 'error', message: describeConnectError(err, {}) }),
    );
  }
}
