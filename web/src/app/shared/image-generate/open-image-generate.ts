import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';
import type { Observable } from 'rxjs';

import { openSheet } from '../sheet/sheet-host';
import { ImageGenerateDialog } from './image-generate-dialog';
import type { GenerateOutcome, ImageGenerateData } from './image-generate.types';

/** Opens "Gerar imagem": a dialog from a tablet up and a bottom sheet with a fixed footer on a phone. The opener gets what was made when it closes. */
export function openImageGenerate(
  dialog: MatDialog,
  bottomSheet: MatBottomSheet,
  data: ImageGenerateData,
): Observable<GenerateOutcome | undefined> {
  return openSheet<ImageGenerateDialog, ImageGenerateData, GenerateOutcome>(
    dialog,
    bottomSheet,
    ImageGenerateDialog,
    {
      data,
      ariaLabel: 'Gerar imagem',
      labelledBy: 'gen-t',
      width: '580px',
      panelClass: 'mr-sheet-image',
    },
  );
}
