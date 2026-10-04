import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import {
  CharacterBlockedReason,
  CharacterBlockedSchema,
  LevelUpRefusalReason,
  LevelUpRefusalSchema,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { describeLevelUpFailure, refusalMessage, refusalStep } from './levelup-errors';

function refused(reason: LevelUpRefusalReason) {
  return new ConnectError('x', Code.FailedPrecondition, undefined, [
    { desc: LevelUpRefusalSchema, value: create(LevelUpRefusalSchema, { reason, field: 'full.cantrip_keys' }) },
  ]);
}

describe('the failures of the guided level-up', () => {
  it('reads a stale revision (aborted) as "read the sheet again"', () => {
    const f = describeLevelUpFailure(new ConnectError('x', Code.Aborted));
    expect(f.kind).toBe('stale');
    expect(f.message).toContain('A ficha mudou');
  });

  it('reads a refusal by its reason, never its message, and points at the step that owns it', () => {
    const f = describeLevelUpFailure(refused(LevelUpRefusalReason.CANTRIPS));
    expect(f).toMatchObject({ kind: 'refusal', step: 'spells' });
    expect(f.message).toBe('Escolha todos os truques novos do nível, nem mais nem menos.');
    expect(describeLevelUpFailure(refused(LevelUpRefusalReason.ABILITY_ABOVE_20))).toMatchObject({ step: 'abilities', message: 'Nenhum atributo passa de 20. Escolha outro.' });
    expect(describeLevelUpFailure(refused(LevelUpRefusalReason.HIT_POINT_ROLL_MISSING))).toMatchObject({ step: 'hp' });
    expect(describeLevelUpFailure(refused(LevelUpRefusalReason.SUBCLASS))).toMatchObject({ step: 'picks' });
  });

  it('gives every reason words, and the ones no step owns no step', () => {
    for (const reason of Object.values(LevelUpRefusalReason).filter((v): v is LevelUpRefusalReason => typeof v === 'number' && v > 0)) {
      expect(refusalMessage({ reason }).length).toBeGreaterThan(10);
    }
    expect(refusalStep(LevelUpRefusalReason.SHEET_NEEDS_MASTER)).toBeNull();
    expect(refusalMessage({ reason: LevelUpRefusalReason.SHEET_NEEDS_MASTER })).toContain('Peça ao mestre');
  });

  it('reads why the character cannot level up from the CharacterBlocked detail', () => {
    const blocked = new ConnectError('x', Code.FailedPrecondition, undefined, [
      { desc: CharacterBlockedSchema, value: create(CharacterBlockedSchema, { reason: CharacterBlockedReason.CANNOT_LEVEL_UP }) },
    ]);
    const f = describeLevelUpFailure(blocked);
    expect(f).toMatchObject({ kind: 'blocked', reason: CharacterBlockedReason.CANNOT_LEVEL_UP });
    expect(f.message).toContain('não pode subir de nível');
  });

  it('maps the plain codes', () => {
    expect(describeLevelUpFailure(new ConnectError('x', Code.PermissionDenied)).message).toContain('dono do personagem');
    expect(describeLevelUpFailure(new ConnectError('x', Code.NotFound)).message).toContain('não existe');
    expect(describeLevelUpFailure(new Error('network')).kind).toBe('other');
  });
});
