import type { GalleryImage } from '../../../gen/meurpg/maps/v1/gallery_pb';
import type { Map as MapMessage } from '../../../gen/meurpg/maps/v1/maps_pb';
import type { GenerateOrigin } from '../../core/images/imagegen-copy';

/** What the opener hands the dialog. */
export interface ImageGenerateData {
  readonly campaignId: string;
  readonly origin: GenerateOrigin;
  /** An older generated image of the gallery: the dialog opens on it, to adjust it (E10-07 7). */
  readonly image?: GalleryImage;
}

/** What the dialog tells the opener when it closes: how many pictures it made, and the map it changed ("Usar como imagem do mapa"). */
export interface GenerateOutcome {
  readonly generated: number;
  /** The last picture made while the dialog was open (the one the master ended on), or `null` when none was. */
  readonly lastImageId: string | null;
  readonly map: MapMessage | null;
}

export type Stage = 'loading' | 'form' | 'running' | 'result';

/** Where a request goes back to when it ends without a picture: a new image to the form, an adjustment to the result it started from. */
export type RunFor = 'generate' | 'edit';
