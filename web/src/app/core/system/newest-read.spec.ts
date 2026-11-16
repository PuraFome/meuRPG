import { NewestRead } from './newest-read';

describe('NewestRead', () => {
  const deferred = () => {
    let done!: (v: boolean) => void;
    const promise = new Promise<boolean>((resolve) => (done = resolve));
    return { promise, done };
  };

  it('answers an overtaken read with the newer read, once it has landed', async () => {
    const reads = new NewestRead();
    const older = deferred();
    const newer = deferred();
    const first = reads.start((seq) => older.promise.then((v) => v || reads.overtaken(seq)));
    const second = reads.start(() => newer.promise);
    let answered = false;
    void first.then(() => (answered = true));
    older.done(false); // the older read lands first and finds it was overtaken
    await Promise.resolve();
    await Promise.resolve();
    expect(answered).toBe(false);
    newer.done(true);
    expect(await second).toBe(true);
    expect(await first).toBe(true);
  });

  it('answers false when no read started after it, as when a page load took over', async () => {
    const reads = new NewestRead();
    const seq = reads.next();
    expect(reads.holds(seq)).toBe(true);
    reads.next(); // a load takes the screen
    expect(reads.holds(seq)).toBe(false);
    expect(await reads.overtaken(seq)).toBe(false);
  });

  it('follows the newest of three', async () => {
    const reads = new NewestRead();
    const a = reads.start((seq) => Promise.resolve(false).then(() => reads.overtaken(seq)));
    const b = reads.start((seq) => Promise.resolve(false).then(() => reads.overtaken(seq)));
    const c = reads.start(() => Promise.resolve(true));
    expect(await Promise.all([a, b, c])).toEqual([true, true, true]);
  });
});
