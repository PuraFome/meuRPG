import { create } from '@bufbuild/protobuf';

import { PuzzleKind, PuzzleLastMoveSchema, PuzzleSolveOutcome } from '../../../gen/meurpg/play/v1/puzzles_pb';
import {
  agoText,
  kindName,
  lastMoveText,
  lightLabel,
  litCount,
  litWords,
  moveWord,
  outcomeText,
  pillarsChangedText,
  puzzleSummary,
} from './puzzle-format';
import { NOW, at, lightsPuzzle, lockPuzzle, pillarsPuzzle } from './puzzles-testing';

const plain = (text: string) => text.replace(/\u00a0/g, ' ');

describe('puzzle words', () => {
  it('names each kind and writes the list row\'s second line (E10-06 state 1)', () => {
    expect(kindName(PuzzleKind.LOCK)).toBe('Fechadura de combinação');
    expect(plain(puzzleSummary(lightsPuzzle('a', 'O selo da Capela')))).toBe('Apagar as luzes · 5 × 5');
    expect(plain(puzzleSummary(lockPuzzle('b', 'O cofre do Refeitório')))).toBe('Fechadura de combinação · 4 rodas de runas');
    expect(plain(puzzleSummary(pillarsPuzzle('c', 'Os pilares da Galeria')))).toBe('Símbolos giratórios · 4 pilares, girando juntos');
  });

  it('names a light by its row, its column and its state (never colour alone)', () => {
    expect(lightLabel(1, 2, true)).toBe('Luz na linha 2, coluna 3, acesa');
    expect(lightLabel(0, 0, false)).toBe('Luz na linha 1, coluna 1, apagada');
  });

  it('counts what the server sent and writes it', () => {
    const start = lightsPuzzle('a', 'x').start;
    expect(litCount(start)).toBe(9);
    expect(litWords(1)).toBe('1 acesa');
    expect(litWords(12)).toBe('12 acesas');
    expect(moveWord(PuzzleKind.LIGHTS, 1)).toBe('toque');
    expect(moveWord(PuzzleKind.PILLARS, 7)).toBe('giros');
  });

  it('says how long ago, from the two clocks', () => {
    expect(agoText(new Date(NOW.getTime() - 2000), NOW)).toBe('agora há pouco');
    expect(plain(agoText(new Date(NOW.getTime() - 12_000), NOW))).toBe('há 12 s');
    expect(plain(agoText(new Date(NOW.getTime() - 180_000), NOW))).toBe('há 3 min');
    expect(agoText(new Date(NOW.getTime() + 5000), NOW)).toBe('agora há pouco');
  });

  it('writes the last move of each kind, and "Você" for the reader\'s own character', () => {
    const lights = create(PuzzleLastMoveSchema, { characterName: 'Lia', move: { kind: { case: 'lights', value: { row: 1, col: 1 } } }, at: at(12) });
    const lock = create(PuzzleLastMoveSchema, { characterName: 'Toren', move: { kind: { case: 'lock', value: { wheel: 2, delta: 1 } } } });
    const pillars = create(PuzzleLastMoveSchema, { characterName: 'Lia', move: { kind: { case: 'pillars', value: { pillar: 0, delta: 1 } } } });
    expect(lastMoveText(lights)).toBe('Lia tocou numa luz');
    expect(lastMoveText(lock)).toBe('Toren girou a 3ª roda');
    expect(lastMoveText(pillars)).toBe('Lia girou o pilar 1');
    expect(lastMoveText(lights, 'Lia')).toBe('Você tocou numa luz');
  });

  it('lists the pillars a move turned', () => {
    expect(pillarsChangedText([])).toBe('');
    expect(pillarsChangedText([0])).toBe('O pilar 1 mudou.');
    expect(pillarsChangedText([0, 1])).toBe('Os pilares 1 e 2 mudaram.');
    expect(pillarsChangedText([0, 1, 3])).toBe('Os pilares 1, 2 e 4 mudaram.');
  });

  it('says what the server did when it was solved', () => {
    expect(outcomeText(PuzzleSolveOutcome.DOOR_OPENED)).toBe('Uma porta se abriu.');
    expect(outcomeText(PuzzleSolveOutcome.TARGET_GONE)).toContain('foi apagado');
    expect(outcomeText(PuzzleSolveOutcome.UNSPECIFIED)).toBe('');
  });
});
