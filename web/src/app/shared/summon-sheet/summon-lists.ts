import { signal } from '@angular/core';

import type { SummonOption } from '../../../gen/meurpg/characters/v1/characters_pb';
import type { Creature, CreatureSummary } from '../../../gen/meurpg/rules/v1/rules_pb';
import { creatureErrorMessage } from '../../core/creatures/creature-errors';
import type { CreaturesClient } from '../../core/creatures/creatures-client';
import { readBlocks } from '../../core/creatures/read-blocks';

/**
 * The creatures an option of the summon sheet offers, read from the book: the forms of a familiar (their
 * numbers), or the beasts a search finds (their numbers as they come). A plain class the sheet builds: an
 * answer that arrives after another option was picked is for a list nobody looks at, so each read numbers
 * itself and the older ones are dropped.
 */
export class SummonLists {
  readonly forms = signal<readonly Creature[] | null>(null);
  readonly beasts = signal<readonly CreatureSummary[] | null>(null);
  /** The book's numbers of each beast of the list (CA, PV, speed, attacks), as they are read. */
  readonly beastBlocks = signal<ReadonlyMap<string, Creature>>(new Map());
  /** The book's armor class of each creature kind of the result (the combat sends none to a player). */
  readonly armorClasses = signal<ReadonlyMap<string, number>>(new Map());
  private beastSeq = 0;
  private formSeq = 0;

  constructor(
    private readonly client: CreaturesClient,
    private readonly campaignId: string,
    /** A read that failed says so in the sheet's own words. */
    private readonly fail: (message: string) => void,
  ) {}

  /** The option brings the forms it lists, or the beasts its type and rating allow. */
  load(o: SummonOption): void {
    if (o.forms.length > 0) {
      void this.loadForms(o);
    } else {
      void this.loadBeasts(o);
    }
  }

  /** The book's armor class of each kind, for the lines of the result (read once, in memory after). */
  readArmorClasses(monsterKeys: readonly string[]): void {
    for (const key of new Set(monsterKeys)) {
      void this.client.statBlock(this.campaignId, key).then(
        (block) => this.armorClasses.update((m) => new Map(m).set(key, block.armorClass)),
        () => undefined,
      );
    }
  }

  private async loadForms(o: SummonOption): Promise<void> {
    const seq = ++this.formSeq;
    // A form whose numbers cannot be read still shows its name; only the line under it is missing.
    const read = await Promise.allSettled(
      o.forms.map((f) => this.client.statBlock(this.campaignId, f.monsterKey)),
    );
    if (seq !== this.formSeq) {
      return;
    }
    this.forms.set(read.flatMap((r) => (r.status === 'fulfilled' ? [r.value] : [])));
  }

  private async loadBeasts(o: SummonOption): Promise<void> {
    const seq = ++this.beastSeq;
    this.beasts.set(null);
    try {
      const found = (await this.client.search(this.campaignId, { type: o.type, maxCr: o.maxCr }))
        .creatures;
      if (seq === this.beastSeq) {
        this.beasts.set(found);
        // The numbers of each beast, as they come: a row shows the book's summary first.
        void readBlocks(
          this.client,
          this.campaignId,
          found.map((b) => b.key),
          (key, block) => this.beastBlocks.update((m) => new Map(m).set(key, block)),
          () => seq === this.beastSeq,
        );
      }
    } catch (err) {
      if (seq === this.beastSeq) {
        this.beasts.set([]);
        this.fail(creatureErrorMessage(err, 'read'));
      }
    }
  }
}
