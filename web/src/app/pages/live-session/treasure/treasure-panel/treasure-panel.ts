import { Component, computed, inject, input, signal } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { MapPoint } from '../../../../../gen/meurpg/maps/v1/maps_pb';
import { MapsClient } from '../../../../core/maps/maps-client';
import type { MapState } from '../../../../core/maps/map-state';
import { trapMapErrorMessage } from '../../../../core/traps/trap-errors';
import { finders, treasureSub } from '../../../../core/traps/treasure-text';
import { treasurePoints } from '../../../../core/traps/trap-text';
import { listNames } from '../../../../core/maps/scene-clues';
import type { PickRow } from '../../../../shared/person-pick/person-pick';
import { type FoundChoice, TreasureCard } from '../treasure-card/treasure-card';

/**
 * "Tesouros do mapa" on the master's session page (E9-09 1 to 3, MR-041, RN-10): the treasures of the current
 * map as cards, hidden, found or converted into XP. Marking one found names who found it (`MarkTreasureFound`,
 * one or more characters) and the point comes back as the server has it; "Desmarcar" takes it back until it is
 * turned into XP (`UnmarkTreasureFound`). The players see a treasure only once it is found; the master's
 * description and the value stay with the master until then. The panel exists only while the map has a
 * treasure.
 */
@Component({
  selector: 'app-treasure-panel',
  imports: [MatIconModule, TreasureCard],
  template: `
    @if (treasures().length > 0) {
      <section class="mr-panel tp" aria-labelledby="treasure-h">
        <h2 class="mr-panel__title" id="treasure-h">Tesouros do mapa</h2>
        <p class="tp__lead">Os jogadores só veem um tesouro depois que você marca que ele foi encontrado.</p>
        @for (p of treasures(); track p.id) {
          <app-treasure-card
            [point]="p"
            [people]="people()"
            [busy]="busy() === p.id"
            [error]="errors().get(p.id) ?? ''"
            (mark)="mark($event)"
            (unmark)="unmark($event)"
          />
        }
        <p class="mr-visually-hidden" role="status" aria-live="polite">{{ announcement() }}</p>
      </section>
    }
  `,
  styles: `
    :host {
      display: contents;
    }

    .tp {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-3);
    }

    .tp .mr-panel__title {
      margin: 0;
    }

    .tp__lead {
      margin: 0;
      font-size: 14px;
      line-height: 19px;
      color: var(--mr-ink-muted);
    }
  `,
})
export class TreasurePanel {
  private readonly api = inject(MapsClient);

  readonly campaignId = input.required<string>();
  readonly state = input.required<MapState>();
  /** The living player characters, for "Quem encontrou". */
  readonly people = input<readonly PickRow[]>([]);

  protected readonly treasures = computed(() => treasurePoints(this.state().points()));
  protected readonly busy = signal<string | null>(null);
  protected readonly errors = signal<ReadonlyMap<string, string>>(new Map());
  protected readonly announcement = signal('');

  private fail(id: string, message: string): void {
    this.errors.update((m) => new Map(m).set(id, message));
  }

  protected async mark(choice: FoundChoice): Promise<void> {
    await this.run(choice.point, 'marcar o tesouro', async () => {
      const point = await this.api.markTreasureFound(this.campaignId(), this.state().map()?.id ?? '', choice.point.id, choice.characterIds);
      this.state().upsertPoint(point);
      const names = finders(point);
      this.announcement.set(`${point.name} marcado como encontrado${names.length > 0 ? ` por ${listNames(names)}` : ''}. ${treasureSub(point)}.`);
    });
  }

  protected async unmark(point: MapPoint): Promise<void> {
    await this.run(point, 'desmarcar o tesouro', async () => {
      const back = await this.api.unmarkTreasureFound(this.campaignId(), this.state().map()?.id ?? '', point.id);
      this.state().upsertPoint(back);
      this.announcement.set(`${back.name} voltou a ficar escondido.`);
    });
  }

  private async run(point: MapPoint, what: string, work: () => Promise<void>): Promise<void> {
    if (this.busy() !== null) {
      return;
    }
    this.busy.set(point.id);
    this.errors.update((m) => new Map([...m].filter(([k]) => k !== point.id)));
    try {
      await work();
    } catch (err) {
      this.fail(point.id, trapMapErrorMessage(err, what));
    } finally {
      this.busy.set(null);
    }
  }
}
