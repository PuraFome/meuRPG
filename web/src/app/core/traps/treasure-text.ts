import { type MapPoint, MapPointKind } from '../../../gen/meurpg/maps/v1/maps_pb';
import { joinDots, tight } from '../format/text';
import { formatClock } from '../../shared/session-time/session-time';
import { listNames } from '../maps/scene-clues';
import { clockOf } from './trap-text';

/** "250 PO", the number and its unit tied with a no-break space; pt-BR thousands separator. */
export function poText(value: number): string {
  return `${String(Math.trunc(value)).replace(/\B(?=(\d{3})+(?!\d))/g, '.')} PO`;
}

/** "Tesouro · valor 250 PO". */
export function treasureSub(point: MapPoint): string {
  return tight(joinDots(['Tesouro', `valor ${poText(point.treasureValuePo)}`]));
}

/** "Brisa", "Brisa e Toren": who found it, by name. */
export function finders(point: MapPoint): string[] {
  return point.treasureFoundBy.map((f) => f.characterName).filter(Boolean);
}

/** "Encontrado por Brisa às 21:40". */
export function foundLine(point: MapPoint): { names: string; at: string } {
  const at = clockOf(point.treasureFoundAt);
  return { names: listNames(finders(point)), at: at ? formatClock(at) : '' };
}

/** The line under the picks while marking: who found it and the value; the server splits it among them, the browser never divides. */
export function summaryLine(names: readonly string[], value: number): string {
  if (names.length === 0) {
    return 'Ninguém marcado. Escolha quem encontrou.';
  }
  return `No resumo da sessão: ${listNames(names)} · ${poText(value)}`;
}

/** "Brisa encontrou: Baú de moedas": no article before the name, which is the master's free text (a guessed "o" or "a" is wrong half the time). */
export function foundToastTitle(point: MapPoint): string {
  const names = finders(point);
  const who = names.length > 0 ? listNames(names) : 'O mestre';
  const verb =
    names.length > 1 ? 'encontraram' : names.length === 1 ? 'encontrou' : 'marcou como encontrado';
  return `${who} ${verb}: ${point.name}`;
}

/**
 * Which treasures were found since the page last looked at the map (E9-09 4): the first look at a map is
 * the baseline (a treasure found before the player got here is not news), and every later read that holds a
 * found treasure it did not hold before is. Pure state, so it is tested without a DOM.
 */
export class TreasureWatch {
  private mapId: string | null = null;
  private seen = new Set<string>();

  newlyFound(mapId: string | null, points: readonly MapPoint[]): readonly MapPoint[] {
    const found = points.filter(
      (p) => p.kind === MapPointKind.TREASURE && p.treasureFoundAt !== undefined,
    );
    if (mapId === null) {
      this.mapId = null;
      this.seen = new Set();
      return [];
    }
    if (mapId !== this.mapId) {
      this.mapId = mapId;
      this.seen = new Set(found.map((p) => p.id));
      return [];
    }
    const fresh = found.filter((p) => !this.seen.has(p.id));
    this.seen = new Set(found.map((p) => p.id));
    return fresh;
  }
}
