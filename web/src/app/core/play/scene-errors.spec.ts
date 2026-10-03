import { Code, ConnectError } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';

import { GameSessionBlockedReason, GameSessionBlockedSchema } from '../../../gen/meurpg/play/v1/play_pb';
import { SceneBlockedReason, SceneBlockedSchema } from '../../../gen/meurpg/play/v1/scene_pb';
import { sceneBlockedMessage, sceneErrorMessage } from './scene-errors';

function blocked(reason: SceneBlockedReason): ConnectError {
  return new ConnectError('no', Code.FailedPrecondition, undefined, [
    { desc: SceneBlockedSchema, value: create(SceneBlockedSchema, { reason }) },
  ]);
}

describe('scene errors', () => {
  const reasons = [
    SceneBlockedReason.NO_ACTIONS,
    SceneBlockedReason.NO_OPEN_SCENE,
    SceneBlockedReason.ALREADY_ROLLED,
    SceneBlockedReason.WRONG_DICE_MODE,
    SceneBlockedReason.NO_CHARACTER,
  ];

  it('has its own Portuguese sentence for every refusal', () => {
    const sentences = reasons.map((r) => sceneBlockedMessage(r));
    expect(new Set(sentences).size).toBe(reasons.length);
    for (const s of sentences) {
      expect(s).toMatch(/[a-zà-ú]/);
      expect(s).not.toMatch(/SCENE|ALREADY|NO_/);
    }
    expect(sceneBlockedMessage(SceneBlockedReason.NO_ACTIONS)).toContain('não tem ações');
    expect(sceneBlockedMessage(SceneBlockedReason.NO_OPEN_SCENE)).toContain('fechou a cena');
    expect(sceneBlockedMessage(SceneBlockedReason.ALREADY_ROLLED)).toContain('já rolou');
    expect(sceneBlockedMessage(SceneBlockedReason.WRONG_DICE_MODE)).toContain('forma de rolar');
    expect(sceneBlockedMessage(SceneBlockedReason.NO_CHARACTER)).toContain('personagem vivo');
  });

  it('reads the typed detail, never the message', () => {
    for (const reason of reasons) {
      const err = blocked(reason);
      expect(sceneErrorMessage(err)).toBe(sceneBlockedMessage(reason));
    }
    // A message that only looks like a reason says nothing.
    expect(sceneErrorMessage(new ConnectError('ALREADY_ROLLED', Code.FailedPrecondition))).not.toBe(
      sceneBlockedMessage(SceneBlockedReason.ALREADY_ROLLED),
    );
  });

  it('says the session is over when it has no open session', () => {
    const err = new ConnectError('x', Code.FailedPrecondition, undefined, [
      {
        desc: GameSessionBlockedSchema,
        value: create(GameSessionBlockedSchema, { reason: GameSessionBlockedReason.NO_OPEN_SESSION }),
      },
    ]);
    expect(sceneErrorMessage(err)).toContain('A sessão acabou');
  });

  it('maps the other codes', () => {
    expect(sceneErrorMessage(new ConnectError('x', Code.NotFound))).toContain('não existe mais');
    expect(sceneErrorMessage(new ConnectError('x', Code.PermissionDenied))).toContain('não pode');
    expect(sceneErrorMessage(new ConnectError('x', Code.Unavailable), 'rolar')).toContain('Não deu para rolar');
    expect(sceneErrorMessage(new Error('network'), 'rolar')).toContain('Não deu para rolar');
  });
});
