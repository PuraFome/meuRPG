import { describe, expect, it } from 'vitest';

import type { TrapSeverity } from '../../../../gen/meurpg/rules/v1/rules_pb';
import { alsoKeyMove, changeAt, severityLine } from './trap-edits';

describe('trap edits', () => {
  it('writes the severity table in one line, lowercased', () => {
    const rows = [
      { namePt: 'Revés', saveDcMin: 10, saveDcMax: 11 },
      { namePt: 'Mortal', saveDcMin: 16, saveDcMax: 20 },
    ] as TrapSeverity[];
    expect(severityLine(rows, (s) => `${s.saveDcMin} a ${s.saveDcMax}`)).toBe(
      'revés 10 a 11 · mortal 16 a 20',
    );
    expect(severityLine([], () => 'x')).toBe('');
  });

  it('changes the part at a place and keeps the others', () => {
    const list = [
      { a: 1, b: 1 },
      { a: 2, b: 2 },
    ];
    expect(changeAt(list, 1, { b: 9 })).toEqual([
      { a: 1, b: 1 },
      { a: 2, b: 9 },
    ]);
    expect(changeAt(list, 5, { b: 9 })).toEqual(list);
  });

  it('moves the cursor of the skill list inside its bounds, and names the other keys', () => {
    expect(alsoKeyMove('ArrowDown', 3, 3)).toBe(3);
    expect(alsoKeyMove('ArrowDown', 1, 3)).toBe(2);
    expect(alsoKeyMove('ArrowUp', 0, 3)).toBe(0);
    expect(alsoKeyMove('ArrowUp', 2, 3)).toBe(1);
    expect(alsoKeyMove('Home', 2, 3)).toBe(0);
    expect(alsoKeyMove('End', 0, 3)).toBe(3);
    expect(alsoKeyMove('End', 0, -1)).toBe(0);
    expect(alsoKeyMove(' ', 0, 3)).toBe('pick');
    expect(alsoKeyMove('Enter', 0, 3)).toBe('pick');
    expect(alsoKeyMove('Escape', 0, 3)).toBe('close');
    expect(alsoKeyMove('Tab', 0, 3)).toBeNull();
  });
});
