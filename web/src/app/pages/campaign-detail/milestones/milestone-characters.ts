import { Component, input } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { RouterLink } from '@angular/router';

import type { ExperienceRow } from '../../../core/progression/experience-store';
import { LevelUpTag } from '../../../shared/xp/level-up-tag';

/**
 * "Personagens" of a milestones campaign (E8-14): each living player
 * character with the tag "Pode subir de nível" (an arrow and the words) or
 * "Sem marco novo". The master reads the whole group. A player reads only
 * their own character (everyone's marks are in "Marcos alcançados", but the
 * rest of the group's sheets are not theirs to list) and, when it can level
 * up, has "Abrir a ficha": a 48px button the width of the row on a phone.
 * The host shows this only after the first milestone: before it there is
 * nothing to say about anyone.
 */
@Component({
  selector: 'app-milestone-characters',
  imports: [LevelUpTag, MatButtonModule, RouterLink],
  template: `
    <ul class="list">
      @for (r of rows(); track r.id) {
        <li class="row">
          <span class="who">
            <span class="who__name">{{ r.name }}</span>
            <span class="who__sub">{{ r.sub }}</span>
          </span>
          @if (r.canLevelUp) {
            <app-level-up-tag class="state" />
          } @else {
            <span class="state state--none">Sem marco novo</span>
          }
          @if (openable() && r.canLevelUp) {
            <a mat-stroked-button class="open" [routerLink]="['/campaigns', campaignId(), 'characters', r.id]">
              Abrir a ficha
            </a>
          }
        </li>
      }
    </ul>
  `,
  styleUrl: './milestone-characters.scss',
})
export class MilestoneCharacters {
  readonly campaignId = input.required<string>();
  readonly rows = input.required<readonly ExperienceRow[]>();
  /** Show "Abrir a ficha" on a row that can level up (the player's own). */
  readonly openable = input(false);

}
