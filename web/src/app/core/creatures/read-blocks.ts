import type { Creature } from '../../../gen/meurpg/rules/v1/rules_pb';
import type { CreaturesClient } from './creatures-client';

/**
 * Reads the stat blocks of a list of creatures, six at a time, calling `got` for each as it arrives: a row shows the
 * book's summary first and its numbers once they are in. A block that cannot be read is skipped (its row keeps the
 * summary line). `current` tells it to stop when the list it was reading for is no longer the one on screen.
 */
export async function readBlocks(
  client: Pick<CreaturesClient, 'statBlock'>,
  campaignId: string,
  keys: readonly string[],
  got: (key: string, block: Creature) => void,
  current: () => boolean = () => true,
): Promise<void> {
  let next = 0;
  const worker = async () => {
    while (next < keys.length && current()) {
      const key = keys[next++];
      try {
        got(key, await client.statBlock(campaignId, key));
      } catch {
        // Its row keeps the summary line.
      }
    }
  };
  await Promise.all(Array.from({ length: 6 }, worker));
}
