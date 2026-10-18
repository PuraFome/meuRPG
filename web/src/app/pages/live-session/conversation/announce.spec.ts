import { line } from '../../../core/conversation/conversation-testing';
import { newTail } from './announce';

describe('newTail', () => {
  const a = line('a', { at: 1 });
  const b = line('b', { at: 2 });
  const c = line('c', { at: 3 });
  const older = line('o', { at: 0 });

  it('is the lines after the last one that was there', () => {
    expect(newTail([a, b], [a, b, c]).map((m) => m.id)).toEqual(['c']);
  });

  it('is nothing when the lines read are older ones put before', () => {
    expect(newTail([a, b], [older, a, b])).toEqual([]);
  });

  it('is nothing for the first lines read: that is history', () => {
    expect(newTail([], [a, b])).toEqual([]);
  });

  it('is nothing when nothing changed', () => {
    expect(newTail([a], [a])).toEqual([]);
  });

  it('counts lines that come in the middle by time as not new to the reader', () => {
    expect(newTail([a, c], [a, b, c])).toEqual([]);
  });
});
