import { signal } from '@angular/core';

import { ConversationSlot } from './conversation-slot';

describe('ConversationSlot', () => {
  function setup(over: { phone?: boolean; master?: boolean; ownId?: string } = {}) {
    const live = signal(true);
    const phone = signal(over.phone ?? false);
    const pageVisible = signal(true);
    const stage = signal<readonly string[]>(['n1']);
    const slot = new ConversationSlot({
      live,
      phone,
      pageVisible,
      isMaster: signal(over.master ?? false),
      campaignId: signal('c1'),
      ownCharacterId: signal(over.ownId ?? 'pc1'),
      userId: signal('u-p'),
      stageCharacterIds: stage,
    });
    return { slot, live, phone, pageVisible, stage };
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

  it('hands the stage as a set of character ids', () => {
    expect([...setup().slot.stageIds()]).toEqual(['n1']);
  });

  it('is back on the scene when the desktop width returns', () => {
    const { slot, phone } = setup({ phone: true });
    slot.choose('chat');
    phone.set(false);
    expect(slot.chatOpen()).toBe(false);
  });
});
