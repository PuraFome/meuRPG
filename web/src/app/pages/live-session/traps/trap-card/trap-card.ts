import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import { type GetTrapNoticersResponse, type MapPoint, TrapState } from '../../../../../gen/meurpg/maps/v1/maps_pb';
import { joinDots } from '../../../../core/format/text';
import {
  areaWords,
  effectLine,
  firstLine,
  trapDcLine,
  trapStateIcon,
  trapStateWord,
  trapVisibility,
  triggerWord,
} from '../../../../core/traps/trap-text';
import { TrapNoticers } from '../../../../shared/trap-noticers/trap-noticers';

/**
 * One trap on the master's "Armadilhas do mapa" (E9-08 1, 5, MR-035): its name and where it stands in play
 * (Armada, Disparada às 21:31, Desarmada), who knows it, the DCs, the trigger and the effect in words,
 * "Quem notaria" and the three equal-width actions ("Revelar para…", "Disparar…", "Desarmar"). A fired
 * trap offers "Marcar como desarmada" and "Incluir mais personagens"; a disarmed one only says so. The
 * card is open or closed (a closed one is two lines and "Abrir a armadilha"). Presentational: the panel owns
 * the calls and the sheets.
 */
@Component({
  selector: 'app-trap-card',
  imports: [MatButtonModule, MatIconModule, TrapNoticers],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './trap-card.html',
  styleUrl: './trap-card.scss',
})
export class TrapCard {
  readonly point = input.required<MapPoint>();
  readonly open = input(false);
  /** "Quem notaria", once read. */
  readonly noticers = input<GetTrapNoticersResponse | undefined>(undefined);
  /** When it fired, if the activity knows. */
  readonly firedAt = input<Date | null>(null);
  /** The trap has a firing in the activity that creatures can still be added to. */
  readonly canExtend = input(false);
  readonly busy = input(false);
  /** "Quem notaria" could not be read. */
  readonly noticersFailed = input(false);
  readonly retryNoticers = output<void>();

  readonly toggle = output<void>();
  readonly reveal = output<void>();
  readonly fire = output<void>();
  readonly extend = output<void>();
  readonly disarm = output<void>();

  protected readonly state = computed(() => this.point().trap?.state ?? TrapState.ARMED);
  protected readonly armed = computed(() => this.state() === TrapState.ARMED);
  protected readonly fired = computed(() => this.state() === TrapState.TRIGGERED);
  protected readonly disarmed = computed(() => this.state() === TrapState.DISARMED);
  protected readonly stateWord = computed(() => trapStateWord(this.state(), this.firedAt()));
  protected readonly stateIcon = computed(() => trapStateIcon(this.state()));
  protected readonly visibility = computed(() => trapVisibility(this.point()));
  protected readonly sub = computed(() =>
    joinDots([firstLine(this.point().description), areaWords(this.point().trap?.areaSize ?? 1)].filter(Boolean)),
  );
  protected readonly dcLine = computed(() => trapDcLine(this.point()));
  protected readonly notice = computed(() => this.point().trap?.noticeDc ?? 0);
  protected readonly find = computed(() => this.point().trap?.findDc ?? 0);
  protected readonly trigger = computed(() => triggerWord(this.point().trap?.trigger ?? 0));
  protected readonly effect = computed(() => effectLine(this.point()) || 'Só descrição, sem efeito calculado');
  protected readonly everyoneSees = computed(() => this.visibility().kind === 'all');
  protected readonly closedNote = computed(() =>
    this.notice() === 0 && this.armed() ? 'Ninguém nota sozinho: só se acha com Investigação.' : '',
  );
}
