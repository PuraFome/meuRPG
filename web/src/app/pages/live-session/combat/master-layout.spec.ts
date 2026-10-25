import { signal } from '@angular/core';

import type { MapTab } from './map-tabs/map-tab';
import { MasterLayout } from './master-layout';

function tab(id: string, count?: number): MapTab {
  return { appMapTab: () => id, mapTabCount: () => count } as unknown as MapTab;
}

describe('MasterLayout', () => {
  function setup() {
    const waiting = signal(0);
    const unread = signal(0);
    const hasChat = signal(true);
    const projected = signal<readonly MapTab[]>([]);
    const effects = signal(0);
    const combatants = signal(6);
    const theatre = signal(false);
    const layout = new MasterLayout({
      waiting,
      unread,
      hasChat,
      projected,
      combatants,
      effects,
      encounter: signal(null),
      theatre,
    });
    return { layout, waiting, unread, hasChat, projected, combatants, theatre, effects };
  }

  it('has the rail tabs "Turno" and "Conversa", with the counts in words', () => {
    const { layout, waiting, unread } = setup();
    expect(layout.railTabs().map((t) => t.label)).toEqual(['Turno', 'Conversa']);
    expect(layout.railTabs()[0].badge).toBeUndefined();
    waiting.set(2);
    unread.set(3);
    expect(layout.railTabs()[0]).toMatchObject({
      badge: '2',
      tone: 'wait',
      words: '2 esperam você',
    });
    expect(layout.railTabs()[1]).toMatchObject({
      badge: '3',
      tone: 'news',
      words: '3 mensagens novas',
    });
    waiting.set(1);
    unread.set(1);
    expect(layout.railTabs()[0].words).toBe('1 espera você');
    expect(layout.railTabs()[1].words).toBe('1 mensagem nova');
  });

  it('has only "Turno" where the page keeps the conversation elsewhere', () => {
    const { layout, hasChat } = setup();
    hasChat.set(false);
    expect(layout.railTabs().map((t) => t.id)).toEqual(['turn']);
  });

  it('caps the unread badge at 99+', () => {
    const { layout, unread } = setup();
    unread.set(120);
    expect(layout.railTabs()[1].badge).toBe('99+');
  });

  it('has the rows "Registro" and "Efeitos" always, and the projected panels in the board order, only when they apply', () => {
    const { layout, projected } = setup();
    expect(layout.panels().map((p) => p.label)).toEqual(['Registro', 'Efeitos']);
    projected.set([tab('treasure', 1), tab('fog'), tab('traps', 2)]);
    expect(layout.panels().map((p) => p.label)).toEqual([
      'Registro',
      'Efeitos',
      'Névoa e luz',
      'Armadilhas',
      'Tesouro',
    ]);
    expect(layout.panels().find((p) => p.id === 'traps')?.count).toBe(2);
    expect(layout.panels().find((p) => p.id === 'fog')?.count).toBeUndefined();
    expect(layout.hasPanel('fog')).toBe(true);
    expect(layout.hasPanel('puzzles')).toBe(false);
  });

  it('says the count of the effects in their row, like the traps', () => {
    const { layout, effects } = setup();
    expect(layout.panels().find((p) => p.id === 'effects')?.count).toBeUndefined();
    effects.set(2);
    expect(layout.panels().find((p) => p.id === 'effects')?.count).toBe(2);
  });

  it('keeps each row open or folded by itself', () => {
    const { layout } = setup();
    expect(layout.isRowOpen('effects')).toBe(false);
    layout.setRowOpen('effects', true);
    layout.setRowOpen('fog', true);
    expect(layout.isRowOpen('effects') && layout.isRowOpen('fog')).toBe(true);
    layout.setRowOpen('effects', false);
    expect(layout.isRowOpen('effects')).toBe(false);
    expect(layout.isRowOpen('fog')).toBe(true);
  });

  it('has the phone tabs "Turno · Ordem · Mapa", and no "Mapa" without a map', () => {
    const { layout, theatre, waiting } = setup();
    expect(layout.phoneTabs().map((t) => t.label)).toEqual(['Turno', 'Ordem', 'Mapa']);
    expect(layout.phoneTabs()[1]).toMatchObject({ badge: '6', words: '6 combatentes' });
    waiting.set(1);
    expect(layout.phoneTabs()[0]).toMatchObject({ badge: '1', tone: 'wait' });
    theatre.set(true);
    expect(layout.phoneTabs().map((t) => t.label)).toEqual(['Turno', 'Ordem']);
    layout.phoneTab.set('map');
    expect(layout.shownPhoneTab()).toBe('turn');
  });

  it('keeps the order folded only in memory, open at the start', () => {
    const { layout } = setup();
    expect(layout.orderFolded()).toBe(false);
  });
});
