import { create } from '@bufbuild/protobuf';

import { AttackOutcome, PendingDamageStatus, SaveOutcome, TrapFiringSchema } from '../../../gen/meurpg/play/v1/combat_pb';
import { TrapActivitySchema, TrapSearchSkill } from '../../../gen/meurpg/play/v1/traps_pb';
import { activityLines, fallNote, trapLogText } from './trap-log';

const firing = create(TrapFiringSchema, {
  pointId: 'p',
  name: 'Fosso escondido',
  caught: [
    {
      targetId: 'c1',
      targetLabel: 'Toren',
      characterId: 'char-toren',
      saves: [{ outcome: SaveOutcome.FAILED }],
      damages: [{ amount: 7, damageTypePt: 'concussão', status: PendingDamageStatus.ROLLED }],
    },
    { targetId: 'c2', targetLabel: 'Goblin 1', attacks: [{ outcome: AttackOutcome.MISS }] },
  ],
});

describe('trap log', () => {
  it('writes the public line of a firing without any DC', () => {
    const text = trapLogText(firing, false);
    expect(text).toContain('A armadilha Fosso escondido foi disparada: Toren caiu');
    expect(text).toContain('7 de concussão, esperando o mestre aplicar');
    expect(text).toContain('falhou no teste');
    expect(text).not.toMatch(/CD/);
    expect(trapLogText(create(TrapFiringSchema, { name: 'X' }), false)).toBe('A armadilha X foi disparada. Ninguém estava na área.');
  });

  it('tells the player what happened to their own character and whether the damage waits', () => {
    const note = fallNote(firing, 'char-toren')!;
    expect(note.title).toBe('Você caiu na armadilha Fosso escondido.');
    expect(note.detail).toContain('7 de concussão');
    expect(note.detail).not.toContain('esperando');
    expect(note.waiting).toBe(true);
    expect(fallNote(firing, 'other')).toBeNull();
  });

  it('writes the search and notice lines of the activity', () => {
    const search = create(TrapActivitySchema, {
      id: 's',
      search: { characterName: 'Brisa', skill: TrapSearchSkill.INVESTIGATION, roll: { diceCount: 1, diceSides: 20, faces: [13], modifier: 4, total: 17 }, foundNames: ['Fosso escondido'] },
    });
    const [line] = activityLines(search, true);
    expect(line.actor).toBe('Brisa');
    expect(line.text).toContain('procurou armadilhas (Investigação)');
    expect(line.text).toContain('achou a armadilha Fosso escondido');
    const none = create(TrapActivitySchema, { id: 'n', search: { characterName: 'Pensantus', skill: TrapSearchSkill.PERCEPTION, roll: { diceCount: 1, diceSides: 20, faces: [6], total: 6 } } });
    expect(activityLines(none, false)[0].text).toContain('não achou nada');
    const notice = create(TrapActivitySchema, { id: 'q', notice: { characterNames: ['Sálvia'], trapName: 'Fosso escondido' } });
    expect(activityLines(notice, false)[0]).toMatchObject({ actor: 'Sálvia', text: ' notou a armadilha Fosso escondido ao passar' });
  });
});
