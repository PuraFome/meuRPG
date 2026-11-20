import { NpcCheckLauncher } from './npc-check-launcher';

describe('NpcCheckLauncher', () => {
  it('waits only while a refusal for a combat stands and the page still shows the combat', () => {
    const launcher = new NpcCheckLauncher();
    expect(launcher.blocked()).toBe(false);
    launcher.combatOpen.set(true);
    expect(launcher.blocked()).toBe(false);
    launcher.pageCombat.set(true);
    expect(launcher.blocked()).toBe(true);
    launcher.pageCombat.set(false);
    expect(launcher.blocked()).toBe(false);
  });

  it('keeps the request until the host takes it', () => {
    const launcher = new NpcCheckLauncher();
    launcher.ask('npc-1');
    expect(launcher.request()).toEqual({ characterId: 'npc-1' });
    launcher.take();
    expect(launcher.request()).toBeNull();
  });
});
