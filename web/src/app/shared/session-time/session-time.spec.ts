import { formatClock, formatDayAt, sessionSince } from './session-time';

describe('session time', () => {
  it('writes the clock and the day with the clock', () => {
    const at = new Date(2026, 8, 30, 20, 5);
    expect(formatClock(at)).toBe('20:05');
    expect(formatDayAt(at).replace(/\u00a0/g, ' ')).toBe('30/09 às 20:05');
  });

  it('says only the clock for a session that started today (shared decision 8)', () => {
    const now = new Date(2026, 9, 3, 22, 40);
    expect(sessionSince(new Date(2026, 9, 3, 20, 5), now)).toBe('Em andamento desde 20:05');
  });

  it('adds the day when it started on another one', () => {
    const now = new Date(2026, 9, 3, 1, 10);
    expect(sessionSince(new Date(2026, 9, 2, 23, 30), now).replace(/\u00a0/g, ' ')).toBe(
      'Em andamento desde 02/10 às 23:30',
    );
    expect(sessionSince(new Date(2025, 9, 3, 20, 5), now).replace(/\u00a0/g, ' ')).toBe(
      'Em andamento desde 03/10 às 20:05',
    );
  });
});
