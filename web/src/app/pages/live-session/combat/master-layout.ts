import { type Signal, computed, signal } from '@angular/core';

import type { TabItem, TabTone } from '../../../shared/tab-list/tab-list';
import type { MapTab, MapTabId } from './map-tabs/map-tab';

/** What a tab of the tabs under the map says: its words and icon (the order the board draws them in). */
const MAP_TABS: Readonly<Record<'log' | 'effects' | MapTabId, { label: string; icon: string }>> = {
  log: { label: 'Registro', icon: 'history' },
  effects: { label: 'Efeitos', icon: 'hourglass_empty' },
  fog: { label: 'Névoa e luz', icon: 'lightbulb' },
  traps: { label: 'Armadilhas', icon: 'warning' },
  treasure: { label: 'Tesouro', icon: 'star_outline' },
  puzzles: { label: 'Quebra-cabeças', icon: 'extension' },
};
const MAP_ORDER = ['log', 'effects', 'fog', 'traps', 'treasure', 'puzzles'] as const;

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
  /** The panels the page projected under the map. */
  readonly projected: Signal<readonly MapTab[]>;
  readonly combatants: Signal<number>;
  /** A combat without a map: no "Mapa" tab on a phone. */
  readonly theatre: Signal<boolean>;
}

/**
 * The tabs of the master's running combat and which one is open (all in memory, nothing on the device): the rail's "Turno · Conversa",
 * the set under the map and the phone's "Turno · Ordem · Mapa". The order's fold lives here too.
 */
export class MasterLayout {
  readonly railTab = signal('turn');
  readonly mapTab = signal('log');
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
              badge: unread > 99 ? '99+' : String(unread),
              tone: 'news' as TabTone,
              words: newWords(unread),
            }
          : {}),
      },
    ];
  });

  /** The ids of the tabs under the map that apply now, in the board's order. */
  private readonly mapIds = computed<readonly string[]>(() => {
    const present = new Set(this.deps.projected().map((t) => t.appMapTab()));
    return MAP_ORDER.filter((id) => id === 'log' || id === 'effects' || present.has(id));
  });

  readonly mapTabItems = computed<readonly TabItem[]>(() =>
    this.mapIds().map((id) => {
      const count = this.deps
        .projected()
        .find((t) => t.appMapTab() === id)
        ?.count();
      return {
        id,
        ...MAP_TABS[id as keyof typeof MAP_TABS],
        ...(count ? { badge: String(count), tone: 'count' as TabTone } : {}),
      };
    }),
  );

  /** The open tab under the map, the first when the one chosen no longer applies. */
  readonly shownMapTab = computed(() =>
    this.mapIds().includes(this.mapTab()) ? this.mapTab() : this.mapIds()[0],
  );

  hasMapTab(id: MapTabId): boolean {
    return this.mapIds().includes(id);
  }

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
