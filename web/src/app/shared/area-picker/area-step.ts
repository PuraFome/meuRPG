import { ChangeDetectionStrategy, Component, ElementRef, input, viewChild } from '@angular/core';

/**
 * The heading of a step of the area picker ("1 Onde ela explode", "2 Quem está na área"): the number in a ring and the
 * title, which takes the focus when its step opens (`tabindex="-1"`, no ring drawn for it).
 */
@Component({
  selector: 'app-area-step',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <h3 #heading class="step" tabindex="-1" [id]="titleId() || null">
      <span class="step__n" aria-hidden="true">{{ n() }}</span>
      <span>{{ title() }}</span>
    </h3>
  `,
  styles: `
    :host {
      display: block;
    }

    .step {
      display: flex;
      align-items: center;
      gap: var(--mr-space-2);
      margin: 0;
      font-size: 16px;
      font-weight: 700;
      line-height: 22px;

      &:focus {
        outline: none;
      }
    }

    .step__n {
      display: inline-flex;
      flex: none;
      align-items: center;
      justify-content: center;
      box-sizing: border-box;
      width: 24px;
      height: 24px;
      border: 1.5px solid var(--mr-accent);
      border-radius: 50%;
      color: var(--mr-accent-text);
      font-size: 13px;
    }
  `,
})
export class AreaStep {
  readonly n = input.required<number>();
  readonly title = input.required<string>();
  readonly titleId = input('');

  private readonly heading = viewChild.required<ElementRef<HTMLElement>>('heading');

  focus(): void {
    this.heading().nativeElement.focus();
  }
}
