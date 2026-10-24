import { dayAndMonth, sessionStartText } from './campaign-detail.copy';

describe('the times of the session strip and of "Sessões anteriores" (boards C2-01, C6-04)', () => {
  const now = new Date(2026, 9, 24, 21, 0);

  it('says "20:12" for a session that started today', () => {
    expect(sessionStartText(new Date(2026, 9, 24, 20, 12), now)).toBe('20:12');
    expect(sessionStartText(new Date(2026, 9, 24, 0, 5), now)).toBe('00:05');
  });

  it('says the day too for one that started on another day', () => {
    // The spaces around "às" do not break.
    expect(sessionStartText(new Date(2026, 9, 23, 23, 30), now).replace(/\s/g, ' ')).toBe(
      '23/10 às 23:30',
    );
  });

  it('says the day and the month of an ended session, with two digits each', () => {
    expect(dayAndMonth(new Date(2026, 9, 18, 19, 5))).toBe('18/10');
    expect(dayAndMonth(new Date(2026, 0, 4, 19, 5))).toBe('04/01');
  });
});
