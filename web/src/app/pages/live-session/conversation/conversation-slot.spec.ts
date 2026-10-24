import { signal } from '@angular/core';

import { ConversationSlot } from './conversation-slot';

describe('ConversationSlot', () => {
  function setup(
    over: { phone?: boolean; master?: boolean; ownId?: string; railed?: boolean } = {},
  ) {
    const live = signal(true);
    const phone = signal(over.phone ?? false);
    const pageVisible = signal(true);
    const stage = signal<readonly string[]>(['n1']);
    const unread = signal(0);
    const slot = new ConversationSlot({
      live,
      phone,
      pageVisible,
      isMaster: signal(over.master ?? false),
      campaignId: signal('c1'),
      ownCharacterId: signal(over.ownId ?? 'pc1'),
      userId: signal('u-p'),
      stageCharacterIds: stage,
      unread,
      railed: signal(over.railed ?? false),
    });
    return { slot, live, phone, pageVisible, stage, unread };
  }

  it('is on screen on the desktop while the session is live and the page is visible', () => {
    const { slot, live, pageVisible } = setup();
    expect(slot.onScreen()).toBe(true);
    pageVisible.set(false);
    expect(slot.onScreen()).toBe(false);
    pageVisible.set(true);
    live.set(false);
    expect(slot.onScreen()).toBe(false);
  });

  it('is on screen on a phone only in the Conversa tab', () => {
    const { slot } = setup({ phone: true });
    expect(slot.onScreen()).toBe(false);
    slot.choose('chat');
    expect(slot.onScreen()).toBe(true);
    expect(slot.chatOpen()).toBe(true);
    slot.choose('scene');
    expect(slot.onScreen()).toBe(false);
  });

  it('loads the panel at once on the desktop, and on a phone only once the tab was opened', () => {
    expect(setup().slot.wanted()).toBe(true);
    const { slot } = setup({ phone: true });
    expect(slot.wanted()).toBe(false);
    slot.choose('chat');
    expect(slot.wanted()).toBe(true);
    slot.choose('scene');
    expect(slot.wanted()).toBe(true);
  });

  it('does not want the panel before the session is live', () => {
    const { slot, live } = setup();
    live.set(false);
    expect(slot.wanted()).toBe(false);
  });

  it('links "Ficha" to the own character of a player and to the campaign for the master', () => {
    expect(setup().slot.sheetLink()).toEqual(['/campaigns', 'c1', 'characters', 'pc1']);
    expect(setup({ ownId: '' }).slot.sheetLink()).toEqual(['/campaigns', 'c1']);
    expect(setup({ master: true }).slot.sheetLink()).toEqual(['/campaigns', 'c1']);
  });

  it('opens "Ficha" inside the session on a phone: the chat is not on screen and the count goes on', () => {
    const { slot, unread } = setup({ phone: true });
    slot.choose('sheet');
    expect(slot.sheetOpen()).toBe(true);
    expect(slot.chatOpen()).toBe(false);
    expect(slot.onScreen()).toBe(false);
    unread.set(2);
    expect(slot.news()).toBe(2);
    slot.choose('chat');
    expect(slot.sheetOpen()).toBe(false);
  });

  it('has no open "Ficha" panel when the screen is not a phone', () => {
    const { slot } = setup();
    slot.choose('sheet');
    expect(slot.sheetOpen()).toBe(false);
  });

  it('counts the lines while the rail shows "Ficha", and not while it shows "Conversa"', () => {
    const { slot } = setup({ railed: true });
    expect(slot.onScreen()).toBe(true);
    expect(slot.shown()).toBe(true);
    slot.railTab.set('sheet');
    expect(slot.onScreen()).toBe(false);
    expect(slot.shown()).toBe(false);
    slot.railTab.set('chat');
    expect(slot.onScreen()).toBe(true);
  });

  it('hands the stage as a set of character ids', () => {
    expect([...setup().slot.stageIds()]).toEqual(['n1']);
  });

  it('is back on the scene when the desktop width returns', () => {
    const { slot, phone } = setup({ phone: true });
    slot.choose('chat');
    phone.set(false);
    expect(slot.chatOpen()).toBe(false);
  });

  describe('the news notice at the top of the page', () => {
    it('counts the unread lines on a phone while the chat is closed', () => {
      const { slot, unread } = setup({ phone: true });
      expect(slot.news()).toBe(0);
      unread.set(3);
      expect(slot.news()).toBe(3);
    });

    it('is 0 while the chat is open, and back when it closes with news', () => {
      const { slot, unread } = setup({ phone: true });
      unread.set(2);
      slot.choose('chat');
      expect(slot.news()).toBe(0);
      slot.choose('scene');
      unread.set(1);
      expect(slot.news()).toBe(1);
    });

    it('is 0 on a computer, where the column is on screen, and while the session is not live', () => {
      const desktop = setup();
      desktop.unread.set(4);
      expect(desktop.slot.news()).toBe(0);
      const { slot, unread, live } = setup({ phone: true });
      unread.set(4);
      live.set(false);
      expect(slot.news()).toBe(0);
    });
  });
});
