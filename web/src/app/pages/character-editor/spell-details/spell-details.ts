import { Component, computed, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MAT_BOTTOM_SHEET_DATA, MatBottomSheetRef } from '@angular/material/bottom-sheet';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';

import { SpellDetailsVm } from '../character-editor.types';
import { SpellFieldValue, spellFields, spellSubtitle } from './spell-details-format';

export interface SpellDetailsData {
  /** The Portuguese name the "?" belongs to: the title shows it while the details load. */
  readonly namePt: string;
  /** Fetches the details (the page caches them per spell, so a second open is instant). */
  readonly load: () => Promise<SpellDetailsVm>;
}

type DetailsState =
  { status: 'loading' } | { status: 'error' } | { status: 'ready'; details: SpellDetailsVm };

/**
 * What the "?" next to a spell opens (E6-22, E6-23): the spell's name, its
 * circle and school, the four things a player looks up at the table (casting
 * time, range, components, duration) in Portuguese, and the SRD's own text in
 * English, marked as such. One component in two containers: a bottom sheet
 * on a phone, a 560px dialog from a tablet up (the page picks). Esc, the X
 * and "Fechar" close it, and focus goes back to the "?" that opened it
 * (Material does that for both containers).
 */
@Component({
  selector: 'app-spell-details',
  imports: [MatButtonModule, MatIconModule, MatProgressSpinnerModule],
  templateUrl: './spell-details.html',
  styleUrl: './spell-details.scss',
})
export class SpellDetails {
  protected readonly data =
    inject<SpellDetailsData | null>(MAT_DIALOG_DATA, { optional: true }) ??
    inject<SpellDetailsData>(MAT_BOTTOM_SHEET_DATA);
  private readonly dialogRef = inject<MatDialogRef<SpellDetails>>(MatDialogRef, {
    optional: true,
  });
  private readonly sheetRef = inject<MatBottomSheetRef<SpellDetails>>(MatBottomSheetRef, {
    optional: true,
  });

  protected readonly inSheet = this.sheetRef !== null;
  protected readonly state = signal<DetailsState>({ status: 'loading' });

  protected readonly details = computed(() => {
    const s = this.state();
    return s.status === 'ready' ? s.details : null;
  });
  protected readonly fields = computed(() => {
    const d = this.details();
    return d ? spellFields(d) : null;
  });
  protected readonly subtitle = computed(() => {
    const d = this.details();
    return d ? spellSubtitle(d) : '';
  });
  /** The four fields in the order the sheet prints them. */
  protected readonly rows = computed<{ label: string; value: SpellFieldValue }[]>(() => {
    const f = this.fields();
    return f
      ? [
          { label: 'Tempo de conjuração', value: f.castingTime },
          { label: 'Alcance', value: f.range },
          { label: 'Componentes', value: f.components },
          { label: 'Duração', value: f.duration },
        ]
      : [];
  });

  constructor() {
    void this.load();
  }

  protected async load(): Promise<void> {
    this.state.set({ status: 'loading' });
    try {
      this.state.set({ status: 'ready', details: await this.data.load() });
    } catch {
      this.state.set({ status: 'error' });
    }
  }

  protected close(): void {
    this.dialogRef?.close();
    this.sheetRef?.dismiss();
  }
}
