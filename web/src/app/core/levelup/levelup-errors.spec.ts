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
    {
      desc: LevelUpRefusalSchema,
      value: create(LevelUpRefusalSchema, { reason, field: 'full.cantrip_keys' }),
    },
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
    expect(describeLevelUpFailure(refused(LevelUpRefusalReason.ABILITY_ABOVE_20))).toMatchObject({
      step: 'abilities',
      message: 'Nenhuma habilidade passa de 20. Escolha outra.',
    });
    expect(
      describeLevelUpFailure(refused(LevelUpRefusalReason.HIT_POINT_ROLL_MISSING)),
    ).toMatchObject({ step: 'hp' });
    expect(describeLevelUpFailure(refused(LevelUpRefusalReason.SUBCLASS))).toMatchObject({
      step: 'picks',
    });
  });

  it('says a choice the master switched off is not available any more, and sends it to the step that holds it (RN-23)', () => {
    const off = (field: string) =>
      new ConnectError('x', Code.FailedPrecondition, undefined, [
        {
          desc: LevelUpRefusalSchema,
          value: create(LevelUpRefusalSchema, {
            reason: LevelUpRefusalReason.SWITCHED_OFF_CHOICE,
            field,
          }),
        },
      ]);
    const subclass = describeLevelUpFailure(off('full.subclass_key'));
    expect(subclass).toMatchObject({ kind: 'refusal', step: 'picks' });
    expect(subclass.message).toBe(
      'O mestre desligou uma das opções que você escolheu para os jogadores. Volte e escolha outra.',
    );
    expect(describeLevelUpFailure(off('full.cantrip_keys'))).toMatchObject({ step: 'spells' });
    expect(describeLevelUpFailure(off('full.known_spell_keys'))).toMatchObject({ step: 'spells' });
  });

  it('gives every reason words, and the ones no step owns no step', () => {
    for (const reason of Object.values(LevelUpRefusalReason).filter(
      (v): v is LevelUpRefusalReason => typeof v === 'number' && v > 0,
    )) {
      expect(refusalMessage({ reason }).length).toBeGreaterThan(10);
    }
    expect(refusalStep(LevelUpRefusalReason.SHEET_NEEDS_MASTER)).toBeNull();
    expect(refusalMessage({ reason: LevelUpRefusalReason.SHEET_NEEDS_MASTER })).toContain(
      'Peça ao mestre',
    );
  });

  it('reads why the character cannot level up from the CharacterBlocked detail', () => {
    const blocked = new ConnectError('x', Code.FailedPrecondition, undefined, [
      {
        desc: CharacterBlockedSchema,
        value: create(CharacterBlockedSchema, { reason: CharacterBlockedReason.CANNOT_LEVEL_UP }),
      },
    ]);
    const f = describeLevelUpFailure(blocked);
    expect(f).toMatchObject({ kind: 'blocked', reason: CharacterBlockedReason.CANNOT_LEVEL_UP });
    expect(f.message).toContain('não pode subir de nível');
  });

  it('maps the plain codes', () => {
    expect(describeLevelUpFailure(new ConnectError('x', Code.PermissionDenied)).message).toContain(
      'dono do personagem',
    );
    expect(describeLevelUpFailure(new ConnectError('x', Code.NotFound)).message).toContain(
      'não existe',
    );
    expect(describeLevelUpFailure(new Error('network')).kind).toBe('other');
  });
});

describe('an option the master retired (RN-23, 10.1d)', () => {
  const refusedAt = (reason: LevelUpRefusalReason, field: string) =>
    new ConnectError('x', Code.FailedPrecondition, undefined, [
      { desc: LevelUpRefusalSchema, value: create(LevelUpRefusalSchema, { reason, field }) },
    ]);

  it('says an archived choice and sends the player to the step of its field', () => {
    const f = describeLevelUpFailure(
      refusedAt(LevelUpRefusalReason.ARCHIVED_CHOICE, 'subclass_key'),
    );
    expect(f).toMatchObject({ kind: 'refusal', step: 'picks' });
    expect(f.message).toContain('O mestre arquivou uma das opções que você escolheu');
    expect(
      describeLevelUpFailure(
        refusedAt(LevelUpRefusalReason.ARCHIVED_CHOICE, 'full.known_spell_keys'),
      ),
    ).toMatchObject({ step: 'spells' });
  });

  it('says a switched-off choice too, never the generic line', () => {
    const f = describeLevelUpFailure(
      refusedAt(LevelUpRefusalReason.SWITCHED_OFF_CHOICE, 'full.cantrip_keys'),
    );
    expect(f.message).toContain('desligou uma das opções');
    expect(f.message).not.toContain('As regras não aceitaram');
    expect(refusalStep(LevelUpRefusalReason.SWITCHED_OFF_CHOICE)).toBeNull();
  });

  it('sends a level refused for a late choice to the Escolhas step, with its own words', () => {
    const f = describeLevelUpFailure(
      refusedAt(LevelUpRefusalReason.LATE_CHOICE_MISSING, 'full.feature_choice_keys'),
    );
    expect(f).toMatchObject({ kind: 'refusal', step: 'picks' });
    expect(f.message).toContain('escolhas que ficaram para trás');
  });
});
