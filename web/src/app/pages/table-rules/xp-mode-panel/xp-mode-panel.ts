import { Component, ElementRef, inject, input, signal, viewChild } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';
import { timestampDate } from '@bufbuild/protobuf/wkt';

import { XpMode } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import type { DiceOption } from '../../../core/campaigns/dice-labels';
import {
  TableRulesClient,
  XP_MODE_WORDS,
  tableRulesError,
  xpModeBlocked,
  type XpModeBlocked,
} from '../../../core/campaigns/table-rules';
import { formatXp } from '../../../core/format/text';
import { DiceChoice } from '../../../shared/dice-choice/dice-choice';
import { MapAsk } from '../../maps/map-ask/map-ask';

const OPTIONS: readonly DiceOption<XpMode>[] = [
  { value: XpMode.ENEMIES, title: 'Por inimigos', description: 'Cada inimigo derrotado dá XP.' },
  { value: XpMode.MILESTONES, title: 'Por marcos', description: 'O nível sobe quando você marca um marco.' },
  { value: XpMode.GOLD, title: 'Por ouro', description: 'O ouro encontrado vira XP em “Voltar à cidade”.' },
];

const WHEN = new Intl.DateTimeFormat('pt-BR', { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' });

/**
 * "Experiência" on "Regras da mesa" (RN-09, E10-03 state 3): the three ways the campaign awards XP, changed after the
 * campaign was made. The change has its own flow, apart from "Salvar regras": a switch with nothing awarded yet is made at
 * once; with XP already given the server answers `XpModeChangeBlocked`, and the question opens in place ("Mudar para
 * “por marcos”?", "Voltar" and the one filled button), with the focus on its title. What was awarded stays as it is.
 */
@Component({
  selector: 'app-xp-mode-panel',
  imports: [DiceChoice, MapAsk, MatIconModule],
  templateUrl: './xp-mode-panel.html',
  styleUrl: './xp-mode-panel.scss',
})
export class XpModePanel {
  private readonly client = inject(TableRulesClient);

  readonly campaignId = input.required<string>();
  /** The mode the campaign has now. */
  readonly mode = input.required<XpMode>();

  protected readonly options = OPTIONS;
  protected readonly current = signal<XpMode | null>(null);
  protected readonly asking = signal<{ target: XpMode; blocked: XpModeBlocked } | null>(null);
  protected readonly busy = signal(false);
  protected readonly error = signal('');
  protected readonly changedLine = signal('');
  private readonly group = viewChild<ElementRef<HTMLElement>>('group');

  /** The card on: the one the question is about while it is open (the choice is not made yet), else the mode in force. */
  protected readonly shown = () => this.asking()?.target ?? this.current() ?? this.mode();
  protected readonly word = (m: XpMode): string => XP_MODE_WORDS[m] ?? '';
  protected readonly xp = formatXp;

  protected async choose(target: XpMode): Promise<void> {
    if (target === (this.current() ?? this.mode()) || this.busy()) {
      return;
    }
    this.error.set('');
    this.changedLine.set('');
    await this.send(target, false);
  }

  protected async confirm(): Promise<void> {
    const a = this.asking();
    if (a && !this.busy()) {
      await this.send(a.target, true);
    }
  }

  protected cancel(): void {
    this.asking.set(null);
    this.error.set('');
    queueMicrotask(() => this.group()?.nativeElement.querySelector<HTMLInputElement>('input:checked')?.focus());
  }

  private async send(target: XpMode, confirm: boolean): Promise<void> {
    this.busy.set(true);
    try {
      const res = await this.client.setXpMode(this.campaignId(), target, confirm);
      this.current.set(res.xpMode);
      this.asking.set(null);
      const at = res.changedAt ? WHEN.format(timestampDate(res.changedAt)).replace(',', ' às') : '';
      this.changedLine.set(
        `Modo de XP mudado para ${XP_MODE_WORDS[res.xpMode]}${at ? `, em ${at}` : ''}. Vale daqui para frente.`,
      );
    } catch (err) {
      const blocked = xpModeBlocked(err);
      if (blocked && !confirm) {
        this.asking.set({ target, blocked });
      } else {
        this.error.set(tableRulesError(err, 'mudar o modo de XP'));
      }
    } finally {
      this.busy.set(false);
    }
  }
}
