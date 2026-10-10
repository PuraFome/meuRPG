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

import type { Encounter } from '../../../../../gen/meurpg/play/v1/combat_pb';
import type { SurpriseSuggestion } from '../../../../../gen/meurpg/play/v1/contest_types_pb';
import { combatErrorMessage } from '../../../../core/combat/combat-errors';
import type { CombatState } from '../../../../core/combat/combat-state';
import { ContestClient } from '../../../../core/combat/contest-client';
import { surpriseRows } from '../../../../core/combat/contest-master';
import { ActionKey } from '../../../../core/connect/idempotency';

let nextId = 0;

/**
 * "Quem está surpreso?" (W7-X, board W7-Xc 11, the master's side; SRD 5.1, Surprise): in the combat's setup, before "Começar o combate",
 * the app compares the Stealth of each creature that hides with the passive Perception of every creature (`GetSurpriseSuggestion`:
 * "Percepção passiva 9 · Furtividade de Brisa 19 vence") and the master decides: "Surpreso" on any creature, player characters
 * included, one by one (`SetSurprised`; a surprised creature does not move, act, take a bonus action or react until its first turn
 * ends: the bonus action is the app's reading). "Marcar os sugeridos" marks the app's suggestion in one go; nothing is marked until the
 * master says so. The player reads only their own state. It reads the suggestion again after each mark.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-surprise-card',
  imports: [MatButtonModule, MatIconModule],
  styleUrl: '../contest-master/master-card.scss',
  template: `
    <section class="card" [attr.aria-labelledby]="uid + 't'" data-testid="surprise-card">
      <h2 class="card__title" [id]="uid + 't'">Quem está surpreso?</h2>
      <p class="card__sub">
        Sugestão: a Furtividade de cada um que se esconde contra a Percepção passiva de cada criatura. Você decide.
      </p>
      <ul class="rows">
        @for (r of rows(); track r.id) {
          <li class="row">
            <span class="row__main">
              <span class="row__name">{{ r.label }}</span>
              <span class="row__sub">{{ r.detail }}</span>
            </span>
            @if (r.suggested) {
              <span class="tag">Sugerido</span>
            }
            <label class="choice" [class.choice--on]="marked(r.id)">
              <input
                type="checkbox"
                class="mr-visually-hidden"
                [checked]="marked(r.id)"
                [disabled]="busy()"
                [attr.aria-label]="'Surpreso: ' + r.label"
                (change)="mark(r.id, !marked(r.id))"
              />
              <span class="choice__name">Surpreso</span>
            </label>
          </li>
        }
      </ul>
      @if (error()) {
        <div class="mr-notice mr-notice--danger" role="alert">
          <mat-icon aria-hidden="true">error</mat-icon>
          <p>{{ error() }}</p>
        </div>
      }
      @if (toSuggest().length > 0) {
        <div class="btns">
          <button mat-stroked-button type="button" class="btn" [disabled]="busy()" (click)="markSuggested()">
            Marcar os sugeridos
          </button>
        </div>
      }
      <p class="small">
        Quem está surpreso não se move, não age e não reage até o fim do primeiro turno. A ação bônus também fica indisponível
        (leitura do app).
      </p>
      <span class="mr-visually-hidden" role="status" aria-live="polite">{{ said() }}</span>
    </section>
  `,
})
export class SurpriseCard {
  private readonly api = inject(ContestClient);
  private readonly keys = new ActionKey();

  readonly encounter = input.required<Encounter>();
  readonly campaignId = input.required<string>();
  readonly state = input.required<CombatState>();

  protected readonly uid = `sc-${nextId++}-`;
  protected readonly busy = signal(false);
  protected readonly error = signal('');
  protected readonly said = signal('');
  protected readonly suggestions = signal<readonly SurpriseSuggestion[]>([]);

  protected readonly rows = computed(() => surpriseRows(this.encounter(), this.suggestions()));
  protected readonly toSuggest = computed(() =>
    this.suggestions().filter((s) => s.suggested && !s.surprised),
  );

  constructor() {
    effect(() => {
      const e = this.encounter();
      const campaignId = this.campaignId();
      void e.revision;
      untracked(() => void this.reload(campaignId, e.id));
    });
  }

  protected marked(id: string): boolean {
    return this.suggestions().find((s) => s.combatantId === id)?.surprised ?? false;
  }

  private async reload(campaignId: string, encounterId: string): Promise<void> {
    try {
      this.suggestions.set(await this.api.surpriseSuggestion(campaignId, encounterId));
    } catch {
      // Best effort: the next change of the combat asks again.
    }
  }

  protected async mark(id: string, surprised: boolean): Promise<void> {
    await this.run('marcar a surpresa', async () => {
      await this.set(id, surprised);
      this.said.set(
        `${this.rows().find((r) => r.id === id)?.label ?? ''}: ${surprised ? 'surpreso' : 'não surpreso'}.`,
      );
    });
  }

  protected async markSuggested(): Promise<void> {
    await this.run('marcar a surpresa', async () => {
      for (const s of this.toSuggest()) {
        await this.set(s.combatantId, true);
      }
      this.said.set('Sugestão marcada.');
    });
  }

  private async set(id: string, surprised: boolean): Promise<void> {
    const encounterId = this.encounter().id;
    const res = await this.api.setSurprised(
      this.campaignId(),
      encounterId,
      id,
      surprised,
      this.keys.keyFor({ id, surprised }),
    );
    this.state().apply(res.encounter);
    this.keys.renew();
    await this.reload(this.campaignId(), encounterId);
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
    } finally {
      this.busy.set(false);
    }
  }
}
