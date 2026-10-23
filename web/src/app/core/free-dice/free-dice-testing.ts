import type { FreeRollVm } from './free-roll';

const EVENING = new Date('2026-10-18T20:00:00').getTime();
const MINUTE_MS = 60_000;

/** A table roll of Brisa's, 2d6 + 3 = 11, for specs; `at` is minutes into the evening. */
export function freeRoll(id: string, over: Partial<FreeRollVm> = {}, at = 0): FreeRollVm {
  return {
    id,
    rollerName: 'Brisa',
    byMaster: false,
    secret: false,
    sides: 6,
    count: 2,
    modifier: 3,
    mode: 'normal',
    dice: [
      { face: 4, discarded: false },
      { face: 4, discarded: false },
    ],
    total: 11,
    formula: '2d6 + 3',
    logText: 'Rolagem livre · Brisa: 2d6 + 3 = 11 (4, 4)',
    createdAt: new Date(EVENING + at * MINUTE_MS),
    ...over,
  };
}

/** The client's answers for specs; each call is recorded and answers what the spec put in `lists` or `answer`. */
export class FakeFreeDiceClient {
  lists: { table: FreeRollVm[]; mine: FreeRollVm[] } = { table: [], mine: [] };
  listError: unknown = null;
  rollError: unknown = null;
  /** What the next roll answers; a spec that wants to hold the answer sets `hold`. */
  answer: FreeRollVm = freeRoll('answer');
  hold: Promise<void> | null = null;
  readonly rolls: {
    campaignId: string;
    sides: number;
    count: number;
    modifier: number;
    mode: string;
    secret: boolean;
    idempotencyKey: string;
  }[] = [];
  listCalls = 0;

  list = (): Promise<{ table: FreeRollVm[]; mine: FreeRollVm[] }> => {
    this.listCalls++;
    return this.listError ? Promise.reject(this.listError) : Promise.resolve(this.lists);
  };

  roll = async (request: (typeof this.rolls)[number]): Promise<FreeRollVm> => {
    this.rolls.push(request);
    await this.hold;
    if (this.rollError) {
      throw this.rollError;
    }
    return this.answer;
  };
}
