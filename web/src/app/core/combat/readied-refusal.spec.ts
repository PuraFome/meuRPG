import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import { ReadiedActionSchema, ReadiedKind } from '../../../gen/meurpg/play/v1/combat_pb';
import { readiedRefusalText } from './readied-refusal';

const held = (kind: ReadiedKind) => create(ReadiedActionSchema, { id: 'r', kind });

describe('the refusal of the answer of a readied action (#582)', () => {
  it('tells a refused spell to check its targets and darts, which the answer does send', () => {
    const refused = new ConnectError(
      'every target of Magic Missile needs at least one dart',
      Code.InvalidArgument,
    );
    const text = readiedRefusalText(refused, held(ReadiedKind.SPELL), 'responder à reação');
    expect(text).toBe(
      'Não deu para soltar a magia com essa escolha. Confira os alvos e os dardos. Dá para deixar passar.',
    );
    // The answer carries the targets and the darts now: it must not say they are not sent.
    expect(text).not.toContain('ainda não envia');
    expect(text).not.toContain('ponto no mapa');
  });

  it('leaves any other refusal, and any other action, to the usual words', () => {
    const usual = (err: unknown, kind: ReadiedKind) =>
      readiedRefusalText(err, held(kind), 'responder à reação');
    expect(usual(new ConnectError('x', Code.InvalidArgument), ReadiedKind.ATTACK)).not.toContain(
      'dardos',
    );
    expect(usual(new ConnectError('x', Code.FailedPrecondition), ReadiedKind.SPELL)).not.toContain(
      'dardos',
    );
  });
});
