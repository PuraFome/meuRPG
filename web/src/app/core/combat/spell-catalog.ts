import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import { ContentService, type SpellDetails } from '../../../gen/meurpg/rules/v1/rules_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';

/**
 * What the SRD says about a spell (`GetSpellDetails`), read when the cast sheet
 * opens: whether it rolls to hit, asks for a save or heals. The names of the
 * spells a combat shows come with the combat itself (`concentration_spell_name_pt`,
 * `ReactionPrompt.spell_name_pt`), so nothing here reads names. A lazy-route
 * service: only the combat imports it.
 */
@Injectable({ providedIn: 'root' })
export class SpellCatalog {
  private readonly client = createClient(ContentService, inject(CONNECT_TRANSPORT));
  private readonly detailsByKey = new Map<string, Promise<SpellDetails | null>>();

  /** What the SRD says about one spell (`GetSpellDetails`): the cast sheet
   * needs to know whether it rolls to hit, asks for a save or heals. One read
   * for each spell, kept; `null` when it could not be read (and then asked again
   * the next time). */
  details(campaignId: string, spellKey: string): Promise<SpellDetails | null> {
    const id = `${campaignId}/${spellKey}`;
    let known = this.detailsByKey.get(id);
    if (!known) {
      known = this.client
        .getSpellDetails({ campaignId, spellKey })
        .then((res) => res.spell ?? null)
        .catch(() => {
          this.detailsByKey.delete(id);
          return null;
        });
      this.detailsByKey.set(id, known);
    }
    return known;
  }
}
