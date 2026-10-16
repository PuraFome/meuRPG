import { signal } from '@angular/core';

import type { GalleryImage } from '../../../gen/meurpg/maps/v1/gallery_pb';
import { ImageAspectRatio, ImageStyle } from '../../../gen/meurpg/maps/v1/imagegen_pb';
import { type GenerateOrigin, defaultImageName } from '../../core/images/imagegen-copy';
import type { GenerateIssue } from '../../core/images/imagegen-errors';
import type { ImageForm } from '../../core/images/imagegen-form';

/** The form the master fills, the references he picked and the refusal over them. */
export class ImageFormState {
  readonly form: ReturnType<typeof signal<ImageForm>>;
  readonly references = signal<readonly GalleryImage[]>([]);
  readonly issue = signal<GenerateIssue | null>(null);
  private nameTouched = false;

  constructor(private readonly origin: GenerateOrigin) {
    this.form = signal<ImageForm>({
      kind: 'scene',
      prompt: origin.kind === 'scene' ? (origin.name ?? '') : '',
      name: defaultImageName(origin, 'scene'),
      style: ImageStyle.OIL_PAINTING,
      ratio: ImageAspectRatio.IMAGE_ASPECT_RATIO_UNSPECIFIED,
      objectImageIds: [],
      npcIds: [],
    });
  }

  patch(patch: Partial<ImageForm>): void {
    if (patch.name !== undefined) {
      this.nameTouched = true;
    }
    this.form.update((f) => {
      const next = { ...f, ...patch };
      // Until the master writes a name, it follows the way ("… · vista isométrica").
      return patch.kind !== undefined && !this.nameTouched
        ? { ...next, name: defaultImageName(this.origin, patch.kind) }
        : next;
    });
    this.issue.set(null);
  }

  hasReference(id: string): boolean {
    return this.form().objectImageIds.includes(id);
  }

  addReference(image: GalleryImage): void {
    this.references.update((list) => [...list, image]);
    this.patch({ objectImageIds: [...this.form().objectImageIds, image.id] });
  }

  removeReference(id: string): void {
    this.references.update((list) => list.filter((i) => i.id !== id));
    this.patch({ objectImageIds: this.form().objectImageIds.filter((i) => i !== id) });
  }

  /** The portraits the players do not see leave the references (and the form that carries them). */
  dropHidden(hidden: ReadonlySet<string>): void {
    this.references.update((l) => l.filter((r) => !hidden.has(r.id)));
    this.form.update((f) => ({
      ...f,
      objectImageIds: f.objectImageIds.filter((id) => !hidden.has(id)),
    }));
  }

  /** The marked NPCs that are no longer listed are dropped. */
  keepNpcs(listed: ReadonlySet<string>): void {
    this.form.update((f) => ({ ...f, npcIds: f.npcIds.filter((id) => listed.has(id)) }));
  }
}
