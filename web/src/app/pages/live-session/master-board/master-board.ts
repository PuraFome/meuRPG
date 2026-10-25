import { type Signal, computed, effect, signal, untracked } from '@angular/core';
import type { ActivatedRoute, Router } from '@angular/router';

import { type MasterTab, TAB_PARAM, type TabCounts, masterTabs, resolveTab } from './master-tabs';

/** What the board reads from the session page. */
export interface MasterBoardInputs {
  readonly route: ActivatedRoute;
  readonly router: Router;
  /** A scene is open. */
  readonly sceneOpen: Signal<boolean>;
  /** How many rolls the open scene has, in all. */
  readonly rolls: Signal<number>;
  /** The master just opened or swapped the scene here: the board goes to "Cena". */
  readonly openedHere: Signal<boolean>;
  /** A phone: the map is a fold of "Cena" and the tabs are the pinned nav's. */
  readonly phone: Signal<boolean>;
  /** A tablet (768 to 1023 px): the Conversa is a tab of the board, not a rail. */
  readonly narrow?: Signal<boolean>;
  /** The lines that came while the conversation was not on screen. */
  readonly unread?: Signal<number>;
}

/**
 * The master's board outside a combat: which of "Cena", "Mapa" and "Grupo" is shown, and what their names count. The tab lives in
 * the address (`?vista=mapa`), so a reload keeps it; with no name in it the board opens on "Cena" while a scene is open and on
 * "Mapa" otherwise. A scene the master opens or swaps takes the board to "Cena". The panels of the other tabs stay mounted
 * (hidden), so a draft and a place in a list survive a change of tab.
 */
export class MasterBoard {
  private readonly param = signal<string | null>(null);
  /** The rolls the master has looked at: those of the scene while "Cena" is shown. */
  private readonly seenRolls = signal(0);
  private readonly castsWaiting = signal(0);

  readonly tab = computed<MasterTab>(() => {
    const tab = resolveTab(this.param(), this.inputs.sceneOpen());
    // On a phone the map is a fold of the scene's page: there is no tab for it.
    if (this.inputs.phone() && (tab === 'mapa' || tab === 'conversa')) {
      return 'cena';
    }
    // The conversation is a tab only where it has no rail and no pinned nav.
    return tab === 'conversa' && !this.chatTab() ? 'cena' : tab;
  });
  /** The new scene rolls, counted from when the master last had "Cena" on screen. */
  readonly newRolls = computed(() => Math.max(this.inputs.rolls() - this.seenRolls(), 0));
  /** The Conversa is a tab of the board (a tablet). */
  readonly chatTab = computed(() => !!this.inputs.narrow?.() && !this.inputs.phone());
  readonly counts = computed<TabCounts>(() => ({
    newRolls: this.tab() === 'cena' ? 0 : this.newRolls(),
    castsWaiting: this.castsWaiting(),
    unread: this.tab() === 'conversa' ? 0 : (this.inputs.unread?.() ?? 0),
  }));
  readonly tabs = masterTabs(this.counts, this.chatTab);
  /** A phone: no tab list, the pinned nav chooses. */
  readonly phone = computed(() => this.inputs.phone());

  constructor(private readonly inputs: MasterBoardInputs) {
    inputs.route.queryParamMap.subscribe((params) => this.param.set(params.get(TAB_PARAM)));
    let wasOpen = false;
    effect(() => {
      // While "Cena" is on screen every roll is seen; a scene that has just appeared (opened, or read when the page loads) is
      // seen as it stands; the count starts from there when another tab is chosen.
      const open = this.inputs.sceneOpen();
      const here = this.tab() === 'cena' || !open || !wasOpen;
      wasOpen = open;
      const rolls = this.inputs.rolls();
      if (here) {
        untracked(() => this.seenRolls.set(rolls));
      }
    });
    effect(() => {
      if (this.inputs.openedHere()) {
        untracked(() => this.choose('cena'));
      }
    });
  }

  /** The tab the master chose: it goes in the address, replacing the entry, so "back" leaves the page. */
  choose(tab: string): void {
    void this.inputs.router.navigate([], {
      queryParams: { [TAB_PARAM]: tab },
      queryParamsHandling: 'merge',
      replaceUrl: true,
    });
  }

  /** How many long casts wait for the master (the casting panel says it). */
  setCastsWaiting(n: number): void {
    this.castsWaiting.set(n);
  }
}
