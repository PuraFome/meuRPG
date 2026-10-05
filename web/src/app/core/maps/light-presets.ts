import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import { ContentService, type LightPreset } from '../../../gen/meurpg/rules/v1/rules_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';
import { feetToMeters } from '../units';
import { tight } from '../format/text';

/** What a preset is called and how far it reaches, as the sheet and the master's selects say it. */
export interface LightOption {
  readonly key: string;
  readonly name: string;
  /** "6 m claro + 6 m de penumbra", or "6 m claro" when the light has no dim part. */
  readonly radii: string;
}

/** The light a character carries when it carries none. */
export const NO_LIGHT = '';

/** "6 m claro + 6 m de penumbra": the radii in meters as the table says them (RN-21). */
export function lightRadii(brightFt: number, dimFt: number): string {
  const m = (ft: number) => String(feetToMeters(ft)).replace('.', ',');
  return dimFt > 0
    ? tight(`${m(brightFt)} m claro + ${m(dimFt)} m de penumbra`)
    : tight(`${m(brightFt)} m claro`);
}

export function lightOption(p: LightPreset): LightOption {
  return { key: p.key, name: p.namePt, radii: lightRadii(p.brightFt, p.dimFt) };
}

/**
 * The SRD's light presets (`ListLightPresets`), read once for the campaign: the
 * list of the carried-light sheet and the master's selects (MR-036, E9-04). A
 * lazy-route service. A failed read is tried again the next time.
 */
@Injectable({ providedIn: 'root' })
export class LightPresets {
  private readonly client = createClient(ContentService, inject(CONNECT_TRANSPORT));
  private readonly byCampaign = new Map<string, Promise<readonly LightOption[]>>();

  list(campaignId: string): Promise<readonly LightOption[]> {
    let known = this.byCampaign.get(campaignId);
    if (!known) {
      known = this.client
        .listLightPresets({ campaignId })
        .then((res) => res.presets.map(lightOption))
        .catch((err) => {
          this.byCampaign.delete(campaignId);
          throw err;
        });
      this.byCampaign.set(campaignId, known);
    }
    return known;
  }
}
