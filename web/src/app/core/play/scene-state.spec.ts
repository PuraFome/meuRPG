import type { OpenSceneInfo } from '../../../gen/meurpg/play/v1/scene_pb';
import { SceneState } from './scene-state';
import { masterScene, playerScene, sceneRoll } from './scene-testing';

describe('SceneState', () => {
  function stateWith(isMaster: boolean, ...answers: (OpenSceneInfo | null | Error)[]): SceneState {
    let i = 0;
    return new SceneState(
      () => {
        const next = answers[Math.min(i++, answers.length - 1)];
        return next instanceof Error ? Promise.reject(next) : Promise.resolve(next);
      },
      () => isMaster,
    );
  }

  it('says nothing for the first read (the page was just opened)', async () => {
    const state = stateWith(false, playerScene());
    await state.refresh();
    expect(state.scene()?.name).toBe('A carroça tombada');
    expect(state.notice()).toBe('');
  });

  it('tells a player when the master opens a scene and when it closes', async () => {
    const state = stateWith(false, null, playerScene(), null);
    await state.refresh();
    await state.refresh();
    expect(state.notice()).toBe('O mestre abriu uma cena: A carroça tombada.');
    await state.refresh();
    expect(state.notice()).toBe('O mestre fechou a cena.');
    expect(state.scene()).toBeNull();
  });

  it("does not tell a player about another player's roll or their own new row", async () => {
    const state = stateWith(false, playerScene(), playerScene([sceneRoll('r1', 'a1', 'Pensantus', 17)]));
    await state.refresh();
    await state.refresh();
    expect(state.notice()).toBe('');
  });

  it('reads each new roll aloud to the master, oldest of the news first', async () => {
    const toren = sceneRoll('r1', 'a2', 'Toren', 7, { passed: false });
    const pens = sceneRoll('r2', 'a1', 'Pensantus', 17, { passed: true });
    const state = stateWith(true, masterScene(), masterScene([toren]), masterScene([pens, toren]));
    await state.refresh();
    await state.refresh();
    expect(state.notice()).toBe('Toren: Seguir os rastros dos goblins, 7, não passou');
    await state.refresh();
    expect(state.notice()).toBe('Pensantus: Procurar pistas na carroça, 17, passou');
  });

  it('keeps what it had when a read fails', async () => {
    const state = stateWith(false, playerScene(), new Error('network'));
    await state.refresh();
    await state.refresh();
    expect(state.scene()?.name).toBe('A carroça tombada');
  });

  it('never lets a late answer overwrite a newer one', async () => {
    let release: (s: OpenSceneInfo | null) => void = () => undefined;
    const slow = new Promise<OpenSceneInfo | null>((r) => (release = r));
    let calls = 0;
    const state = new SceneState(() => (calls++ === 0 ? slow : Promise.resolve(null)), () => false);
    const first = state.refresh();
    await state.refresh();
    release(playerScene());
    await first;
    expect(state.scene()).toBeNull();
  });

  it("marks where focus goes after the master's own open and close", () => {
    const state = stateWith(true, null);
    state.openedHere(masterScene());
    expect(state.focusNext()).toBe('title');
    state.focusNext.set(null);
    state.closedHere();
    expect(state.focusNext()).toBe('open');
    expect(state.scene()).toBeNull();
    state.clear();
    expect(state.focusNext()).toBeNull();
  });
});
