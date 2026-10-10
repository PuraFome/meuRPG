import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatSelectModule } from '@angular/material/select';

import type { FeatCatalogVm } from '../character-editor.types';

/** One feat the sheet has, as the list shows it. */
interface FeatRow {
  readonly key: string;
  readonly name: string;
  /** Where it came from, in words. */
  readonly source: string;
  /** What the server said about it in the draft (a prerequisite not met), or empty. */
  readonly warning: string;
  readonly table: boolean;
}

/**
 * "Talentos" (MR-025): the feats of a sheet, for the master. The master adds a feat from the campaign's list (the SRD's
 * and the table's own, without the retired ones) and removes one, whatever the table rule "Talentos" says: the master
 * decides. A feat taken at a level-up in place of an Ability Score Improvement says so, and removing it brings the
 * improvement back. The prerequisite is the server's: `issues` carries what it said about the draft, shown as a warning
 * that does not stop the master. A feat that raises abilities is not asked which ones here: the master gives them in
 * "Bônus manuais". Only the master sees this; the editor decides.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-feats-field',
  imports: [MatButtonModule, MatFormFieldModule, MatIconModule, MatSelectModule],
  templateUrl: './feats-field.html',
  styleUrl: './feats-field.scss',
})
export class FeatsField {
  /** The feats the sheet has, as content keys. */
  readonly featKeys = input.required<readonly string[]>();
  /** A feat taken in place of an improvement (feat key to feature key). */
  readonly featSlots = input<Readonly<Record<string, string>>>({});
  /** The campaign's feats and whether the table uses them; absent while it could not be read. */
  readonly catalog = input<FeatCatalogVm | undefined>(undefined);
  /** The server's warning per feat key. */
  readonly issues = input<Readonly<Record<string, string>>>({});

  readonly added = output<string>();
  readonly removed = output<string>();

  protected readonly rows = computed<readonly FeatRow[]>(() => {
    const options = new Map((this.catalog()?.options ?? []).map((o) => [o.key, o]));
    return this.featKeys().map((key) => {
      const option = options.get(key);
      return {
        key,
        name: option?.namePt ?? key,
        table: option?.fromTable ?? key.endsWith('@mesa'),
        source:
          key in this.featSlots()
            ? 'Talento · no lugar de um aumento de habilidade'
            : 'Talento · dado pelo mestre',
        warning: this.issues()[key] ?? '',
      };
    });
  });

  /** The feats that can still be added: not the ones the sheet has. */
  protected readonly available = computed(() => {
    const has = new Set(this.featKeys());
    return (this.catalog()?.options ?? []).filter((o) => !has.has(o.key));
  });

  protected readonly rulesOff = computed(() => this.catalog()?.featsAllowed === false);
  protected readonly unavailable = computed(() => this.catalog() === undefined);

  protected add(key: string): void {
    if (key) {
      this.added.emit(key);
    }
  }
}
