import { type Signal, computed } from '@angular/core';

import type { TabItem } from '../../../shared/tab-list/tab-list';

/** The master's tabs outside a combat (C3, C6-05, C6-06): the scene, the map and the group. */
export type MasterTab = 'cena' | 'mapa' | 'grupo' | 'conversa';

/** The query parameter that names the tab, so a reload keeps it and nothing is stored on the device. */
export const TAB_PARAM = 'vista';

const TABS: readonly MasterTab[] = ['cena', 'mapa', 'grupo', 'conversa'];

/** The tab a person gets: the one the address names, else "Cena" while a scene is open and "Mapa" otherwise. */
export function resolveTab(param: string | null, sceneOpen: boolean): MasterTab {
  const named = TABS.find((t) => t === param);
  return named ?? (sceneOpen ? 'cena' : 'mapa');
}

/** What the tabs count: the new scene rolls, the people and things waiting in the group. */
export interface TabCounts {
  /** Rolls that came to the open scene and nobody has looked at since. */
  readonly newRolls: number;
  /** Long casts that wait for the master to finish them. */
  readonly castsWaiting: number;
  /** Lines of the conversation that came while another tab was shown (only a tablet has the tab). */
  readonly unread?: number;
}

const WORDS: Readonly<Record<MasterTab, { label: string; icon: string }>> = {
  cena: { label: 'Cena', icon: 'forum' },
  mapa: { label: 'Mapa', icon: 'map' },
  grupo: { label: 'Grupo', icon: 'groups' },
  conversa: { label: 'Conversa', icon: 'chat_bubble_outline' },
};

/** The count as the eye reads it, and in words for the tab's name. */
function counted(n: number, one: string, many: string): Pick<TabItem, 'badge' | 'tone' | 'words'> {
  return n > 0 ? { badge: String(n), tone: 'wait', words: `${n} ${n === 1 ? one : many}` } : {};
}

/** The tab list of the master's board; the count of "Cena" is amber (something to read), the count of "Grupo" too (a cast waits). */
export function masterTabItems(counts: TabCounts, withChat = false): readonly TabItem[] {
  return TABS.filter((id) => withChat || id !== 'conversa').map((id) => {
    const base: TabItem = { id, ...WORDS[id] };
    if (id === 'cena') {
      return { ...base, ...counted(counts.newRolls, 'rolagem nova', 'rolagens novas') };
    }
    if (id === 'grupo') {
      return {
        ...base,
        ...counted(counts.castsWaiting, 'conjuração esperando', 'conjurações esperando'),
      };
    }
    if (id === 'conversa') {
      const n = counts.unread ?? 0;
      return n > 0
        ? {
            ...base,
            badge: String(n),
            tone: 'news',
            words: `${n} ${n === 1 ? 'mensagem nova' : 'mensagens novas'}`,
          }
        : base;
    }
    return base;
  });
}

/** The tab items as a signal of the counts. */
export function masterTabs(
  counts: Signal<TabCounts>,
  withChat: Signal<boolean> = computed(() => false),
): Signal<readonly TabItem[]> {
  return computed(() => masterTabItems(counts(), withChat()));
}
