import { NpcCheckLauncher } from './npc-check-launcher';

describe('NpcCheckLauncher', () => {
  it('keeps the request until the host takes it', () => {
    const launcher = new NpcCheckLauncher();
    launcher.ask('npc-1');
    expect(launcher.request()).toEqual({ characterId: 'npc-1' });
    launcher.take();
    expect(launcher.request()).toBeNull();
  });
});
