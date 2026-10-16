import type { GalleryImage } from '../../../gen/meurpg/maps/v1/gallery_pb';
import type { ImagePickerData } from '../gallery-picker/image-picker-dialog/image-picker-dialog';

/** "Escolher uma referência": the gallery picker, set up to add the picked image as a reference of the request. */
export function referencePickerOptions(o: {
  readonly campaignId: string;
  /** The portraits of NPCs the players do not see: the server refuses them as references. */
  readonly excluded: ReadonlySet<string>;
  readonly has: (imageId: string) => boolean;
  readonly add: (image: GalleryImage) => void;
}): ImagePickerData {
  return {
    campaignId: o.campaignId,
    title: 'Escolher uma referência',
    lead: 'A imagem vai ao Google como referência para a nova imagem.',
    confirmLabel: 'Usar como referência',
    confirmIcon: 'image',
    current: null,
    currentTag: '',
    currentNote: '',
    note: () => 'Não use foto de pessoa: a imagem inteira vai ao Google.',
    noteIcon: 'info',
    hiddenMapImages: new Map(),
    excluded: o.excluded,
    emptyError: 'Escolha uma imagem para usar como referência.',
    submit: async (image) => {
      if (o.has(image.id)) {
        throw new Error('duplicate');
      }
      o.add(image);
    },
    errorMessage: () => 'Essa imagem já é uma referência.',
  };
}
