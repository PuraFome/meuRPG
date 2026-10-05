import { TOAST_MS, ToastQueue } from './toast-queue';

describe('ToastQueue', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it('goes away by itself after 8 seconds', () => {
    const q = new ToastQueue();
    q.push('visibility', 'Você notou uma armadilha.', 'Fosso escondido, no mapa.');
    expect(q.toasts()).toHaveLength(1);
    vi.advanceTimersByTime(TOAST_MS - 1);
    expect(q.toasts()).toHaveLength(1);
    vi.advanceTimersByTime(1);
    expect(q.toasts()).toHaveLength(0);
  });

  it('is dismissed at once by the close button, and clear stops the timers', () => {
    const q = new ToastQueue();
    q.push('a', 'um');
    q.push('b', 'dois');
    q.dismiss(q.toasts()[0].id);
    expect(q.toasts().map((t) => t.title)).toEqual(['dois']);
    q.clear();
    expect(q.toasts()).toEqual([]);
    vi.advanceTimersByTime(TOAST_MS * 2);
    expect(q.toasts()).toEqual([]);
  });
});
