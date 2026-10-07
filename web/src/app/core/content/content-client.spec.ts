import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import {
  TableContentBlockedReason,
  TableContentBlockedSchema,
  TableContentRefusalSchema,
  TableContentViolationSchema,
} from '../../../gen/meurpg/rules/v1/table_content_pb';
import { blockedReason, contentErrorText, isStale, refusalOf } from './content-client';

function blocked(code: Code, reason: TableContentBlockedReason) {
  return new ConnectError('x', code, undefined, [
    {
      desc: TableContentBlockedSchema,
      value: create(TableContentBlockedSchema, { reason, key: 'race:corujeiro@mesa' }),
    },
  ]);
}

describe('the errors of the table content calls', () => {
  it('reads a refusal by its typed detail', () => {
    const err = new ConnectError('refused', Code.InvalidArgument, undefined, [
      {
        desc: TableContentRefusalSchema,
        value: create(TableContentRefusalSchema, {
          violations: [
            create(TableContentViolationSchema, {
              field: 'table_spell.name_pt',
              reason: 'duplicate_name',
            }),
          ],
        }),
      },
    ]);
    expect(refusalOf(err)?.map((v) => v.field)).toEqual(['table_spell.name_pt']);
    expect(refusalOf(new ConnectError('x', Code.NotFound))).toBeNull();
    expect(refusalOf(new Error('offline'))).toBeNull();
  });

  it('says "Esta entrada mudou enquanto você editava" for a stale write, by the detail and not by the message', () => {
    const err = blocked(Code.Aborted, TableContentBlockedReason.STALE);
    expect(isStale(err)).toBe(true);
    expect(blockedReason(err)).toBe(TableContentBlockedReason.STALE);
    expect(contentErrorText(err, 'salvar a magia')).toContain(
      'Esta entrada mudou enquanto você editava',
    );
    expect(isStale(new ConnectError('stale entry', Code.Aborted))).toBe(false);
  });

  it('says what archiving twice and unarchiving what is not archived mean', () => {
    expect(
      contentErrorText(
        blocked(Code.FailedPrecondition, TableContentBlockedReason.ARCHIVED),
        'arquivar',
      ),
    ).toBe('Esta entrada já está arquivada.');
    expect(
      contentErrorText(
        blocked(Code.FailedPrecondition, TableContentBlockedReason.NOT_ARCHIVED),
        'desarquivar',
      ),
    ).toBe('Esta entrada não está arquivada.');
  });

  it('says the rest in words: no permission, gone, offline', () => {
    expect(contentErrorText(new ConnectError('x', Code.PermissionDenied), 'salvar')).toContain(
      'Só o mestre',
    );
    expect(contentErrorText(new ConnectError('x', Code.NotFound), 'salvar')).toContain(
      'não existe mais',
    );
    expect(contentErrorText(new Error('network'), 'salvar a raça')).toBe(
      'Não foi possível salvar a raça. Confira a conexão e tente de novo.',
    );
  });
});
