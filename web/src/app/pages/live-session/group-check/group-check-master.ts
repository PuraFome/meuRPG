import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  input,
  signal,
  untracked,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { DiceMode, DicePreference } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import type { GroupCheckMemberView } from '../../../../gen/meurpg/play/v1/contest_types_pb';
import { combatErrorMessage } from '../../../core/combat/combat-errors';
import { type CheckDie, ContestClient } from '../../../core/combat/contest-client';
import {
  groupProgress,
  groupTitle,
  groupVerdict,
  memberBonus,
  memberResult,
} from '../../../core/combat/contest-master';
import { GroupCheckState } from '../../../core/combat/group-check-state';
import { HELP_TASKS } from '../../../core/combat/help-view';
import { ActionKey } from '../../../core/connect/idempotency';
import { CheckRollForm } from '../combat/check-roll-form/check-roll-form';

let nextId = 0;

/** The lowest and the highest DC the server takes. */
const DC_MIN = 1;
const DC_MAX = 40;

/** The DC typed, or `0` (none) when it is empty, or `null` when it is not a whole number the server takes. */
function dcOf(text: string): number | null {
  const t = text.trim();
  if (t === '') {
    return 0;
  }
  const n = Number(t);
  return Number.isInteger(n) && n >= DC_MIN && n <= DC_MAX ? n : null;
}

/**
 * "Teste em grupo" on the master's page (W7-X, board W7-Xc 10, the master's side; SRD 5.1, Group Checks). Asking: the skill, the DC
 * (empty for none) and "Mostrar a CD aos jogadores" (`show_dc`: the players read passou or falhou, never the DC itself). Waiting: who
 * answered ("4 de 5 responderam"), each character's bonus, total and "Passou" or "Falhou", "Não respondeu" for the one who has not,
 * "Rolar por Ragna" for that one (the master's roll, app or typed) and "Encerrar o teste" (whoever did not answer counts as failed).
 * The verdict ("2 de 5 passaram; precisa de 3. O grupo falhou.") is only his, until the DC is shown. It reads the check again on each
 * `group_check_changed` (the page bumps `tick`). Every write carries its own key, made once.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-group-check-master',
  imports: [CheckRollForm, MatButtonModule, MatIconModule],
  styleUrl: '../combat/contest-master/master-card.scss',
  template: `
    @if (open(); as g) {
      <section class="card" [attr.aria-labelledby]="uid + 't'" data-testid="group-open">
        <h2 class="card__title" [id]="uid + 't'">{{ title() }}</h2>
        <p class="card__sub">{{ progress() }}</p>
        <ul class="rows">
          @for (m of g.members; track m.characterId) {
            <li class="row">
              <span class="row__main">
                <span class="row__name">{{ m.name }}</span>
                <span class="row__sub">{{ bonus(m) }}</span>
              </span>
              <span class="tag" [class.tag--ok]="m.passedKnown && m.passed">{{ result(m) }}</span>
              @if (!m.answered) {
                <button
                  mat-stroked-button
                  type="button"
                  class="btn btn--small"
                  [disabled]="busy()"
                  (click)="rolling.set(rolling() === m.characterId ? '' : m.characterId)"
                >
                  Rolar por {{ m.name }}
                </button>
              }
              @if (rolling() === m.characterId) {
                <div class="row__form">
                  <app-check-roll-form
                    [checkName]="g.skillNamePt + ' de ' + m.name"
                    [appLabel]="'Rolar por ' + m.name"
                    totalNote="Só o d20; o modificador entra depois"
                    [diceMode]="diceMode()"
                    [preference]="preference()"
                    [busy]="busy()"
                    (roll)="rollFor(m, $event)"
                  />
                </div>
              }
            </li>
          }
        </ul>
        @if (verdict()) {
          <p class="card__text">
            <b>Veredito do grupo (só o mestre):</b> {{ verdict() }}
            @if (!g.showDc) {
              Os jogadores veem passou ou falhou só se você mostrar a CD.
            }
          </p>
        }
        @if (error()) {
          <div class="mr-notice mr-notice--danger" role="alert">
            <mat-icon aria-hidden="true">error</mat-icon>
            <p>{{ error() }}</p>
          </div>
        }
        <div class="btns">
          <button mat-flat-button type="button" class="btn" [disabled]="busy()" (click)="close(g.id)">
            Encerrar o teste
          </button>
        </div>
      </section>
    } @else {
      @if (last(); as g) {
        <section class="card" [attr.aria-labelledby]="uid + 'l'" data-testid="group-last">
          <h2 class="card__title" [id]="uid + 'l'">{{ lastTitle() }}</h2>
          <p class="card__text">{{ lastVerdict() }}</p>
          <div class="btns">
            <button mat-stroked-button type="button" class="btn" (click)="dismissed.set(g.id)">Dispensar</button>
          </div>
        </section>
      }
      <section class="card" [attr.aria-labelledby]="uid + 'a'" data-testid="group-ask">
        <h2 class="card__title" [id]="uid + 'a'">Teste em grupo</h2>
        <p class="card__sub">
          Todos os personagens rolam o mesmo teste; o grupo passa se ao menos metade passar.
        </p>
        <div class="field">
          <label [for]="uid + 's'">Teste</label>
          <select [id]="uid + 's'" (change)="pickSkill($event)">
            @for (t of tasks; track t.key) {
              <option [value]="t.key" [selected]="t.key === skillKey()">{{ t.name }}</option>
            }
          </select>
        </div>
        <div class="field">
          <label [for]="uid + 'd'">CD ({{ dcMin }} a {{ dcMax }}; vazio: sem CD)</label>
          <input
            [id]="uid + 'd'"
            type="text"
            inputmode="numeric"
            autocomplete="off"
            [value]="dcText()"
            (input)="typeDc($event)"
          />
        </div>
        <label class="choice" [class.choice--on]="showDc()">
          <input
            type="checkbox"
            class="mr-visually-hidden"
            [checked]="showDc()"
            (change)="showDc.set(!showDc())"
          />
          <span class="choice__text">
            <span class="choice__name">Mostrar a CD aos jogadores</span>
            <span class="choice__sub">Eles leem passou ou falhou; a CD continua só sua.</span>
          </span>
        </label>
        @if (error()) {
          <div class="mr-notice mr-notice--danger" role="alert">
            <mat-icon aria-hidden="true">error</mat-icon>
            <p>{{ error() }}</p>
          </div>
        }
        <div class="btns">
          <button
            mat-flat-button
            type="button"
            class="btn"
            [disabled]="busy() || dc() === null"
            (click)="request()"
          >
            Pedir o teste
          </button>
        </div>
      </section>
    }
    <span class="mr-visually-hidden" role="status" aria-live="polite">{{ said() }}</span>
  `,
  styles: `
    .row__form {
      flex: 1 1 100%;
    }
  `,
})
export class GroupCheckMaster {
  private readonly api = inject(ContestClient);
  private readonly keys = new ActionKey();

  readonly campaignId = input.required<string>();
  /** Goes up on every `group_check_changed` and every (re)connection of the stream. */
  readonly tick = input(0);
  readonly diceMode = input.required<DiceMode>();
  readonly preference = input.required<DicePreference>();

  protected readonly uid = `gm-${nextId++}-`;
  protected readonly tasks = HELP_TASKS;
  protected readonly dcMin = DC_MIN;
  protected readonly dcMax = DC_MAX;
  protected readonly checks = new GroupCheckState();
  protected readonly busy = signal(false);
  protected readonly error = signal('');
  protected readonly said = signal('');
  protected readonly skillKey = signal(HELP_TASKS[0].key);
  protected readonly dcText = signal('');
  protected readonly showDc = signal(false);
  /** The character the master is rolling for. */
  protected readonly rolling = signal('');
  /** The closed check the master put away. */
  protected readonly dismissed = signal('');

  protected readonly open = computed(() => {
    const g = this.checks.view();
    return g?.open ? g : null;
  });
  protected readonly last = computed(() => {
    const g = this.checks.view();
    return g && !g.open && g.id !== this.dismissed() ? g : null;
  });
  protected readonly title = computed(() => {
    const g = this.open();
    return g ? groupTitle(g) : '';
  });
  protected readonly progress = computed(() => {
    const g = this.open();
    return g ? groupProgress(g) : '';
  });
  protected readonly verdict = computed(() => {
    const g = this.open();
    return g ? groupVerdict(g) : '';
  });
  protected readonly lastTitle = computed(() => {
    const g = this.last();
    return g ? groupTitle(g) : '';
  });
  protected readonly lastVerdict = computed(() => {
    const g = this.last();
    return g ? groupVerdict(g) || 'Encerrado.' : '';
  });
  protected readonly dc = computed(() => dcOf(this.dcText()));
  protected readonly result = memberResult;

  constructor() {
    effect(() => {
      const campaignId = this.campaignId();
      this.tick();
      untracked(() => void this.checks.load(this.api, campaignId));
    });
  }

  protected bonus(m: GroupCheckMemberView): string {
    const g = this.checks.view();
    return g ? memberBonus(g, m) : '';
  }

  protected pickSkill(event: Event): void {
    this.skillKey.set((event.target as HTMLSelectElement).value);
  }

  protected typeDc(event: Event): void {
    this.dcText.set((event.target as HTMLInputElement).value);
    this.error.set('');
  }

  protected async request(): Promise<void> {
    const dc = this.dc();
    if (dc === null) {
      return;
    }
    const request = { skillKey: this.skillKey(), dc, showDc: dc > 0 && this.showDc() };
    await this.run('pedir o teste em grupo', async () => {
      const view = await this.api.requestGroupCheck(
        this.campaignId(),
        request,
        this.keys.keyFor(request),
      );
      this.checks.apply(view);
      this.keys.renew();
      this.said.set(`Teste em grupo pedido: ${view.skillNamePt}.`);
    });
  }

  protected async rollFor(m: GroupCheckMemberView, die: CheckDie): Promise<void> {
    const g = this.open();
    if (!g) {
      return;
    }
    await this.run(`rolar por ${m.name}`, async () => {
      const view = await this.api.rollForPlayer(
        this.campaignId(),
        g.id,
        m.characterId,
        die,
        this.keys.keyFor({ g: g.id, c: m.characterId, die }),
      );
      this.checks.apply(view);
      this.rolling.set('');
      this.said.set(`Rolado por ${m.name}.`);
    });
  }

  protected async close(id: string): Promise<void> {
    await this.run('encerrar o teste', async () => {
      const view = await this.api.closeGroupCheck(
        this.campaignId(),
        id,
        this.keys.keyFor({ close: id }),
      );
      this.checks.apply(view);
      this.said.set('Teste em grupo encerrado.');
    });
  }

  private async run(what: string, call: () => Promise<void>): Promise<void> {
    if (this.busy()) {
      return;
    }
    this.busy.set(true);
    this.error.set('');
    try {
      await call();
    } catch (err) {
      this.error.set(combatErrorMessage(err, what));
      void this.checks.load(this.api, this.campaignId());
    } finally {
      this.busy.set(false);
    }
  }
}
