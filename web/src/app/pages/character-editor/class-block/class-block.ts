import { Component, computed, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';

import { TableMark } from '../../../shared/table-mark/table-mark';
import type { ClassBlock } from '../class-blocks';
import type { RulesCatalogVm } from '../character-editor.types';

/**
 * One class of a multiclass sheet (MR-025, E10-02 state 6): "Classe 2", with its class, its level
 * in it and its subclass. The table's classes and subclasses carry "Da mesa". The subclass is
 * offered once the level reaches the one the class chooses it at (the catalog's `subclass_level`;
 * the server checks it anyway and shows an issue when a sheet takes it early). Which classes a
 * multiclass may take (the SRD's prerequisites) is the server's: it shows as issues on the sheet.
 */
@Component({
  selector: 'app-class-block',
  imports: [MatButtonModule, MatFormFieldModule, MatIconModule, MatInputModule, MatSelectModule, TableMark],
  templateUrl: './class-block.html',
  styleUrl: './class-block.scss',
})
export class ClassBlockFields {
  readonly catalog = input.required<RulesCatalogVm>();
  readonly block = input.required<ClassBlock>();
  /** Position in the sheet, from 0. */
  readonly index = input.required<number>();
  /** The first class cannot be removed. */
  readonly removable = input(false);

  readonly changed = output<Partial<ClassBlock>>();
  readonly removed = output<void>();

  protected readonly cls = computed(() => this.catalog().classes.find((c) => c.key === this.block().classKey));
  protected readonly title = computed(() => `Classe ${this.index() + 1}`);
  protected readonly titleId = computed(() => `class-block-${this.index()}`);
  protected readonly subclasses = computed(() => this.cls()?.subclasses ?? []);
  protected readonly chosenSubclass = computed(() => this.subclasses().find((c) => c.key === this.block().subclassKey));

  /** The subclass field stays shut until the class's level for it, unless one is already chosen. */
  protected readonly subclassWaits = computed(() => {
    const c = this.cls();
    return !!c && c.subclassLevel > 0 && this.block().level < c.subclassLevel && this.block().subclassKey === '';
  });
  protected readonly subclassHint = computed(() => {
    const c = this.cls();
    return c && c.subclassLevel > 0 && this.block().level < c.subclassLevel
      ? `O ${c.namePt} escolhe a subclasse no nível ${c.subclassLevel}.`
      : '';
  });

  protected pickClass(classKey: string): void {
    // A subclass belongs to one class: the old one goes with it.
    this.changed.emit({ classKey, subclassKey: '', customSubclassName: '' });
  }

  protected setLevel(value: number): void {
    this.changed.emit({ level: Number.isNaN(value) ? 0 : value });
  }
}
