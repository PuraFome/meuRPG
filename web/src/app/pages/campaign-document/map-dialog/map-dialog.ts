import { Component, OnInit, inject, input, output, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { RouterLink } from '@angular/router';

import { DocDialog } from '../doc-dialog/doc-dialog';
import { DocumentLinks, type MapView } from '../document-clients';
import { type LookupState, lookupFailure } from '../document-copy';

/**
 * The map link's dialog (E5-29): the map's name, whether it is revealed
 * (only the master reads the document, so the line says it either way), its
 * image and "Abrir mapa". Read-only. The details come from `GetMap`, the
 * same authorized call as the maps screens: a map deleted since the link
 * was written answers `not_found` and shows "Este mapa foi apagado."
 */
@Component({
  selector: 'app-document-map-dialog',
  imports: [DocDialog, MatButtonModule, MatIconModule, RouterLink],
  templateUrl: './map-dialog.html',
  styleUrl: './map-dialog.scss',
})
export class DocumentMapDialog implements OnInit {
  private readonly links = inject(DocumentLinks);

  readonly campaignId = input.required<string>();
  readonly mapId = input.required<string>();
  /** The link's own text, for the title while the map loads. */
  readonly text = input('');
  readonly closed = output<void>();

  protected readonly state = signal<LookupState<MapView>>({ status: 'loading' });

  ngOnInit(): void {
    this.load();
  }

  protected load(): void {
    this.state.set({ status: 'loading' });
    this.links.getMap(this.campaignId(), this.mapId()).then(
      (value) => this.state.set({ status: 'ready', value }),
      (err: unknown) => this.state.set(lookupFailure(err)),
    );
  }
}
