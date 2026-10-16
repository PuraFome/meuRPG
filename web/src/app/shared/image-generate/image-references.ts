import { computed, signal } from '@angular/core';

import { ImageGenerationKind } from '../../../gen/meurpg/maps/v1/imagegen_pb';
import { pngDataUrl } from '../../core/images/bytes-url';
import type { ImageGenClient } from '../../core/images/imagegen-client';
import type { PickableNpc } from '../../core/images/imagegen-form';
import type { MapsClient } from '../../core/maps/maps-client';
import type { RosterClient } from '../../core/maps/roster-client';
import type { ReferenceView } from './generate-form';

type ReferenceAnswer = Awaited<ReturnType<ImageGenClient['reference']>>;

function viewOf(res: ReferenceAnswer): ReferenceView {
  return {
    previewUrl: pngDataUrl(res.preview, res.previewContentType || 'image/png'),
    seenSquares: res.seenSquares,
    totalSquares: res.totalSquares,
    playersSee: res.playersSeeSomething,
    rooms: res.roomsListed,
    gridColumns: res.gridColumns,
    gridRows: res.gridRows,
  };
}

/** What the dialog reads from the server about the map it was opened from: the drawings that go along, the NPCs and the hidden portraits. */
export class ImageReferences {
  // What the server drew of the map, for the way chosen: one for the scene art and the isometric view, one for the textured map.
  readonly scene = signal<ReferenceView | null>(null);
  readonly texture = signal<ReferenceView | null>(null);
  readonly npcs = signal<readonly PickableNpc[]>([]);
  readonly textureTooLarge = signal(false);
  readonly state = signal<'none' | 'loading' | 'ready' | 'error'>('none');
  /** The portraits of NPCs on this map that the players do not see: a request would be refused with them, so the picker leaves them out. */
  readonly hiddenPortraits = signal<ReadonlySet<string>>(new Set());
  readonly textureGrid = computed(() => {
    const t = this.texture();
    return t ? { columns: t.gridColumns, rows: t.gridRows } : null;
  });

  constructor(
    private readonly deps: {
      readonly api: ImageGenClient;
      readonly roster: RosterClient;
      readonly maps: MapsClient;
      readonly campaignId: string;
    },
  ) {}

  /** The drawing for the scene art (and the isometric view, which starts from the same view) and for the textured map, in parallel. */
  async load(mapId: string): Promise<void> {
    this.state.set('loading');
    const { api, campaignId } = this.deps;
    try {
      const [scene, texture] = await Promise.all([
        api.reference(campaignId, mapId, ImageGenerationKind.MAP_SCENE),
        api.reference(campaignId, mapId, ImageGenerationKind.TEXTURED_MAP),
      ]);
      this.scene.set(viewOf(scene));
      this.texture.set(viewOf(texture));
      this.textureTooLarge.set(texture.textureTooLarge);
      this.npcs.set(
        scene.creatures.map((c) => ({
          characterId: c.characterId,
          name: c.name,
          portraitImageId: c.portraitImageId,
        })),
      );
      this.state.set('ready');
    } catch {
      this.state.set('error');
    }
  }

  /**
   * The portraits of the NPCs that have a token on this map and are not on the players' list: the server refuses them as references (also
   * as objects), so the picker does not offer them. Best effort: when it cannot be read, the server's refusal says it. Gives the ids, or
   * `null` when they could not be read.
   */
  async readHiddenPortraits(mapId: string): Promise<ReadonlySet<string> | null> {
    const { roster, maps, campaignId } = this.deps;
    try {
      const [list, map] = await Promise.all([roster.list(campaignId), maps.get(campaignId, mapId)]);
      const seen = new Set(this.npcs().map((n) => n.characterId));
      const onMap = new Set(map.tokens.map((t) => t.characterId));
      const hidden = new Set(
        list
          .filter(
            (c) =>
              c.playerUserId === '' &&
              onMap.has(c.id) &&
              !seen.has(c.id) &&
              c.portraitImageId !== '',
          )
          .map((c) => c.portraitImageId),
      );
      this.hiddenPortraits.set(hidden);
      return hidden;
    } catch {
      // The picker offers everything; a refused portrait is said by the server.
      return null;
    }
  }
}
