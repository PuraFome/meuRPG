import { Code, ConnectError } from '@connectrpc/connect';

import { XpMode } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  XPBlockedReason,
  XPBlockedSchema,
} from '../../../gen/meurpg/progression/v1/progression_pb';
import {
  milestoneHasHistoryText,
  townErrorMessage,
  xpAborted,
  xpBlocked,
  xpBlockedMessage,
  xpErrorMessage,
} from './xp-errors';

/** A `failed_precondition` the way the server sends it: the typed `XPBlocked`. */
function blocked(reason: XPBlockedReason, xpMode = XpMode.UNSPECIFIED): ConnectError {
  return new ConnectError('x', Code.FailedPrecondition, undefined, [
    { desc: XPBlockedSchema, value: { reason, xpMode } },
  ]);
}

describe('XPBlocked: Portuguese for every reason', () => {
  const said = (reason: XPBlockedReason, xpMode = XpMode.UNSPECIFIED) =>
    xpBlockedMessage({ reason, xpMode } as never);

  it('says what the campaign takes when the mode does not fit', () => {
    expect(said(XPBlockedReason.XP_BLOCKED_REASON_MODE_NOT_ALLOWED, XpMode.MILESTONES)).toContain(
      'Registrar marco',
    );
    expect(said(XPBlockedReason.XP_BLOCKED_REASON_MODE_NOT_ALLOWED, XpMode.GOLD)).toContain('ouro');
    expect(said(XPBlockedReason.XP_BLOCKED_REASON_MODE_NOT_ALLOWED, XpMode.ENEMIES)).toContain(
      'inimigos',
    );
    expect(said(XPBlockedReason.XP_BLOCKED_REASON_MODE_NOT_ALLOWED)).toMatch(/não aceita/);
  });

  it('says a milestone reached once and undone cannot be removed', () => {
    const msg = said(XPBlockedReason.XP_BLOCKED_REASON_MILESTONE_HAS_HISTORY);
    expect(msg).toContain('não pode ser removido');
    expect(msg).not.toMatch(/não pode ser feito agora/);
  });

  it('says why a combat cannot be paid', () => {
    expect(said(XPBlockedReason.XP_BLOCKED_REASON_ENCOUNTER_NOT_ENDED)).toMatch(
      /ainda não terminou/,
    );
    expect(said(XPBlockedReason.XP_BLOCKED_REASON_ALREADY_AWARDED)).toMatch(/já foi dado/);
  });

  it('says why there is nothing to give, to undo, or nobody to give to', () => {
    expect(said(XPBlockedReason.XP_BLOCKED_REASON_NOTHING_TO_GIVE)).toMatch(/pelo menos 1 XP/);
    expect(said(XPBlockedReason.XP_BLOCKED_REASON_NOTHING_TO_UNDO)).toMatch(
      /nenhum prêmio para desfazer/,
    );
    expect(said(XPBlockedReason.XP_BLOCKED_REASON_CHARACTER_NOT_ELIGIBLE)).toMatch(
      /morreu ou saiu/,
    );
  });

  it('never leaves a reason, even an unknown one, without a sentence', () => {
    for (const reason of [
      XPBlockedReason.XP_BLOCKED_REASON_UNSPECIFIED,
      XPBlockedReason.XP_BLOCKED_REASON_MODE_NOT_ALLOWED,
      XPBlockedReason.XP_BLOCKED_REASON_ENCOUNTER_NOT_ENDED,
      XPBlockedReason.XP_BLOCKED_REASON_ALREADY_AWARDED,
      XPBlockedReason.XP_BLOCKED_REASON_NOTHING_TO_GIVE,
      XPBlockedReason.XP_BLOCKED_REASON_NOTHING_TO_UNDO,
      XPBlockedReason.XP_BLOCKED_REASON_CHARACTER_NOT_ELIGIBLE,
    ]) {
      expect(said(reason).length).toBeGreaterThan(20);
    }
  });
});

describe('xpErrorMessage', () => {
  it('reads the typed detail, never the message', () => {
    const err = blocked(XPBlockedReason.XP_BLOCKED_REASON_ALREADY_AWARDED);
    expect(xpBlocked(err)?.reason).toBe(XPBlockedReason.XP_BLOCKED_REASON_ALREADY_AWARDED);
    expect(xpErrorMessage(err)).toMatch(/já foi dado/);
    // A message that looks like a reason means nothing without the detail.
    expect(xpBlocked(new ConnectError('ALREADY_AWARDED', Code.FailedPrecondition))).toBeNull();
    expect(xpErrorMessage(new ConnectError('ALREADY_AWARDED', Code.FailedPrecondition))).toMatch(
      /servidor/,
    );
  });

  it('speaks by code when there is no detail', () => {
    expect(xpErrorMessage(new ConnectError('x', Code.InvalidArgument), 'dar o XP')).toMatch(
      /Não deu para dar o XP/,
    );
    expect(xpErrorMessage(new ConnectError('x', Code.NotFound))).toMatch(/não existe mais/);
    expect(xpErrorMessage(new ConnectError('x', Code.PermissionDenied))).toMatch(/Só o mestre/);
    expect(xpErrorMessage(new ConnectError('x', Code.Aborted))).toMatch(/mudou/);
    expect(xpErrorMessage(new Error('Failed to fetch'), 'dar o XP')).toMatch(
      /servidor não respondeu/,
    );
  });

  it('knows an aborted call (the last award is not the one on screen)', () => {
    expect(xpAborted(new ConnectError('x', Code.Aborted))).toBe(true);
    expect(xpAborted(new ConnectError('x', Code.NotFound))).toBe(false);
    expect(xpAborted(new Error('x'))).toBe(false);
  });
});

describe('XPBlocked for "Voltar à cidade" (E9-09)', () => {
  const said = (reason: XPBlockedReason) =>
    xpBlockedMessage({ reason, xpMode: XpMode.GOLD } as never);

  it('says why a treasure cannot be converted, and that the list was updated', () => {
    expect(said(XPBlockedReason.XP_BLOCKED_REASON_TREASURE_NOT_FOUND_YET)).toMatch(
      /não está mais como encontrado.*lista foi atualizada/,
    );
    expect(said(XPBlockedReason.XP_BLOCKED_REASON_TREASURE_ALREADY_CONVERTED)).toMatch(
      /já virou XP.*lista foi atualizada/,
    );
  });

  it('says the limit in PO and what to do', () => {
    expect(said(XPBlockedReason.XP_BLOCKED_REASON_TREASURES_OVER_LIMIT)).toMatch(
      /1\.000\.000 PO.*Desmarque alguns/,
    );
  });

  it("says in the treasures' words that a campaign by enemies or milestones does not convert them", () => {
    const enemies = townErrorMessage(
      blocked(XPBlockedReason.XP_BLOCKED_REASON_MODE_NOT_ALLOWED, XpMode.ENEMIES),
    );
    expect(enemies).toBe(
      'Esta campanha dá XP por inimigos, então o tesouro não vira XP. Ele continua aparecendo no resumo de cada sessão.',
    );
    expect(
      townErrorMessage(
        blocked(XPBlockedReason.XP_BLOCKED_REASON_MODE_NOT_ALLOWED, XpMode.MILESTONES),
      ),
    ).toMatch(/marcos, não XP/);
  });

  it('keeps every other error as for any award', () => {
    expect(
      townErrorMessage(blocked(XPBlockedReason.XP_BLOCKED_REASON_CHARACTER_NOT_ELIGIBLE)),
    ).toMatch(/morreu ou saiu/);
    expect(townErrorMessage(new ConnectError('x', Code.PermissionDenied))).toBe(
      'Só o mestre da campanha pode fazer isso.',
    );
  });
});

describe('XPBlocked, every reason in turn', () => {
  const REASONS: readonly (readonly [XPBlockedReason, string])[] = [
    [
      XPBlockedReason.XP_BLOCKED_REASON_ENCOUNTER_NOT_ENDED,
      'O combate ainda não terminou. Encerre o combate para dar o XP dele.',
    ],
    [
      XPBlockedReason.XP_BLOCKED_REASON_ALREADY_AWARDED,
      'O XP desse combate já foi dado. Para dar de novo, desfaça esse prêmio em "Experiência", na página da campanha.',
    ],
    [
      XPBlockedReason.XP_BLOCKED_REASON_NOTHING_TO_GIVE,
      'Não há XP para dar: o total é pequeno demais para cada um receber pelo menos 1 XP.',
    ],
    [
      XPBlockedReason.XP_BLOCKED_REASON_NOTHING_TO_UNDO,
      'Não há nenhum prêmio para desfazer. A tela foi atualizada.',
    ],
    [
      XPBlockedReason.XP_BLOCKED_REASON_CHARACTER_NOT_ELIGIBLE,
      'Um dos personagens marcados não pode receber: ele morreu ou saiu da campanha. A lista foi atualizada.',
    ],
    [
      XPBlockedReason.XP_BLOCKED_REASON_MILESTONE_ALREADY_REACHED,
      'Esse marco já foi alcançado. A lista foi atualizada.',
    ],
    [
      XPBlockedReason.XP_BLOCKED_REASON_MILESTONE_NOT_REACHED,
      'Esse marco não está mais alcançado: ele foi desfeito. A lista foi atualizada.',
    ],
    [
      XPBlockedReason.XP_BLOCKED_REASON_CHARACTER_ALREADY_MARKED,
      'Um dos personagens marcados já tem esse marco. A lista foi atualizada.',
    ],
    [
      XPBlockedReason.XP_BLOCKED_REASON_TREASURE_NOT_FOUND_YET,
      'Um dos tesouros marcados não está mais como encontrado: o mestre desmarcou o achado. A lista foi atualizada; confira e tente de novo.',
    ],
    [
      XPBlockedReason.XP_BLOCKED_REASON_TREASURE_ALREADY_CONVERTED,
      'Um dos tesouros marcados já virou XP em outro prêmio. A lista foi atualizada; confira e tente de novo.',
    ],
    [
      XPBlockedReason.XP_BLOCKED_REASON_TREASURES_OVER_LIMIT,
      'Os tesouros marcados valem mais de 1.000.000 PO juntos, o máximo de um prêmio. Desmarque alguns e converta o resto depois.',
    ],
    [
      XPBlockedReason.XP_BLOCKED_REASON_MILESTONE_HAS_HISTORY,
      milestoneHasHistoryText('Esse marco'),
    ],
  ];
  it.each(REASONS)('says reason %i in words', (reason, message) => {
    expect(xpBlockedMessage({ reason, xpMode: XpMode.UNSPECIFIED } as never)).toBe(message);
  });

  it('has the words of every reason the server can name', () => {
    const named: XPBlockedReason[] = REASONS.map(([reason]) => reason);
    named.push(XPBlockedReason.XP_BLOCKED_REASON_MODE_NOT_ALLOWED);
    const all = Object.values(XPBlockedReason).filter(
      (v): v is XPBlockedReason =>
        typeof v === 'number' && v !== XPBlockedReason.XP_BLOCKED_REASON_UNSPECIFIED,
    );
    expect(all.filter((v) => !named.includes(v))).toEqual([]);
  });

  it('says what the campaign takes, per mode, and a general sentence for another mode', () => {
    const mode = (xpMode: XpMode) =>
      xpBlockedMessage({
        reason: XPBlockedReason.XP_BLOCKED_REASON_MODE_NOT_ALLOWED,
        xpMode,
      } as never);
    expect(mode(XpMode.MILESTONES)).toBe(
      'Esta campanha conta marcos, não XP. Use "Registrar marco".',
    );
    expect(mode(XpMode.GOLD)).toBe(
      'Esta campanha conta XP por ouro: dê o ouro encontrado ou um valor avulso.',
    );
    expect(mode(XpMode.ENEMIES)).toBe(
      'Esta campanha conta XP por inimigos derrotados: dê o XP de um combate ou um valor avulso.',
    );
    expect(mode(XpMode.UNSPECIFIED)).toBe('Esta campanha não aceita essa forma de dar XP.');
  });

  it('falls back to a general sentence for a reason it does not know', () => {
    const fallback = 'Isso não pode ser feito agora. A tela foi atualizada.';
    expect(
      xpBlockedMessage({ reason: XPBlockedReason.XP_BLOCKED_REASON_UNSPECIFIED } as never),
    ).toBe(fallback);
    expect(xpBlockedMessage({ reason: 9999 } as never)).toBe(fallback);
  });
});
