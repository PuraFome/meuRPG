import { signal } from '@angular/core';
import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import { SpellCastSchema } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { encounter } from '../../../../core/combat/combat-testing';
import { HeldRollAsked } from '../../../../core/held-rolls/held-roll-offer';
import { heldAnswer, heldDie, heldLucky } from '../../../../core/held-rolls/held-roll-testing';
import { CastQuestions } from './cast-questions';

const lucky = heldLucky();
const die = heldDie(9, 5, 8, { holdId: 'h2' });
const enc = encounter({});

describe('CastQuestions', () => {
  let resources: { answerHeldRoll: ReturnType<typeof vi.fn> };
  let shown: unknown[];
  let played: unknown[];
  let applied: unknown[];
  const busy = signal(false);
  const error = signal('');
  const typing = signal(true);

  function questions() {
    return new CastQuestions({
      resources: () => resources as never,
      campaignId: 'c',
      name: 'Raio',
      state: { apply: (e) => applied.push(e) },
      animator: { play: (s) => played.push(s) },
      busy,
      error,
      typing,
      showCast: (e, c) => shown.push([e, c]),
    });
  }

  beforeEach(() => {
    resources = { answerHeldRoll: vi.fn() };
    shown = [];
    played = [];
    applied = [];
    busy.set(false);
    error.set('');
    typing.set(true);
  });

  describe('a cast that answered with a question', () => {
    it('shows the question, applies the combat, stops typing and draws the held d20', () => {
      const q = questions();
      expect(q.asks(new HeldRollAsked(lucky, enc))).toBe(true);
      expect(q.held.offer()).toBe(lucky);
      expect(applied).toEqual([enc]);
      expect(typing()).toBe(false);
      expect(played.length).toBe(1);
    });

    it('draws nothing when the held d20 cannot be drawn', () => {
      const q = questions();
      const bare = heldDie(9, 5, 8, { holdId: 'h' });
      bare.d20 = undefined;
      expect(q.asks(new HeldRollAsked(bare, enc))).toBe(true);
      expect(q.held.offer()?.holdId).toBe('h');
      expect(played).toEqual([]);
    });

    it('is not one for any other error', () => {
      expect(questions().asks(new Error('x'))).toBe(false);
      expect(applied).toEqual([]);
    });
  });

  describe('the answer', () => {
    const cast = create(SpellCastSchema, {});
    const reroll = { kind: 'lucky', reroll: true, roll: { face: 14 } } as const;

    it('does nothing without the question, or while busy', async () => {
      const q = questions();
      await q.held.answer(reroll);
      q.held.ask(lucky);
      busy.set(true);
      await q.held.answer(reroll);
      expect(resources.answerHeldRoll).not.toHaveBeenCalled();
    });

    it('shows the cast it brings, and renews the key', async () => {
      resources.answerHeldRoll.mockResolvedValue(
        heldAnswer({ result: { case: 'spellCast', value: { encounter: enc, cast } } }),
      );
      const q = questions();
      q.held.ask(lucky);
      await q.held.answer(reroll);
      expect(resources.answerHeldRoll).toHaveBeenCalledWith('c', 'h1', reroll, expect.any(String));
      expect(shown).toEqual([[enc, cast]]);
      expect(q.held.offer()).toBeNull();
      expect(typing()).toBe(false);
      q.held.ask(lucky);
      await q.held.answer(reroll);
      const keys = resources.answerHeldRoll.mock.calls.map((c) => c[3]);
      expect(keys[1]).not.toBe(keys[0]);
    });

    it('goes on to the die’s question on the new roll, tumbling the d20 the app rolled', async () => {
      const again = heldDie(14, 5, 8, { holdId: 'h3' }, 1);
      resources.answerHeldRoll.mockResolvedValue(
        heldAnswer({
          result: { case: 'spellCast', value: { encounter: enc, heldRollOffer: again } },
          next: again,
        }),
      );
      const q = questions();
      q.held.ask(lucky);
      await q.held.answer({ kind: 'lucky', reroll: true, roll: { inApp: true } });
      expect(q.held.offer()).toBe(again);
      expect(applied).toEqual([enc]);
      expect(played.length).toBe(1);
      expect(shown).toEqual([]);
    });

    it('says when the answer brings no cast, or fails, and frees the sheet', async () => {
      resources.answerHeldRoll
        .mockResolvedValueOnce(heldAnswer())
        .mockResolvedValueOnce(
          heldAnswer({ result: { case: 'spellCast', value: { encounter: enc } } }),
        )
        .mockRejectedValueOnce(new ConnectError('x', Code.Unavailable));
      const q = questions();
      for (let i = 0; i < 3; i++) {
        q.held.ask(lucky);
        error.set('');
        await q.held.answer({ kind: 'lucky', reroll: false, roll: null });
        expect(error()).not.toBe('');
        expect(busy()).toBe(false);
      }
    });

    it('the die’s answer shows the cast, and says when it brings none', async () => {
      resources.answerHeldRoll
        .mockResolvedValueOnce(
          heldAnswer({ result: { case: 'spellCast', value: { encounter: enc, cast } } }),
        )
        .mockResolvedValueOnce(heldAnswer({ result: { case: 'spellCast', value: {} } }));
      const q = questions();
      q.held.ask(die);
      await q.held.answer({ kind: 'die', use: true, roll: { inApp: true } });
      expect(shown).toEqual([[enc, cast]]);
      expect(q.held.offer()).toBeNull();
      q.held.ask(die);
      await q.held.answer({ kind: 'die', use: false, roll: null });
      expect(error()).not.toBe('');
      expect(q.held.offer()).toBe(die);
    });
  });
});
