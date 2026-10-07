import { Component, computed, input } from '@angular/core';

type Shape = 'bird' | 'dog' | 'paw' | 'person' | 'giant' | 'spider' | 'other';

const BIRDS = new Set([
  'raven',
  'hawk',
  'owl',
  'crow',
  'eagle',
  'giant-eagle',
  'giant-owl',
  'vulture',
  'blood-hawk',
  'swarm-of-ravens',
  'giant-vulture',
]);
const DOGS = new Set(['wolf', 'dire-wolf', 'mastiff', 'jackal', 'worg', 'winter-wolf']);

/**
 * A creature's picture where the app has none: a flat silhouette (a bird, a dog's
 * head, a paw for the rest) in a rounded tile, as the artboards draw it. It
 * stands for the creature's icon or a gallery image, which the app does not
 * have for the SRD's 334 creatures. Decorative: the name is always beside it.
 */
@Component({
  selector: 'app-creature-art',
  template: `
    <svg viewBox="0 0 32 32" aria-hidden="true" focusable="false" [attr.data-shape]="shape()">
      @switch (shape()) {
        @case ('bird') {
          <path d="M5 14 1.5 21l9-1.2z" />
          <ellipse cx="15" cy="17" rx="8.2" ry="5.4" />
          <circle cx="22.5" cy="11.5" r="3.7" />
          <path d="M25.6 10.6 31 12.2l-5.4 1.6z" />
          <path d="M12.6 21.5v6.5m4.6-6.5V28" class="art__stroke" />
        }
        @case ('dog') {
          <path d="M5 3.5 11 9h10l6-5.5 1.5 11.5-3.5 4.2V25l-5 4h-9l-5-4v-6.3L3.5 15z" />
          <path d="M11 16.5h2v2h-2zm8 0h2v2h-2zM14.5 22h3l-1.5 2z" class="art__hole" />
        }
        @case ('person') {
          <circle cx="16" cy="9" r="5" />
          <path d="M5 29c0-7 4.5-11.5 11-11.5S27 22 27 29z" />
        }
        @case ('giant') {
          <circle cx="16" cy="7" r="4.4" />
          <path d="M4 30V19c0-4 3-6.5 7-6.5h10c4 0 7 2.5 7 6.5v11h-6.5v-8h-1.5v8h-8v-8h-1.5v8z" />
        }
        @case ('spider') {
          <ellipse cx="16" cy="19" rx="5.4" ry="6.4" />
          <circle cx="16" cy="10.5" r="3.2" />
          <path d="M11 16 3 11m8 8-9 1m10 3-7 7m16-14 8-5m-8 8 9 1m-10 3 7 7" class="art__stroke" />
        }
        @case ('other') {
          <path d="M16 2.5 28 9.5v13L16 29.5 4 22.5v-13z" />
          <path d="M16 10v8m0 3v1.5" class="art__hole art__stroke--hole" />
        }
        @default {
          <ellipse cx="16" cy="21" rx="6.4" ry="5.2" />
          <ellipse cx="7" cy="14" rx="2.7" ry="3.4" />
          <ellipse cx="13" cy="8.8" rx="2.7" ry="3.5" />
          <ellipse cx="19.6" cy="8.8" rx="2.7" ry="3.5" />
          <ellipse cx="25.6" cy="14" rx="2.7" ry="3.4" />
        }
      }
    </svg>
  `,
  styles: `
    :host {
      display: inline-flex;
      flex: none;
      align-items: center;
      justify-content: center;
      box-sizing: border-box;
      width: var(--art-size, 40px);
      height: var(--art-size, 40px);
      border: 1px solid var(--mr-line);
      border-radius: var(--mr-radius-md);
      background: var(--mr-ground);
      color: var(--mr-ink);
    }

    svg {
      width: 68%;
      height: 68%;
      fill: currentColor;
    }

    .art__stroke {
      fill: none;
      stroke: currentColor;
      stroke-width: 1.6;
      stroke-linecap: round;
    }

    .art__hole {
      fill: var(--mr-ground);
    }

    .art__stroke--hole {
      fill: none;
      stroke: var(--mr-ground);
      stroke-width: 2.4;
      stroke-linecap: round;
    }
  `,
})
export class CreatureArt {
  /** "monster:raven" (or "raven"). */
  readonly monsterKey = input('');
  /**
   * The SRD type ("humanoid", "beast"...): the bestiary picks the shape by it, so a person is a
   * person and a creature of an unusual type gets a neutral glyph, never a paw. Unset (a
   * familiar, a Wild Shape form: always a beast) keeps the bird, the dog and the paw.
   */
  readonly type = input('');

  protected readonly shape = computed<Shape>(() => {
    const key = this.monsterKey().replace(/^monster:/, '');
    const type = this.type();
    // By key first: a wolf is a wolf even as the Werewolf's wolf form (a humanoid), and a wolf spider is a spider.
    if (key.includes('spider')) {
      return 'spider';
    }
    if (DOGS.has(key) || /(^|-)wolf(-|$)/.test(key)) {
      return 'dog';
    }
    if (BIRDS.has(key)) {
      return 'bird';
    }
    if (type === '' || type === 'beast') {
      return 'paw';
    }
    return type === 'humanoid' ? 'person' : type === 'giant' ? 'giant' : 'other';
  });
}
