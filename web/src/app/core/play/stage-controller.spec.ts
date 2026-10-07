import { SceneState } from './scene-state';
import { FakeSceneClient, masterScene, stageNpc } from './scene-testing';
import { StageController } from './stage-controller';
import { SceneBlockedSchema, SceneBlockedReason } from '../../../gen/meurpg/play/v1/scene_pb';
import { Code, ConnectError } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';

describe('StageController', () => {
  function setup(stage = [stageNpc('s-m', 'Mira', { master: true, characterId: 'mira' })]) {
    const api = new FakeSceneClient();
    api.stage = stage;
    api.names = { aldo: 'Aldo', ivo: 'Barão Ivo', mira: 'Mira' };
    const state = new SceneState(
      () => Promise.resolve(masterScene([], api.stage)),
      () => true,
    );
    state.apply(masterScene([], stage));
    const ctl = new StageController(
      api,
      state,
      () => 'c1',
      (id) => api.names[id] ?? id,
    );
    return { api, state, ctl };
  }

  it('puts an NPC on the stage, applies the answer at once and says so', async () => {
    const { state, ctl } = setup();
    expect(await ctl.put('aldo')).toBe(true);
    expect(state.stage().map((n) => n.name)).toEqual(['Mira', 'Aldo']);
    expect(ctl.status()).toBe('Aldo entrou na cena.');
    expect(ctl.entered()).toBe('Aldo entrou na cena.');
  });

  it('gives the speech, and takes it back when the speaker is pressed again', async () => {
    const { api, state, ctl } = setup();
    await ctl.speak('mira', false);
    expect(state.stage()[0].speaking).toBe(true);
    expect(ctl.status()).toBe('Mira fala.');
    await ctl.speak('mira', true);
    expect(state.stage()[0].speaking).toBe(false);
    expect(ctl.status()).toBe('Ninguém fala.');
    expect(api.calls).toEqual(['speaker mira', 'speaker nobody']);
  });

  it('takes an NPC off at once and says who left', async () => {
    const { state, ctl } = setup();
    expect(await ctl.take('mira')).toBe(true);
    expect(state.stage()).toEqual([]);
    expect(ctl.status()).toBe('Mira saiu da cena.');
    expect(ctl.entered()).toBe('');
  });

  it('runs one write at a time per NPC and lets another NPC go meanwhile', async () => {
    const { api, ctl } = setup();
    let release: () => void = () => undefined;
    api.hold = new Promise<void>((r) => (release = r));
    const first = ctl.put('aldo');
    expect(ctl.busy('aldo')).toBe(true);
    expect(await ctl.put('aldo')).toBe(false);
    const other = ctl.put('ivo');
    expect(ctl.busy('ivo')).toBe(true);
    release();
    expect(await first).toBe(true);
    expect(await other).toBe(true);
    expect(api.calls).toEqual(['put aldo', 'put ivo']);
    expect(ctl.busy('aldo')).toBe(false);
  });

  it('says why a full stage refuses, by the typed detail, and reads the scene again', async () => {
    const { api, ctl } = setup();
    const refresh = vi.spyOn(ctl['state'], 'refresh');
    api.failWith = new ConnectError('full', Code.FailedPrecondition, undefined, [
      {
        desc: SceneBlockedSchema,
        value: create(SceneBlockedSchema, { reason: SceneBlockedReason.STAGE_FULL }),
      },
    ]);
    expect(await ctl.put('aldo')).toBe(false);
    expect(ctl.error()).toBe('A cena comporta 4 NPCs. Tire um para pôr outro.');
    expect(ctl.entered()).toBe('');
    expect(refresh).toHaveBeenCalled();
    expect(ctl.busy('aldo')).toBe(false);
  });

  it('says the scene is gone when it closed meanwhile', async () => {
    const { api, ctl } = setup();
    api.failWith = new ConnectError('gone', Code.FailedPrecondition, undefined, [
      {
        desc: SceneBlockedSchema,
        value: create(SceneBlockedSchema, { reason: SceneBlockedReason.NO_OPEN_SCENE }),
      },
    ]);
    await ctl.take('mira');
    expect(ctl.error()).toContain('Não há cena aberta');
  });

  it('knows whether the stage has room', async () => {
    const { ctl } = setup([1, 2, 3, 4].map((n) => stageNpc(`s${n}`, `N${n}`, { master: true })));
    expect(ctl.hasRoom()).toBe(false);
    await ctl.take('ch-s1');
    expect(ctl.hasRoom()).toBe(true);
  });

  it('clears its messages', async () => {
    const { ctl } = setup();
    await ctl.put('aldo');
    ctl.clearMessages();
    expect(ctl.status()).toBe('');
    expect(ctl.entered()).toBe('');
    expect(ctl.error()).toBe('');
  });
});
