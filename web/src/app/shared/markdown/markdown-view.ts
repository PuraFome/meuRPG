import { NgTemplateOutlet } from '@angular/common';
import { Component, computed, input, output, signal } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { type Block, type Inline, type RefKind, parseMarkdown } from './markdown';

/** What exists in the campaign, so a link to something deleted can say so.
 * `null` (not loaded yet, or the call failed), as a whole or for one kind,
 * shows those links as usual. */
export interface MarkdownRefs {
  readonly maps: ReadonlySet<string> | null;
  readonly characters: ReadonlySet<string> | null;
  readonly images: ReadonlyMap<string, { readonly width: number; readonly height: number }> | null;
}

/** A click on a `map:` or `character:` link. */
export interface RefOpen {
  readonly kind: RefKind;
  readonly id: string;
  readonly text: string;
}

/**
 * Renders a campaign document (MR-018) from its Markdown body: the reading
 * view (E5-27) and the editor's live preview (E5-28).
 *
 * The body becomes a token tree (`parseMarkdown`) and the template draws
 * each token with text interpolation and property bindings, never
 * `innerHTML`, so nothing the text says can run. Images come from
 * `/images/<id>` with their caption (`<figure>`); one that fails to load
 * shows "Imagem apagada". `map:` and `character:` links are buttons that ask
 * the page to open a dialog (`openRef`); one whose target is gone (`refs`
 * says so) is plain text followed by "(mapa apagado)" or "(ficha apagada)".
 *
 * The caller sets `--md-inset` (the text's side margin) and `--md-bleed`
 * (how far a figure breaks out of it) on a parent.
 */
@Component({
  selector: 'app-markdown',
  imports: [MatIconModule, NgTemplateOutlet],
  templateUrl: './markdown-view.html',
  styleUrl: './markdown-view.scss',
})
export class MarkdownView {
  /** The Markdown text. */
  readonly source = input.required<string>();
  readonly refs = input<MarkdownRefs | null>(null);
  /** Prefix of the headings' ids (`<prefix><block index>`), for the
   * table of contents; differs between the page and the preview. */
  readonly idPrefix = input('doc-h-');
  readonly openRef = output<RefOpen>();

  protected readonly blocks = computed<Block[]>(() => parseMarkdown(this.source()));
  private readonly failed = signal<ReadonlySet<string>>(new Set());

  /** The parsed blocks, for the table of contents. */
  readonly parsed = this.blocks;

  protected isMissing(kind: RefKind, id: string): boolean {
    const known = kind === 'map' ? this.refs()?.maps : this.refs()?.characters;
    return known != null && !known.has(id);
  }

  protected imageGone(id: string): boolean {
    const images = this.refs()?.images;
    return this.failed().has(id) || (images != null && !images.has(id));
  }

  protected imageSize(id: string): { width: number; height: number } | null {
    return this.refs()?.images?.get(id) ?? null;
  }

  protected markFailed(id: string): void {
    this.failed.update((set) => new Set(set).add(id));
  }

  protected open(node: Extract<Inline, { type: 'ref' }>): void {
    this.openRef.emit({ kind: node.kind, id: node.id, text: node.text });
  }
}
