import { type MessageInitShape, create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';

import {
  type OpenSceneInfo,
  OpenSceneInfoSchema,
  type SceneActionView,
  SceneActionViewSchema,
  type SceneRoll,
  SceneRollSchema,
  type StageNpc,
  StageNpcSchema,
} from '../../../gen/meurpg/play/v1/scene_pb';
import { DiceRollSchema } from '../../../gen/meurpg/play/v1/combat_pb';
import type { SceneDie } from './scene-client';

/**
 * Builders and a stand-in for the scene specs (never imported by the app
 * itself, so never bundled): the messages as the server sends them, and a
 * `SceneClient` that remembers its calls. Always built with `create`, so a new
 * field of the proto never leaves a spec with a hand-built object missing it.
 */
export function sceneAction(
  id: string,
  checkName: string,
  partial: MessageInitShape<typeof SceneActionViewSchema> = {},
): SceneActionView {
  return create(SceneActionViewSchema, { id, key: 'skill:x', checkName, ...partial });
}

export function sceneRoll(
  id: string,
  actionId: string,
  characterName: string,
  total: number,
  partial: { roll?: MessageInitShape<typeof DiceRollSchema>; passed?: boolean } = {},
): SceneRoll {
  const init: MessageInitShape<typeof SceneRollSchema> = {
    id,
    actionId,
    characterId: characterName.toLowerCase(),
    characterName,
    roll: { diceCount: 1, diceSides: 20, faces: [total - 1], modifier: 1, total },
    rolledAt: timestampFromDate(new Date(2026, 9, 3, 21, 14)),
    ...partial,
  };
  return create(SceneRollSchema, init);
}

/**
 * An NPC on the stage as the server sends it. A player's entry has no
 * `characterId` (RN-20); the master's has one (`ch-<id>` unless given).
 */
export function stageNpc(
  id: string,
  name: string,
  partial: MessageInitShape<typeof StageNpcSchema> & { master?: boolean } = {},
): StageNpc {
  const { master, ...rest } = partial;
  return create(StageNpcSchema, {
    id,
    name,
    characterId: master ? `ch-${id}` : '',
    portraitUrl: '',
    speaking: false,
    ...rest,
  });
}

/** "A carroça tombada" as the master sees it: five actions with their DCs. */
export function masterScene(rolls: SceneRoll[] = [], stage: StageNpc[] = []): OpenSceneInfo {
  return create(OpenSceneInfoSchema, {
    stage,
    pointId: 'p1',
    name: 'A carroça tombada',
    description: 'Uma carroça de mercador tombada na estrada.',
    actions: [
      sceneAction('a1', 'Investigação', { name: 'Procurar pistas na carroça', dc: 12, key: 'skill:investigation' }),
      sceneAction('a2', 'Sobrevivência', { name: 'Seguir os rastros dos goblins', dc: 13 }),
      sceneAction('a3', 'Adestrar Animais', { name: 'Acalmar os cavalos' }),
      sceneAction('a4', 'Percepção'),
      sceneAction('a5', 'Salvaguarda de Constituição', { name: 'Resistir ao cheiro de fumaça', dc: 10, key: 'save:con' }),
    ],
    rolls,
    openedAt: timestampFromDate(new Date(2026, 9, 3, 21, 10)),
  });
}

/** The same scene as Pensantus sees it: his bonuses, no DC. */
export function playerScene(rolls: SceneRoll[] = [], stage: StageNpc[] = []): OpenSceneInfo {
  return create(OpenSceneInfoSchema, {
    stage,
    pointId: 'p1',
    name: 'A carroça tombada',
    description: 'Uma carroça de mercador tombada na estrada.',
    actions: [
      sceneAction('a1', 'Investigação', { name: 'Procurar pistas na carroça', bonus: 6, passive: 16, key: 'skill:investigation' }),
      sceneAction('a2', 'Sobrevivência', { name: 'Seguir os rastros dos goblins', bonus: 1 }),
      sceneAction('a3', 'Adestrar Animais', { name: 'Acalmar os cavalos', bonus: 1 }),
      sceneAction('a4', 'Percepção', { bonus: 1, passive: 11, key: 'skill:perception' }),
      sceneAction('a5', 'Salvaguarda de Constituição', { name: 'Resistir ao cheiro de fumaça', bonus: 3, key: 'save:con' }),
    ],
    rolls,
    openedAt: timestampFromDate(new Date(2026, 9, 3, 21, 10)),
  });
}

/** A `SceneClient` over one scene: every call is recorded in `calls`. */
export class FakeSceneClient {
  scene: OpenSceneInfo | null = null;
  calls: string[] = [];
  failWith: unknown = null;
  /** The roll `roll` answers with. */
  made: SceneRoll = sceneRoll('r1', 'a1', 'Pensantus', 17, {
    roll: { diceCount: 1, diceSides: 20, faces: [11], modifier: 6, total: 17 },
  });

  /** The stage the stage calls answer with; `names` says who each NPC is. */
  stage: StageNpc[] = [];
  names: Record<string, string> = {};
  /** Holds the next stage call until `release` runs, to prove one write per NPC. */
  hold: Promise<void> | null = null;

  private record(call: string): void {
    this.calls.push(call);
    if (this.failWith) {
      throw this.failWith;
    }
  }

  async open(_campaignId: string, pointId: string): Promise<OpenSceneInfo> {
    this.record(`open ${pointId}`);
    return this.scene ?? masterScene();
  }

  async close(): Promise<void> {
    this.record('close');
  }

  async get(): Promise<OpenSceneInfo | null> {
    this.record('get');
    return this.scene;
  }

  async putOnStage(_campaignId: string, characterId: string): Promise<readonly StageNpc[]> {
    this.record(`put ${characterId}`);
    await this.hold;
    if (!this.stage.some((n) => n.characterId === characterId)) {
      this.stage = [...this.stage, stageNpc(`s-${characterId}`, this.names[characterId] ?? characterId, { master: true, characterId })];
    }
    return this.stage;
  }

  async takeOffStage(_campaignId: string, characterId: string): Promise<readonly StageNpc[]> {
    this.record(`take ${characterId}`);
    await this.hold;
    this.stage = this.stage.filter((n) => n.characterId !== characterId);
    return this.stage;
  }

  async setSpeaker(_campaignId: string, characterId: string): Promise<readonly StageNpc[]> {
    this.record(`speaker ${characterId || 'nobody'}`);
    await this.hold;
    this.stage = this.stage.map((n) => ({ ...n, speaking: n.characterId === characterId }));
    return this.stage;
  }

  async roll(_campaignId: string, actionId: string, die: SceneDie, key: string): Promise<SceneRoll> {
    this.record(`roll ${actionId} ${'inApp' in die ? 'app' : die.face} ${key}`);
    return this.made;
  }
}
