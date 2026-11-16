/**
 * The reads of one thing that overtake each other (a screen that reads a character again each time something changes).
 * Every read, and whatever else takes the screen over (a first load), takes a number from `next()`; only the last number holds.
 * A read that a newer one overtook answers with the newer read's result, once it has landed, so whoever waits for "what the
 * screen shows is current" never gets an early yes (the old data) or a no (the newer read had it).
 */
export class NewestRead {
  private count = 0;
  private newest: { readonly seq: number; readonly run: Promise<boolean> } = {
    seq: 0,
    run: Promise.resolve(false),
  };

  /** A new number: whatever holds an older one is overtaken. */
  next(): number {
    return ++this.count;
  }

  /** Whether nothing took a number after `seq`. */
  holds(seq: number): boolean {
    return seq === this.count;
  }

  /** Starts a read with a new number and keeps it as the newest read. */
  start(read: (seq: number) => Promise<boolean>): Promise<boolean> {
    const seq = this.next();
    const run = read(seq);
    this.newest = { seq, run };
    return run;
  }

  /** The answer of the read numbered `seq` that found it was overtaken: the newest read's, or `false` when no read started after it. */
  overtaken(seq: number): Promise<boolean> {
    return this.newest.seq > seq ? this.newest.run : Promise.resolve(false);
  }
}
