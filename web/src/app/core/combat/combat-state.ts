import { computed, signal } from '@angular/core';

import { type Encounter, EncounterStatus } from '../../../gen/meurpg/play/v1/combat_pb';

/** What `turn_changed` carries (play.proto). */
export interface TurnChange {
  readonly encounterId: string;
  readonly round: number;
  readonly currentCombatantId: string;
  readonly masterTurn: boolean;
}

/** What `combatant_moved` carries (play.proto). */
export interface CombatantMove {
  readonly encounterId: string;
  readonly combatantId: string;
  readonly col: number;
  readonly row: number;
}

/**
 * The open session's combat on this screen: the encounter as the caller may
 * see it, with the small updates the page makes after its own calls and
 * after the stream's events. Pure TypeScript with signals, so its rules
 * (a copy with an older revision never replaces a newer one; `turn_changed`
 * and `combatant_moved` apply in place; a combatant the screen doesn't know
 * means "read the combat again") are tested without a DOM.
 */
export class CombatState {
  /** `null` while the session has had no combat. */
  readonly encounter = signal<Encounter | null>(null);
  /** The player is on the "Mover" page (E6-10): the session page leaves its
   * header out, since the page has its own one-line one. */
  readonly moving = signal(false);
  /** The player opened the full-screen map ("Ver mapa"): same one-line header. */
  readonly mapOpen = signal(false);
  /** Either full-page view is open. */
  readonly fullPage = computed(() => this.moving() || this.mapOpen());
  /** The ended combat the person already left ("Voltar à sessão"). */
  private readonly dismissedId = signal<string | null>(null);

  /** The combat the page shows: running, or one that ended and wasn't left. */
  readonly shown = computed(() => {
    const e = this.encounter();
    return e && !(e.status === EncounterStatus.ENDED && e.id === this.dismissedId()) ? e : null;
  });

  /** A fresh copy (a read, or an answer to a call). Of two copies of the
   * same combat the larger revision wins; another combat replaces it. */
  apply(next: Encounter | null): void {
    const current = this.encounter();
    if (next && current && next.id === current.id && next.revision < current.revision) {
      return;
    }
    this.encounter.set(next);
  }

  clear(): void {
    this.encounter.set(null);
    this.dismissedId.set(null);
    this.moving.set(false);
    this.mapOpen.set(false);
  }

  /** "Voltar à sessão" on the summary of an ended combat. */
  dismissEnded(): void {
    const e = this.encounter();
    if (e) {
      this.dismissedId.set(e.id);
    }
  }

  /** `turn_changed`. `false` when it is about a combat the screen doesn't
   * have: read it again. */
  applyTurn(turn: TurnChange): boolean {
    const e = this.encounter();
    if (!e || e.id !== turn.encounterId) {
      return false;
    }
    this.encounter.set({
      ...e,
      round: turn.round,
      currentCombatantId: turn.currentCombatantId,
      masterTurn: turn.masterTurn,
    });
    return true;
  }

  /** `combatant_moved`. `false` when the combatant is not known (a missed
   * `encounter_changed`): read the combat again. */
  applyMove(move: CombatantMove): boolean {
    const e = this.encounter();
    if (!e || e.id !== move.encounterId) {
      return false;
    }
    if (!e.combatants.some((c) => c.id === move.combatantId)) {
      return false;
    }
    this.encounter.set({
      ...e,
      combatants: e.combatants.map((c) =>
        c.id === move.combatantId ? { ...c, placed: true, col: move.col, row: move.row } : c,
      ),
    });
    return true;
  }
}
