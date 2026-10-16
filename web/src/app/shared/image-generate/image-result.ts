import { signal } from '@angular/core';

import type { GalleryImage } from '../../../gen/meurpg/maps/v1/gallery_pb';
import type { ImageEdit } from '../../../gen/meurpg/maps/v1/imagegen_pb';
import type { Map as MapMessage } from '../../../gen/meurpg/maps/v1/maps_pb';
import { KIND_LABEL } from '../../core/images/imagegen-copy';
import type { ImageGenClient } from '../../core/images/imagegen-client';
import { useIssue } from '../../core/images/imagegen-errors';
import { type ShownImageClient, showImageIssue } from '../../core/images/shown-client';
import type { ResultView, ShowStage, UseStage } from './generate-result';
import { chainView, openedView } from './image-generate-view';

/** The picture on screen and what can be done with it: show it to the players, use it as the map's image, look at its chain of adjustments. */
export class GeneratedResult {
  readonly view = signal<ResultView | null>(null);
  readonly chain = signal<readonly ImageEdit[]>([]);
  readonly shownIds = signal<ReadonlySet<string>>(new Set());
  readonly showBusy = signal(false);
  readonly showError = signal<string | null>(null);
  readonly showStage = signal<ShowStage>('idle');
  readonly useStage = signal<UseStage>('idle');
  readonly useError = signal<string | null>(null);
  /** The way of the picture in words ("Ajuste", "O mapa com textura"), under the heading. */
  readonly kindText = signal('');
  /** The map the master changed with "Usar como imagem do mapa", for the opener. */
  changedMap: MapMessage | null = null;

  constructor(
    private readonly deps: {
      readonly api: ImageGenClient;
      readonly shownApi: ShownImageClient;
      readonly campaignId: string;
      /** The question over the actions is at the end of the body: the dialog scrolls and focuses there. */
      readonly toTheQuestion: () => void;
    },
  ) {}

  /** A new picture to look at (one that just came back, or an older one); the questions over the old one are gone. */
  present(view: ResultView, kindText: string): void {
    this.kindText.set(kindText);
    this.view.set(view);
    this.showStage.set('idle');
    this.useStage.set('idle');
    this.useError.set(null);
    this.showError.set(null);
  }

  /** An older generated image: its chain, and its place in it. */
  async openOn(image: GalleryImage): Promise<void> {
    let chain: ImageEdit[] = [];
    try {
      chain = await this.deps.api.edits(this.deps.campaignId, image.id);
    } catch {
      // The chain is a detail: the picture opens without it.
    }
    this.chain.set(chain);
    this.present(
      openedView(image, chain),
      image.showsWholeMap ? KIND_LABEL.texture.long : 'Imagem gerada',
    );
  }

  shown(): boolean {
    const id = this.view()?.image.id;
    return id !== undefined && this.shownIds().has(id);
  }

  /** Another picture of the chain. */
  pickChain(imageId: string, fromMap: boolean): void {
    const edit = this.chain().find((e) => e.image?.id === imageId);
    if (!edit?.image) {
      return;
    }
    this.view.set(chainView({ ...edit, image: edit.image }, fromMap));
    this.useStage.set('idle');
    this.showStage.set('idle');
    this.showError.set(null);
  }

  async loadChain(image: GalleryImage): Promise<void> {
    try {
      const chain = await this.deps.api.edits(this.deps.campaignId, image.id);
      if (
        this.view()?.image.id === image.id ||
        chain.some((e) => e.image?.id === this.view()?.image.id)
      ) {
        this.chain.set(chain);
      }
    } catch {
      this.chain.set([]);
    }
  }

  /** "Mostrar aos jogadores" of a picture that shows the whole map asks first, in place (RN-10). */
  askShow(): void {
    this.showError.set(null);
    this.showStage.set('asking');
    this.deps.toTheQuestion();
  }

  cancelShow(): void {
    this.showStage.set('idle');
    this.showError.set(null);
  }

  async show(): Promise<void> {
    const image = this.view()?.image;
    if (!image || this.showBusy()) {
      return;
    }
    this.showBusy.set(true);
    this.showError.set(null);
    try {
      await this.deps.shownApi.show(this.deps.campaignId, image.id);
      this.shownIds.update((ids) => new Set([...ids, image.id]));
      this.showStage.set('idle');
    } catch (err) {
      this.showError.set(showImageIssue(err));
    } finally {
      this.showBusy.set(false);
    }
  }

  askUse(): void {
    this.useError.set(null);
    this.useStage.set('asking');
    this.deps.toTheQuestion();
  }

  cancelUse(): void {
    this.useStage.set('idle');
    this.useError.set(null);
  }

  async confirmUse(): Promise<void> {
    const image = this.view()?.image;
    if (!image || this.useStage() === 'busy') {
      return;
    }
    this.useStage.set('busy');
    this.useError.set(null);
    try {
      this.changedMap = await this.deps.api.useAsMapImage(this.deps.campaignId, image.id);
      this.useStage.set('done');
    } catch (err) {
      this.useError.set(useIssue(err));
      this.useStage.set('asking');
    }
  }
}
