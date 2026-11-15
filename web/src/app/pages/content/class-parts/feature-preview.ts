import { Component, input } from '@angular/core';

import type { TableEntry } from '../../../../gen/meurpg/rules/v1/table_content_pb';
import type { CatalogVm } from '../../../core/content/catalog';
import type { EffectMenuVm } from '../../../core/content/effect-draft';
import type { FeatureDraft } from '../../../core/content/feature-draft';

/** "Como os jogadores veem" for the open feature of a class or subclass; stub. */
@Component({ selector: 'app-feature-preview', template: '' })
export class FeaturePreview {
  readonly feature = input<FeatureDraft | null>(null);
  readonly menu = input.required<EffectMenuVm>();
  readonly catalog = input<CatalogVm | null>(null);
  readonly entries = input<readonly TableEntry[]>([]);
}
