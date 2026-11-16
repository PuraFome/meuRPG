import { computed, signal } from '@angular/core';

import { type NpcCheckCandidate, NpcCheckSource } from '../../../gen/meurpg/play/v1/scene_pb';
import type { NpcCheckRoll } from './npc-check-client';
import {
  D20_RANGE_TEXT,
  WHO_PICK_TEXT,
  boxHint,
  boxStart,
  checkHint,
  checkOptions,
  defaultCheck,
  groupsOf,
  noBonusLine,
  parseD20,
  rollLabel,
  rowId,
  showsBox,
} from './npc-check-picker';

/** How the master's d20 comes. */
export type DieMode = 'app' | 'type';

/**
 * The form of "Teste de um NPC" without the screen: who rolls, which check, how the d20 comes and the box "Os bardos veem a
 * criatura e ela os ouve". Choosing a creature picks its check (Deception when it has it) and the box's start (unticked for a
 * hidden creature); what the master did with the box stays until another creature is chosen.
 */
export class NpcCheckForm {
  private readonly list = signal<readonly NpcCheckCandidate[]>([]);
  private readonly boxChoice = signal<boolean | null>(null);
  readonly selectedId = signal('');
  readonly checkKey = signal('');
  readonly mode = signal<DieMode>('app');
  readonly text = signal('');

  readonly groups = computed(() => groupsOf(this.list()));
  readonly selected = computed(() => this.list().find((c) => rowId(c) === this.selectedId()) ?? null);
  readonly options = computed(() => {
    const c = this.selected();
    return c ? checkOptions(c) : [];
  });
  readonly noBonus = computed(() => {
    const c = this.selected();
    return c ? noBonusLine(c, this.checkKey()) : '';
  });
  readonly showsBox = computed(() => {
    const c = this.selected();
    return c !== null && showsBox(c);
  });
  readonly box = computed(() => {
    const c = this.selected();
    return this.boxChoice() ?? (c ? boxStart(c) : false);
  });
  readonly checkHint = computed(() => {
    const c = this.selected();
    return c ? checkHint(c) : '';
  });
  /** The line under the box, or the quiet line that stands in its place when the map measures. */
  readonly boxHint = computed(() => {
    const c = this.selected();
    return c ? boxHint(c) : '';
  });
  /** The d20 the master typed; `null` while the dice are the app's or the text is not a d20. */
  readonly face = computed(() => (this.mode() === 'type' ? parseD20(this.text()) : null));
  /** The typed d20 is not a number from 1 to 20 (an empty field is not wrong yet). */
  readonly invalid = computed(() => this.mode() === 'type' && this.text().trim() !== '' && this.face() === null);
  readonly ready = computed(
    () => this.selected() !== null && this.checkKey() !== '' && (this.mode() === 'app' || this.face() !== null),
  );
  /** Why the roll button is dashed, above it. */
  readonly reason = computed(() => {
    if (this.selected() === null || this.checkKey() === '') {
      return WHO_PICK_TEXT;
    }
    return this.mode() === 'type' && this.face() === null ? D20_RANGE_TEXT : '';
  });
  readonly rollLabel = computed(() => {
    const c = this.selected();
    return c ? rollLabel(c, this.checkKey()) : 'Rolar o teste';
  });

  /** The creatures the server listed (the stage and the map); the choice stays if its creature is still there. */
  setCandidates(list: readonly NpcCheckCandidate[]): void {
    const found = this.list().find((c) => c.source === NpcCheckSource.MONSTER);
    this.list.set(found && !list.some((c) => rowId(c) === rowId(found)) ? [...list, found] : list);
    if (this.selected() === null) {
      this.selectedId.set('');
      this.checkKey.set('');
    }
  }

  select(id: string): void {
    const c = this.list().find((o) => rowId(o) === id);
    if (!c) {
      return;
    }
    this.selectedId.set(id);
    this.checkKey.set(defaultCheck(c));
    this.boxChoice.set(null);
  }

  /** The creature the bestiary found: the group keeps one, and it is the one chosen. */
  addFound(c: NpcCheckCandidate): void {
    this.list.update((all) => [...all.filter((o) => o.source !== NpcCheckSource.MONSTER), c]);
    this.select(rowId(c));
  }

  setCheck(key: string): void {
    this.checkKey.set(key);
  }
  setMode(mode: DieMode): void {
    this.mode.set(mode);
  }
  setText(text: string): void {
    this.text.set(text);
  }
  setBox(on: boolean): void {
    this.boxChoice.set(on);
  }

  /** What to send, `null` while the form is not ready. */
  roll(): NpcCheckRoll | null {
    const c = this.selected();
    if (!c || !this.ready()) {
      return null;
    }
    const face = this.face();
    return {
      characterId: c.characterId,
      monsterKey: c.monsterKey,
      checkKey: this.checkKey(),
      die: face === null ? { inApp: true } : { face },
      bardSees: showsBox(c) ? this.box() : false,
    };
  }
}
