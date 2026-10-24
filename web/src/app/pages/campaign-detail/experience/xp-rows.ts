import { Component, computed, input } from '@angular/core';

import { formatInt, tight } from '../../../core/format/text';
import type { ExperienceRow } from '../../../core/progression/experience-store';
import { progress } from '../../../core/progression/xp-labels';
import { LevelUpTag } from '../../../shared/xp/level-up-tag';

/**
 * Each living player character's XP on the campaign page (E7-09, boards C2-01 and C2-02): on a
 * line the name, "nível 3", a bar towards the next level and the number ("2.150 XP"; a screen
 * reader hears "2.150 de 2.700 XP"), then, under it, either "Pode subir de nível" (the tag, with
 * its icon and words) or what is missing ("Faltam 334 XP"). The bar is never the only sign: the
 * numbers say the same. In a campaign that levels by milestones there is no number, only the tag.
 */
@Component({
  selector: 'app-xp-rows',
  imports: [LevelUpTag],
  templateUrl: './xp-rows.html',
  styleUrl: './xp-rows.scss',
})
export class XpRows {
  readonly rows = input.required<readonly ExperienceRow[]>();
  readonly milestones = input(false);
  /** What each character just got (by id): "+105 XP", in place of "Faltam…", only right after an award. */
  readonly gained = input<ReadonlyMap<string, number>>(new Map());

  protected readonly lines = computed(() =>
    this.rows().map((r) => {
      const gain = this.gained().get(r.id) ?? 0;
      return {
        row: r,
        progress: progress(r.xp, r.nextLevelXp, r.canLevelUp),
        xp: formatInt(r.xp),
        /** What the next level asks, said only to a screen reader ("2.150 de 3.000 XP"); empty at the top level. */
        next: r.nextLevelXp > 0 ? formatInt(r.nextLevelXp) : '',
        gain: gain > 0 ? tight(`+${formatInt(gain)} XP`) : '',
      };
    }),
  );
}
