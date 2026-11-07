import { toLiveEvent } from './live-event-map';

describe('toLiveEvent', () => {
  it('drops a table-screen change, which the session page does not show', () => {
    const event = { case: 'tableScreenChanged', value: {} } as Parameters<typeof toLiveEvent>[0];
    expect(toLiveEvent(event)).toBeNull();
  });

  it('turns an event this app does not know into a heartbeat', () => {
    const event = { case: 'fromTheFuture', value: {} } as unknown as Parameters<
      typeof toLiveEvent
    >[0];
    expect(toLiveEvent(event)).toEqual({ kind: 'heartbeat' });
  });
});
