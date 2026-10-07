import { Component, DestroyRef, afterNextRender, inject, input, signal } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/** One link of the list of sections: the id of its panel (`sec-<id>`), its word, and whether a refusal is inside it. */
export interface NavSection {
  readonly id: string;
  readonly label: string;
  readonly issue: boolean;
}

/**
 * The list of sections of a long editor page (E10-02 state 1): at the side from 1100 px, a strip of links across the top below
 * it. It marks the section in view (`aria-current="location"`, found with an IntersectionObserver as the page scrolls, never
 * only by the last click) and says "Com erro" on a section a refusal is in. It claims nothing else: no ✓ for "complete".
 * The page keeps it in view while it scrolls (the parent's `position: sticky`).
 */
@Component({
  selector: 'app-section-nav',
  imports: [MatIconModule],
  template: `
    <nav class="sections" [attr.aria-label]="label()">
      <ul>
        @for (s of sections(); track s.id) {
          <li>
            <button type="button" class="link" [class.link--on]="active() === s.id" [attr.aria-current]="active() === s.id ? 'location' : null" (click)="go(s.id)">
              @if (s.issue) {
                <mat-icon class="bad" aria-hidden="true">warning</mat-icon>
                <span class="mr-visually-hidden">Com erro: </span>
              }
              {{ s.label }}
            </button>
          </li>
        }
      </ul>
    </nav>
  `,
  styles: `
    :host {
      display: block;
      min-width: 0;
    }

    .sections {
      border: 1px solid var(--mr-line);
      border-radius: var(--mr-radius-md);
      background: var(--mr-surface);
    }

    ul {
      display: flex;
      gap: 4px;
      margin: 0;
      padding: 6px;
      list-style: none;
      overflow-x: auto;

      @media (min-width: 1100px) {
        flex-direction: column;
        overflow: visible;
      }
    }

    .link {
      display: flex;
      align-items: center;
      gap: var(--mr-space-2);
      width: 100%;
      min-height: 44px;
      padding: 0 var(--mr-space-3);
      border: 2px solid transparent;
      border-radius: var(--mr-radius-sm);
      background: transparent;
      color: var(--mr-ink);
      font: inherit;
      white-space: nowrap;
      cursor: pointer;

      &--on {
        border-color: var(--mr-accent);
        background: var(--mr-accent-soft);
        font-weight: 700;
      }

      &:focus-visible {
        outline: 2px solid var(--mr-focus);
        outline-offset: 2px;
      }
    }

    .bad {
      flex: none;
      width: 18px;
      height: 18px;
      font-size: 18px;
      color: var(--mr-danger-ink);
    }
  `,
})
export class SectionNav {
  private readonly destroyRef = inject(DestroyRef);

  readonly sections = input.required<readonly NavSection[]>();
  readonly label = input('Seções');
  readonly active = signal('');

  constructor() {
    afterNextRender(() => {
      this.active.set(this.sections()[0]?.id ?? '');
      if (typeof IntersectionObserver === 'undefined') {
        return;
      }
      // A band near the top of the screen: the section that crosses it is the one being read.
      const observer = new IntersectionObserver(
        (entries) => {
          for (const e of entries) {
            if (e.isIntersecting) {
              this.active.set(e.target.id.replace(/^sec-/, ''));
            }
          }
        },
        { rootMargin: '-25% 0px -65% 0px' },
      );
      for (const s of this.sections()) {
        const el = document.getElementById(`sec-${s.id}`);
        if (el) observer.observe(el);
      }
      // At the foot of the page the last section never reaches the band: it is the one in view.
      const atFoot = (): void => {
        if (window.innerHeight + window.scrollY >= document.documentElement.scrollHeight - 2) {
          const last = this.sections().at(-1);
          if (last) this.active.set(last.id);
        }
      };
      window.addEventListener('scroll', atFoot, { passive: true });
      this.destroyRef.onDestroy(() => {
        observer.disconnect();
        window.removeEventListener('scroll', atFoot);
      });
    });
  }

  protected go(id: string): void {
    this.active.set(id);
    const el = document.getElementById(`sec-${id}`);
    el?.scrollIntoView?.({ block: 'start', behavior: 'auto' });
    el?.focus({ preventScroll: true });
  }
}
