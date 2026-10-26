import { computed, signal } from '@angular/core';

import type { PickMark, PickNote } from '../../../../shared/combat-map/combat-map';
import type { TargetRule } from '../../../../core/combat/cast-flow';
import {
  type PickRow,
  type PickWords,
  NOT_SEEN,
  counts,
  enough,
  pickWords,
  tapped,
} from '../../../../core/combat/target-pick';

/** The action that waits for its target. */
export interface PickSession {
  /** The action's key (the row says "Escolhendo o alvo"). */
  readonly key: string;
  /** "Raio de Fogo": the spell's or the attack's name. */
  readonly name: string;
  readonly rows: readonly PickRow[];
  readonly rule: TargetRule;
  /** The attack's numbers, said once a target is named. */
  readonly detail: string;
  /** An attack aims at an enemy; a heal or a blessing at a creature. */
  readonly foe: boolean;
  /** The action's own button, where the focus goes back to. */
  readonly origin: HTMLElement | null;
  /** The master acts for this character: a target it does not see reads "<name> não vê este alvo". */
  readonly by?: string;
  /** Opens the action's sheet: with the picks (the target named, the darts placed), or with none for the list. */
  readonly open: (picks: readonly string[] | null) => void;
}

/**
 * The player's choice of a target on the map (C7-04, C7-05), kept in memory: the action that waits (`session`), the creatures
 * the player tapped (`picks`, one entry per dart) and the chip of the last refusal. It decides nothing of the rules: the rows
 * are what the server offered, the taps follow the spell's own limits. "Confirmar" opens the action's sheet with the target
 * named, "Escolher na lista" the sheet as it always was.
 */
export class TargetMode {
  readonly session = signal<PickSession | null>(null);
  readonly picks = signal<readonly string[]>([]);
  readonly note = signal<PickNote | null>(null);

  /** `after` runs once the page has drawn what a change brought (the focus goes back to the action's button). */
  constructor(private readonly after: (run: () => void) => void) {}

  readonly active = computed(() => this.session() !== null);
  /** The creatures of the map, each with its marks, in the server's order. */
  readonly marks = computed<readonly PickMark[] | null>(() => {
    const s = this.session();
    if (!s) {
      return null;
    }
    const n = counts(this.picks());
    const mode = s.rule.kind === 'darts' ? 'darts' : 'target';
    return s.rows.map((r) => ({ ...r, count: n.get(r.id) ?? 0, mode }));
  });
  readonly ready = computed(() => {
    const s = this.session();
    return !!s && enough(s.rule, this.picks());
  });
  readonly words = computed<PickWords | null>(() => {
    const s = this.session();
    if (!s) {
      return null;
    }
    return pickWords({
      name: s.name,
      rule: s.rule,
      picks: this.picks(),
      labels: new Map(s.rows.map((r) => [r.id, r.label])),
      detail: s.detail,
      foe: s.foe,
    });
  });
  /** Several marks to place or take back: "Desfazer" is on the bar. */
  readonly several = computed(() => {
    const kind = this.session()?.rule.kind;
    return kind === 'darts' || kind === 'multi';
  });
  /** The spoken counter ("2 de 3 dardos"), for the polite live status. */
  readonly status = computed(() => {
    const s = this.session();
    if (!s) {
      return '';
    }
    if (s.rule.kind === 'darts') {
      return `${this.picks().length} de ${s.rule.max} ${s.rule.rays ? 'raios' : 'dardos'}`;
    }
    const first = this.picks()[0];
    const named = s.rows.find((r) => r.id === first)?.label;
    return named ? `${named} é o alvo` : '';
  });

  /** The player picked an action that asks for a target. */
  begin(session: PickSession): void {
    this.session.set(session);
    this.picks.set([]);
    this.note.set(null);
  }

  /** A tap on a creature of the map: a ringed one is chosen, any other says why it cannot be. */
  tap(id: string): void {
    const s = this.session();
    const row = s?.rows.find((r) => r.id === id);
    if (!s || !row) {
      return;
    }
    if (row.reason) {
      const text =
        row.reason === NOT_SEEN && s.by ? `${s.by} ${NOT_SEEN}` : `${row.label}: ${row.reason}`;
      this.note.set({ id, text });
      return;
    }
    this.note.set(null);
    this.picks.update((p) => tapped(s.rule, p, id));
  }

  /** "Desfazer": the last dart (or mark) goes. */
  undo(): void {
    this.note.set(null);
    this.picks.update((p) => p.slice(0, -1));
  }

  /** "Cancelar": back to the list of actions, the focus on the button that started it. */
  cancel(): void {
    const origin = this.session()?.origin;
    this.clear();
    this.after(() => origin?.focus());
  }

  /** "Confirmar": the action's sheet opens with the target named. */
  confirm(): void {
    const s = this.session();
    if (!s || !this.ready()) {
      return;
    }
    const picks = this.picks();
    this.clear();
    this.after(() => {
      s.origin?.focus();
      s.open(picks);
    });
  }

  /** "Escolher na lista": the sheet as it was, with its list of targets. */
  list(): void {
    const s = this.session();
    if (!s) {
      return;
    }
    this.clear();
    this.after(() => {
      s.origin?.focus();
      s.open(null);
    });
  }

  /** The mode ends without a word (the turn passed, or another action began). */
  clear(): void {
    this.session.set(null);
    this.picks.set([]);
    this.note.set(null);
  }
}
