import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import {
  ContentService,
  type ListTrapPresetsResponse,
} from '../../../gen/meurpg/rules/v1/rules_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';

/**
 * The SRD's eight sample traps in Portuguese, with the SRD's numbers, and the severity tables of the form's
 * hint (`ListTrapPresets`, MR-035), read once for the campaign: the editor's "Predefinições do SRD". A preset
 * only fills the form; the trap on the map keeps its own copy. A failed read is tried again the next time.
 */
@Injectable({ providedIn: 'root' })
export class TrapPresets {
  private readonly client = createClient(ContentService, inject(CONNECT_TRANSPORT));
  private readonly byCampaign = new Map<string, Promise<ListTrapPresetsResponse>>();

  list(campaignId: string): Promise<ListTrapPresetsResponse> {
    let known = this.byCampaign.get(campaignId);
    if (!known) {
      known = this.client.listTrapPresets({ campaignId }).catch((err) => {
        this.byCampaign.delete(campaignId);
        throw err;
      });
      this.byCampaign.set(campaignId, known);
    }
    return known;
  }
}
