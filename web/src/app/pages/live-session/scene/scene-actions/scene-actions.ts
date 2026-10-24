import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  input,
  model,
  signal,
  untracked,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type {
  OpenSceneInfo,
  SceneActionView,
  SceneRoll,
} from '../../../../../gen/meurpg/play/v1/scene_pb';
import { actionSubtitle, actionTitle } from '../../../../core/maps/scene-actions';
import type { SceneState } from '../../../../core/play/scene-state';
import { initialOf, passText } from '../../../../core/play/scene-view';
import { SceneRollLine } from '../scene-roll-line/scene-roll-line';

/** One roll as a chip beside its action: who, the total, and the verdict when the action has a DC. */
interface Chip {
  readonly roll: SceneRoll;
  readonly who: string;
  readonly initial: string;
  readonly total: number;
  readonly verdict: 'passed' | 'failed' | null;
  readonly words: string;
  readonly fresh: boolean;
}

/** One action of the scene with the rolls made at it. */
interface Row {
  readonly id: string;
  readonly title: string;
  readonly check: string;
  readonly dc: number;
  readonly attempts: string;
  readonly chips: readonly Chip[];
  readonly freshCount: number;
}

/** "1 rolagem nova", "2 rolagens novas". */
function freshWords(n: number): string {
  return n === 1 ? '1 rolagem nova' : `${n} rolagens novas`;
}

/**
 * "Ações e rolagens" of the master's open scene (C3-01): one line per action ("Percepção · CD 14 · 1 tentativa") with
 * each player's roll as a chip beside it ("Brisa 16 ✓", "Toren 7 ✗"). A chip opens the roll's detail under the row
 * (the formula, the mode, "Dar mais uma tentativa"). A roll that arrives while the page is open is "nova" until its
 * chip is opened; the header counts them. "Teste em grupo" sits in the header and opens the group check's form under it.
 */
@Component({
  selector: 'app-scene-actions',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatButtonModule, MatIconModule, SceneRollLine],
  templateUrl: './scene-actions.html',
  styleUrl: './scene-actions.scss',
})
export class SceneActions {
  readonly campaignId = input.required<string>();
  readonly state = input.required<SceneState>();
  readonly scene = input.required<OpenSceneInfo>();
  /** "Teste em grupo" was pressed: the group check's form is open (the page owns it). */
  readonly groupAsk = model(false);

  /** The rolls whose chip waits to be opened, and the ones the master opened now. */
  private readonly fresh = signal<ReadonlySet<string>>(new Set());
  protected readonly open = signal<ReadonlySet<string>>(new Set());
  private known: ReadonlySet<string> | null = null;
  private knownFor = '';

  protected readonly dcNote = computed(() =>
    this.scene().showDc ? 'Os jogadores veem a CD' : 'Só você vê a CD',
  );
  protected readonly freshTotal = computed(() => this.fresh().size);
  protected readonly freshText = freshWords;

  protected readonly rows = computed<Row[]>(() => {
    const scene = this.scene();
    const fresh = this.fresh();
    const chipsOf = (id: string) =>
      scene.rolls
        .filter((r) => r.actionId === id)
        .slice()
        .reverse()
        .map((roll) => this.chip(roll, fresh));
    const rows = scene.actions.map((a) => this.row(a, chipsOf(a.id)));
    const known = new Set(scene.actions.map((a) => a.id));
    const lost = scene.rolls.filter((r) => !known.has(r.actionId));
    if (lost.length > 0) {
      const chips = lost
        .slice()
        .reverse()
        .map((roll) => this.chip(roll, fresh));
      rows.push({
        id: 'lost',
        title: 'Ação que saiu da cena',
        check: '',
        dc: 0,
        attempts: '',
        chips,
        freshCount: chips.filter((c) => c.fresh).length,
      });
    }
    return rows;
  });

  constructor() {
    // The first rolls the page sees are not new; the ones that come after are. A new scene starts over.
    effect(() => {
      const scene = this.scene();
      untracked(() => {
        const ids = new Set(scene.rolls.map((r) => r.id));
        if (this.known === null || this.knownFor !== scene.pointId) {
          this.known = ids;
          this.knownFor = scene.pointId;
          this.fresh.set(new Set());
          this.open.set(new Set());
          return;
        }
        const added = [...ids].filter((id) => !this.known?.has(id));
        this.known = ids;
        if (added.length > 0) {
          this.fresh.update((set) => new Set([...set, ...added]));
        }
      });
    });
  }

  private row(action: SceneActionView, chips: Chip[]): Row {
    return {
      id: action.id,
      title: actionTitle(action),
      check: actionSubtitle(action),
      dc: action.dc,
      attempts: this.attempts(action),
      chips,
      freshCount: chips.filter((c) => c.fresh).length,
    };
  }

  private attempts(action: SceneActionView): string {
    if (action.maxAttempts === 0) {
      return 'sem limite';
    }
    return `${action.maxAttempts} ${action.maxAttempts === 1 ? 'tentativa' : 'tentativas'}`;
  }

  private chip(roll: SceneRoll, fresh: ReadonlySet<string>): Chip {
    const who = roll.characterName.trim() || 'Personagem removido';
    const verdict = roll.passed === undefined ? null : roll.passed ? 'passed' : 'failed';
    return {
      roll,
      who,
      initial: initialOf(roll.characterName),
      total: roll.roll?.total ?? 0,
      verdict,
      words: verdict === null ? '' : passText(verdict === 'passed', 0),
      fresh: fresh.has(roll.id),
    };
  }

  protected isOpen(id: string): boolean {
    return this.open().has(id);
  }

  /** A chip opens the roll under the row; seeing it makes it no longer new. */
  protected toggle(chip: Chip): void {
    const id = chip.roll.id;
    this.open.update((set) => {
      const next = new Set(set);
      if (!next.delete(id)) {
        next.add(id);
      }
      return next;
    });
    if (this.fresh().has(id)) {
      this.fresh.update((set) => {
        const next = new Set(set);
        next.delete(id);
        return next;
      });
    }
  }

  protected openChips(row: Row): Chip[] {
    return row.chips.filter((c) => this.isOpen(c.roll.id));
  }
}
