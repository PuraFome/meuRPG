import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import {
  type CampaignDocument,
  CampaignDocumentService,
} from '../../../gen/meurpg/campaigns/v1/campaign_document_pb';
import { CharacterService } from '../../../gen/meurpg/characters/v1/characters_pb';
import { MapService } from '../../../gen/meurpg/maps/v1/maps_pb';
import { PlayService } from '../../../gen/meurpg/play/v1/play_pb';
import { CONNECT_TRANSPORT } from '../../core/connect/transport';
import { ViewPoint } from '../../shared/map-view/map-geometry';

/** A map, as the document's dialogs show it. */
export interface MapView {
  readonly id: string;
  readonly name: string;
  readonly revealed: boolean;
  /** Absent in a listing; present from `GetMap`. */
  readonly image: { readonly url: string; readonly width: number; readonly height: number } | null;
  /** Every point, hidden ones too (only the master reads the document);
   * empty in a listing. */
  readonly points: readonly ViewPoint[];
}

/** The campaign's open session and the map it is on. */
export interface OpenSessionMap {
  readonly sessionNumber: number;
  /** Empty while the master has not chosen a map. */
  readonly mapId: string;
}

/** A character, as the document's dialogs show it. */
export interface SheetView {
  readonly id: string;
  readonly name: string;
  /** "Mago 3", or "" for a basic sheet. */
  readonly classSummary: string;
  /** "Gnomo das Rochas", or "". */
  readonly raceName: string;
}

/**
 * The generated `CampaignDocumentService` client (MR-018), master-only on
 * the server. `providedIn: 'root'`, imported only by lazy code.
 */
@Injectable({ providedIn: 'root' })
export class DocumentClient {
  private readonly client = createClient(CampaignDocumentService, inject(CONNECT_TRANSPORT));

  async get(campaignId: string): Promise<CampaignDocument> {
    const res = await this.client.getCampaignDocument({ campaignId });
    if (!res.document) {
      throw new Error('GetCampaignDocument answered without the document');
    }
    return res.document;
  }

  async save(campaignId: string, body: string, expectedRevision: number): Promise<CampaignDocument> {
    const res = await this.client.updateCampaignDocument({ campaignId, body, expectedRevision });
    if (!res.document) {
      throw new Error('UpdateCampaignDocument answered without the document');
    }
    return res.document;
  }
}

/**
 * What the document's links point to, through the ordinary authorized calls
 * (maps and characters services): the pickers' lists, the dialogs' details,
 * and the sets that tell a deleted target from a live one.
 */
@Injectable({ providedIn: 'root' })
export class DocumentLinks {
  private readonly maps = createClient(MapService, inject(CONNECT_TRANSPORT));
  private readonly characters = createClient(CharacterService, inject(CONNECT_TRANSPORT));
  private readonly play = createClient(PlayService, inject(CONNECT_TRANSPORT));

  async listMaps(campaignId: string): Promise<MapView[]> {
    const res = await this.maps.listMaps({ campaignId });
    return res.maps.map((m) => ({
      id: m.id,
      name: m.name,
      revealed: m.revealed,
      image: m.image
        ? { url: m.image.url, width: m.image.width, height: m.image.height }
        : null,
      points: [],
    }));
  }

  async getMap(campaignId: string, mapId: string): Promise<MapView> {
    const res = await this.maps.getMap({ campaignId, mapId });
    const m = res.map;
    if (!m) {
      throw new Error('GetMap answered without the map');
    }
    return {
      id: m.id,
      name: m.name,
      revealed: m.revealed,
      image: m.image ? { url: m.image.url, width: m.image.width, height: m.image.height } : null,
      points: res.points,
    };
  }

  /** The open session's number and current map, or null when the campaign
   * has no open session (`failed_precondition`) or the call fails: the
   * dialog only adds "Mapa atual da Sessão 4" when it knows. */
  async openSessionMap(campaignId: string): Promise<OpenSessionMap | null> {
    try {
      const res = await this.play.getLiveSession({ campaignId });
      return res.gameSession ? { sessionNumber: res.gameSession.sessionNumber, mapId: res.currentMapId } : null;
    } catch {
      return null;
    }
  }

  async listCharacters(campaignId: string): Promise<SheetView[]> {
    const res = await this.characters.listCharacters({ campaignId });
    return res.characters.map((c) => ({
      id: c.id,
      name: c.name,
      classSummary: c.classSummary,
      raceName: c.raceNamePt,
    }));
  }

  async getCharacter(campaignId: string, characterId: string): Promise<SheetView> {
    const res = await this.characters.getCharacter({ campaignId, characterId });
    const c = res.character;
    if (!c) {
      throw new Error('GetCharacter answered without the character');
    }
    return {
      id: c.id,
      name: c.name,
      classSummary: c.derived ? c.derived.classes.map((k) => `${k.namePt} ${k.level}`).join(' / ') : '',
      raceName: c.derived?.subraceNamePt || c.derived?.raceNamePt || '',
    };
  }
}
