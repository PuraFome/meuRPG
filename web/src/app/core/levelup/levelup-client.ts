import { Injectable, inject } from '@angular/core';
import type { MessageInitShape } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';

import { CampaignService, DicePreference, XpMode } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  CharacterService,
  type Character,
  type LevelUp,
  type LevelUpChoicesSchema,
  type LevelUpOptions,
  type PreviewLevelUpResponse,
  type RollLevelUpHitPointsResponse,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { ContentService, type Skill, type Spell } from '../../../gen/meurpg/rules/v1/rules_pb';
import { spellDetailsFromGen } from '../../shared/spell-details/spell-details-map';
import type { SpellDetailsVm } from '../../shared/spell-details/spell-details.types';
import { CONNECT_TRANSPORT } from '../connect/transport';

/** What the player chose so far, in the wire's own shape. */
export type LevelUpChoicesInit = MessageInitShape<typeof LevelUpChoicesSchema>;

/** The spells and skills of the campaign's rules, for the pickers. */
export interface LevelUpCatalog {
  readonly spells: readonly Spell[];
  readonly skills: readonly Skill[];
}

/** One page of the master's "O que mudou". */
export interface LevelUpPage {
  readonly levelUps: readonly LevelUp[];
  readonly nextPageToken: string;
}

/**
 * Thin wrapper around the guided level-up's calls (MR-040) on the generated
 * `CharacterService`, plus what the pickers need from `ContentService` and the
 * campaign. `providedIn: 'root'`, and only lazy code imports it, so the
 * generated clients stay out of the initial bundle. Callers map errors to
 * Portuguese with `levelup-errors.ts`; nothing here knows a rule: the server
 * derives every number the screen shows (ADR-0008).
 */
@Injectable({ providedIn: 'root' })
export class LevelUpClient {
  private readonly characters = createClient(CharacterService, inject(CONNECT_TRANSPORT));
  private readonly content = createClient(ContentService, inject(CONNECT_TRANSPORT));
  private readonly campaigns = createClient(CampaignService, inject(CONNECT_TRANSPORT));

  /** The rules content is the same for every call of a page: read once. */
  private readonly catalogs = new Map<string, Promise<LevelUpCatalog>>();
  private readonly details = new Map<string, Promise<SpellDetailsVm>>();

  async character(campaignId: string, characterId: string): Promise<Character> {
    const res = await this.characters.getCharacter({ campaignId, characterId });
    return res.character!;
  }

  async options(campaignId: string, characterId: string): Promise<LevelUpOptions> {
    const res = await this.characters.getLevelUpOptions({ campaignId, characterId });
    return res.options!;
  }

  preview(
    campaignId: string,
    characterId: string,
    choices: LevelUpChoicesInit,
  ): Promise<PreviewLevelUpResponse> {
    return this.characters.previewLevelUp({ campaignId, characterId, choices });
  }

  rollHitPoints(
    campaignId: string,
    characterId: string,
    classKey: string,
    idempotencyKey: string,
  ): Promise<RollLevelUpHitPointsResponse> {
    return this.characters.rollLevelUpHitPoints({ campaignId, characterId, classKey, idempotencyKey });
  }

  async levelUp(
    campaignId: string,
    characterId: string,
    revision: number,
    choices: LevelUpChoicesInit,
  ): Promise<Character> {
    const res = await this.characters.levelUpCharacter({ campaignId, characterId, revision, choices });
    return res.character!;
  }

  /** The master's "O que mudou", newest first. */
  async list(campaignId: string, pageSize = 50, pageToken = ''): Promise<LevelUpPage> {
    const res = await this.characters.listLevelUps({ campaignId, pageSize, pageToken });
    return { levelUps: res.levelUps, nextPageToken: res.nextPageToken };
  }

  catalog(campaignId: string): Promise<LevelUpCatalog> {
    let pending = this.catalogs.get(campaignId);
    if (!pending) {
      pending = this.content.listContent({ campaignId }).then((res) => ({
        spells: res.content?.spells ?? [],
        skills: res.content?.skills ?? [],
      }));
      this.catalogs.set(campaignId, pending);
      pending.catch(() => this.catalogs.delete(campaignId));
    }
    return pending;
  }

  /** The "?" of a spell: its details, kept per spell so a second open is instant. */
  spellDetails(campaignId: string, spellKey: string): Promise<SpellDetailsVm> {
    const id = `${campaignId}/${spellKey}`;
    let pending = this.details.get(id);
    if (!pending) {
      pending = this.content
        .getSpellDetails({ campaignId, spellKey })
        .then((res) => spellDetailsFromGen(res.spell!));
      this.details.set(id, pending);
      pending.catch(() => this.details.delete(id));
    }
    return pending;
  }

  /** How the campaign levels (RN-09): the blocked page says what is missing by it. */
  async xpMode(campaignId: string): Promise<XpMode> {
    const res = await this.campaigns.getCampaign({ campaignId });
    return res.campaign?.xpMode ?? XpMode.UNSPECIFIED;
  }

  /** The caller's own dice choice (RN-18): which way of rolling opens first. */
  async dicePreference(campaignId: string): Promise<DicePreference> {
    const res = await this.campaigns.getCampaign({ campaignId });
    return res.campaign?.myDicePreference ?? DicePreference.UNSPECIFIED;
  }
}
