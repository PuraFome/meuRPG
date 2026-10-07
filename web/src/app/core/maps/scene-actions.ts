import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import { ContentService } from '../../../gen/meurpg/rules/v1/rules_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';
import { ABILITY_LABELS } from '../characters/character-labels';
import { ABILITY_KEYS, type AbilityKey } from '../characters/characters.types';

/** A scene has at most 20 actions (maps.proto, `AddSceneAction`). */
export const SCENE_ACTION_LIMIT = 20;
/** The action's own name: up to 60 characters. */
export const SCENE_ACTION_NAME_MAX = 60;
export const SCENE_DC_MIN = 1;
export const SCENE_DC_MAX = 30;

/** What the master picks first in the add form: where the list below comes from. */
export type CheckKind = 'skill' | 'ability' | 'save';

export const CHECK_KINDS: readonly { readonly kind: CheckKind; readonly label: string }[] = [
  { kind: 'skill', label: 'Perícia' },
  { kind: 'ability', label: 'Teste de habilidade' },
  { kind: 'save', label: 'Teste de resistência' },
];

/** One line of the check list: the key the server takes and the name on screen. */
export interface CheckOption {
  readonly key: string;
  readonly label: string;
}

const ABILITIES: readonly AbilityKey[] = ABILITY_KEYS;

/** The list the second field offers for a kind: the 18 skills (from the rules'
 * content), the six abilities or the six saves. */
export function checkOptions(kind: CheckKind, skills: readonly CheckOption[]): readonly CheckOption[] {
  if (kind === 'skill') {
    return skills;
  }
  return ABILITIES.map((a) => ({ key: `${kind}:${a}`, label: ABILITY_LABELS[a] }));
}

/** What an action is called on screen: its own name, or the check's name. */
export function actionTitle(action: { name: string; checkName: string }): string {
  return action.name || action.checkName;
}

/** The kind of a check by its key, in words ("Perícia"). */
export function checkKindLabel(key: string): string {
  const kind = key.split(':', 1)[0];
  return CHECK_KINDS.find((k) => k.kind === kind)?.label ?? 'Teste';
}

/** The line under the title: the check's name, or, for an action with no name
 * of its own (the check is already the title), the kind of check. */
export function actionSubtitle(action: { name: string; checkName: string; key: string }): string {
  return action.name ? action.checkName : checkKindLabel(action.key);
}

/** The DC field's text: empty means no DC (0); otherwise a whole number from 1 to 30. */
export function parseDc(text: string): number | null {
  const value = text.trim();
  if (value === '') {
    return 0;
  }
  if (!/^\d{1,3}$/.test(value)) {
    return null;
  }
  const n = Number(value);
  return n >= SCENE_DC_MIN && n <= SCENE_DC_MAX ? n : null;
}

/**
 * The 18 skills' names, from the rules' content (`ContentService.ListContent`,
 * read once per campaign and kept). The abilities are the sheet's own labels.
 * `providedIn: 'root'`, imported only by lazy code.
 */
@Injectable({ providedIn: 'root' })
export class SceneChecks {
  private readonly client = createClient(ContentService, inject(CONNECT_TRANSPORT));
  private readonly cache = new Map<string, Promise<readonly CheckOption[]>>();

  skills(campaignId: string): Promise<readonly CheckOption[]> {
    let skills = this.cache.get(campaignId);
    if (!skills) {
      skills = this.client.listContent({ campaignId }).then((res) =>
        (res.content?.skills ?? [])
          .map((s) => ({ key: s.key, label: s.namePt }))
          .sort((a, b) => a.label.localeCompare(b.label, 'pt-BR')),
      );
      this.cache.set(campaignId, skills);
      // A failed read is tried again the next time the form opens.
      skills.catch(() => this.cache.delete(campaignId));
    }
    return skills;
  }
}
