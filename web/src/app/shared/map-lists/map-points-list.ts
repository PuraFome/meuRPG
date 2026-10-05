import { ChangeDetectionStrategy, Component, ElementRef, Injector, afterNextRender, computed, inject, input, output, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import { MapPointKind, type MapPoint } from '../../../gen/meurpg/maps/v1/maps_pb';
import { focusWithRing } from '../../core/creatures/focus-ring';
import { pointSubLine, pointTags } from '../../core/maps/point-text';
import { summaryLine } from '../../core/traps/treasure-text';
import { ChestIcon } from '../chest-icon/chest-icon';
import { PairFoot } from '../pair-foot/pair-foot';
import { type PickRow, PersonPick } from '../person-pick/person-pick';
import { pointHidden, pointKindIcon, pointKindLabel } from '../map-view/map-labels';

export interface PointToggle {
  readonly point: MapPoint;
  /** The state the master asked for. */
  readonly revealed: boolean;
}

/** A treasure marked found from the list: who found it. */
export interface TreasureMark {
  readonly point: MapPoint;
  readonly characterIds: readonly string[];
}

/** "Submapa: Torre de Mirathel", or just "Batalha". */
export function pointSub(point: MapPoint): string {
  const kind = pointKindLabel(point.kind);
  return point.targetMap ? `${kind}: ${point.targetMap.name}` : kind;
}

/**
 * "Pontos do mapa" (E5-04, E5-06, E5-24): every point of the map with its
 * state in words ("Revelado" with an eye, "Escondido" with an eye-off) and
 * an outlined "Revelar aos jogadores" or "Esconder", the non-pointer way to
 * what the map shows (MR-009). The master only: a player never gets the
 * hidden rows. The button's accessible name carries the point's name.
 *
 * In the live session (`openScenePointId` set, `null` when no scene is open) a
 * SCENE point also says what it does with the open scene (MR-015, question 53):
 * "Abrir cena", "Trocar para esta cena" when another one is open, or "Cena
 * aberta agora" when it is this one. A scene with no actions says so and cannot
 * open.
 */
@Component({
  selector: 'app-map-points-list',
  imports: [ChestIcon, MatButtonModule, MatIconModule, PairFoot, PersonPick],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './map-points-list.html',
  styleUrl: './map-lists.scss',
})
export class MapPointsList {
  readonly points = input.required<readonly MapPoint[]>();
  /** The point whose call is in flight: its button waits. */
  readonly pendingId = input<string | null>(null);
  /** The heading's level: 3 inside the map's panel, 2 as a panel of its own. */
  readonly headingLevel = input<2 | 3>(3);
  /** The session's open scene point: `null` for none; left out (`undefined`)
   * where there is no session, and then no row offers "Abrir cena". */
  readonly openScenePointId = input<string | null | undefined>(undefined);
  /** The scene point whose "Abrir cena" call is in flight. */
  readonly pendingSceneId = input<string | null>(null);
  /** The preset names by key, for "Luz · Tocha · 6 m claro + 6 m de penumbra". */
  readonly lightNames = input<ReadonlyMap<string, string>>(new Map());
  /** The living player characters, when the screen can mark a treasure found ("Quem encontrou"): a treasure then offers "Marcar como encontrado" in place of "Revelar aos jogadores". `null`: it cannot (the session's list has the treasure card). */
  readonly people = input<readonly PickRow[] | null>(null);
  readonly markBusy = input(false);
  /** Why marking failed, in words. */
  readonly markError = input('');
  readonly toggle = output<PointToggle>();
  /** The pick was confirmed: the screen makes the call. */
  readonly markFound = output<TreasureMark>();
  /** "Abrir cena" / "Trocar para esta cena" on a scene point. */
  readonly openScene = output<MapPoint>();

  protected readonly Scene = MapPointKind.SCENE;
  protected readonly Treasure = MapPointKind.TREASURE;
  protected readonly Light = MapPointKind.LIGHT;
  private readonly injector = inject(Injector);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  /** The treasure whose "Quem encontrou" is open, and who is picked. */
  protected readonly marking = signal<string | null>(null);
  protected readonly picked = signal<ReadonlySet<string>>(new Set());
  protected readonly summary = computed(() => {
    const id = this.marking();
    const point = this.points().find((p) => p.id === id);
    const names = (this.people() ?? []).filter((p) => this.picked().has(p.id)).map((p) => p.name);
    return point ? summaryLine(names, point.treasureValuePo) : '';
  });

  protected rows = computed(() =>
    this.points().map((point) => ({
      point,
      hidden: pointHidden(point),
      sub: pointSubLine(point, this.lightNames().get(point.light?.presetKey ?? '') ?? ''),
      // A trap and a treasure keep their state in words (Armada, Não encontrado) next to who sees them; a Luz is the master's alone.
      tags: point.kind > MapPointKind.SCENE ? pointTags(point) : [pointHidden(point) ? { icon: 'visibility_off', text: 'Escondido' } : { icon: 'visibility', text: 'Revelado' }],
      canMark: point.kind === MapPointKind.TREASURE && this.people() !== null && !point.treasureFoundAt,
    })),
  );

  protected startMarking(id: string): void {
    this.picked.set(new Set());
    this.marking.set(id);
    afterNextRender(
      () => {
        const form = this.host.nativeElement.querySelector<HTMLElement>('.row__mark');
        form?.scrollIntoView?.({ block: 'nearest' });
        focusWithRing(form?.querySelector<HTMLInputElement>('input[type=checkbox]:not(:disabled)'));
      },
      { injector: this.injector },
    );
  }

  protected cancelMarking(id: string): void {
    this.marking.set(null);
    afterNextRender(() => focusWithRing(this.host.nativeElement.querySelector<HTMLElement>(`[data-mark="${id}"]`)), { injector: this.injector });
  }

  protected confirmMarking(point: MapPoint): void {
    if (this.picked().size === 0 || this.markBusy()) {
      return;
    }
    this.markFound.emit({ point, characterIds: [...this.picked()] });
  }

  /** The screen says the mark worked (the point now carries the finders): the pick closes. */
  closeMarking(): void {
    this.marking.set(null);
  }

  protected readonly icon = pointKindIcon;
  protected readonly sub = pointSub;
}
