import type { SelectOption } from '../../shared/form-fields/select-field';
import type { CatalogVm } from './catalog';
import type { FeatureDraft } from './feature-draft';
import type { KeyName } from './key-name';

/** Names the keys an option asks for; stub. */
export function askKeyNamer(
  _options: readonly FeatureDraft[],
  _catalog: CatalogVm | null,
  _requirable: readonly SelectOption[],
): KeyName {
  return (key) => key;
}
