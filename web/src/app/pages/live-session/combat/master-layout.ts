import { type Signal, computed, signal } from '@angular/core';

import type { TabItem, TabTone } from '../../../shared/tab-list/tab-list';
import type { Encounter } from '../../../../gen/meurpg/play/v1/combat_pb';
import { roundLabel, turnBanner } from '../../../core/combat/combat-view';
import type { MapTab, MapTabId } from './map-tabs/map-tab';

/** The most a tab's count shows before it says "99+". */
const MAX_BADGE = 99;

/** One row of "Mais do combate". */
export interface MorePanel {
  readonly id: 'log' | 'effects' | MapTabId;
  readonly label: string;
  readonly icon: string;
  readonly count?: number;
}

/** What a row of "Mais do combate" says: its words and icon (the order the board draws them in). */
const PANELS: Readonly<Record<'log' | 'effects' | MapTabId, { label: string; icon: string }>> = {
  log: { label: 'Registro', icon: 'history' },
  effects: { label: 'Efeitos', icon: 'hourglass_empty' },
  fog: { label: 'Névoa e luz', icon: 'lightbulb' },
  traps: { label: 'Armadilhas', icon: 'warning' },
  treasure: { label: 'Tesouro', icon: 'star_outline' },
  puzzles: { label: 'Quebra-cabeças', icon: 'extension' },
};
const PANEL_ORDER = ['log', 'effects', 'fog', 'traps', 'treasure', 'puzzles'] as const;

/** "1 espera você", "2 esperam você". */
function waitingWords(n: number): string {
  return n === 1 ? '1 espera você' : `${n} esperam você`;
}

/** "3 mensagens novas" (the words the conversation's tab uses, in the tab's name). */
function newWords(n: number): string {
  return n === 1 ? '1 mensagem nova' : `${n} mensagens novas`;
}

export interface MasterLayoutDeps {
  /** How many things wait for the master in "Turno" (a held question, the damage the turn owes, an answer). */
  readonly waiting: Signal<number>;
  /** The conversation's lines that came while its tab was not shown. */
  readonly unread: Signal<number>;
  /** The page gave the combat the conversation to draw in the rail. */
  readonly hasChat: Signal<boolean>;
  /** The panels the page projected into "Mais do combate". */
  readonly projected: Signal<readonly MapTab[]>;
  readonly combatants: Signal<number>;
  /** The combat, for the words of the phone's line (the round, who comes next). */
  readonly encounter: Signal<Encounter | null>;
  /** A combat without a map: no "Mapa" tab on a phone. */
  readonly theatre: Signal<boolean>;
}

/**
 * The tabs of the master's running combat and which one is open (all in memory, nothing on the device): the rail's "Turno · Conversa",
 * the phone's "Turno · Ordem · Mapa" and the rows of "Mais do combate" that are open. The order's fold lives here too.
 */
export class MasterLayout {
  readonly railTab = signal('turn');
  /** The rows of "Mais do combate" that are open. */
  private readonly rowsOpen = signal<ReadonlySet<string>>(new Set());
  readonly phoneTab = signal('turn');
  /** The order column is folded to a strip of tokens (C1-02). */
  readonly orderFolded = signal(false);

  readonly railTabs = computed<readonly TabItem[]>(() => {
    const waiting = this.deps.waiting();
    const unread = this.deps.unread();
    const turn: TabItem = {
      id: 'turn',
      label: 'Turno',
      icon: 'swords',
      ...(waiting > 0
        ? { badge: String(waiting), tone: 'wait' as TabTone, words: waitingWords(waiting) }
        : {}),
    };
    if (!this.deps.hasChat()) {
      return [turn];
    }
    return [
      turn,
      {
        id: 'chat',
        label: 'Conversa',
        icon: 'chat_bubble_outline',
        ...(unread > 0
          ? {
              badge: unread > MAX_BADGE ? `${MAX_BADGE}+` : String(unread),
              tone: 'news' as TabTone,
              words: newWords(unread),
            }
          : {}),
      },
    ];
  });

  /** The rows of "Mais do combate" that apply now, in the board's order: "Registro" and "Efeitos" always, the projected ones while they exist. */
  readonly panels = computed<readonly MorePanel[]>(() =>
    PANEL_ORDER.filter(
      (id) =>
        id === 'log' || id === 'effects' || this.deps.projected().some((t) => t.appMapTab() === id),
    ).map((id) => {
      const count = this.deps
        .projected()
        .find((t) => t.appMapTab() === id)
        ?.mapTabCount();
      return { id, ...PANELS[id], ...(count ? { count } : {}) };
    }),
  );

  isRowOpen(id: string): boolean {
    return this.rowsOpen().has(id);
  }

  setRowOpen(id: string, open: boolean): void {
    const next = new Set(this.rowsOpen());
    if (open) {
      next.add(id);
    } else {
      next.delete(id);
    }
    this.rowsOpen.set(next);
  }

  hasPanel(id: MapTabId): boolean {
    return this.panels().some((p) => p.id === id);
  }

  /** "Rodada 3", for the phone's line under the bar. */
  readonly roundText = computed(() => roundLabel(this.deps.encounter()?.round ?? 0));
  /** Who comes after the one on turn. */
  readonly nextName = computed(() => {
    const e = this.deps.encounter();
    return e ? (turnBanner(e).after?.name ?? '') : '';
  });

  readonly phoneTabs = computed<readonly TabItem[]>(() => {
    const waiting = this.deps.waiting();
    const tabs: TabItem[] = [
      {
        id: 'turn',
        label: 'Turno',
        icon: 'swords',
        ...(waiting > 0
          ? { badge: String(waiting), tone: 'wait' as TabTone, words: waitingWords(waiting) }
          : {}),
      },
      {
        id: 'order',
        label: 'Ordem',
        icon: 'checklist',
        badge: String(this.deps.combatants()),
        tone: 'count',
        words: `${this.deps.combatants()} combatentes`,
      },
    ];
    if (!this.deps.theatre()) {
      tabs.push({ id: 'map', label: 'Mapa', icon: 'map' });
    }
    return tabs;
  });

  readonly shownPhoneTab = computed(() =>
    this.phoneTabs().some((t) => t.id === this.phoneTab()) ? this.phoneTab() : 'turn',
  );

  constructor(private readonly deps: MasterLayoutDeps) {}
}
