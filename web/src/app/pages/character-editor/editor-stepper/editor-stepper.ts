import { NgTemplateOutlet } from '@angular/common';
import {
  Component,
  ElementRef,
  QueryList,
  ViewChildren,
  afterNextRender,
  inject,
  input,
  Injector,
  signal,
} from '@angular/core';
import { CdkStepHeader, CdkStepper } from '@angular/cdk/stepper';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

/**
 * The editor's stepper, in the "ficha de papel" look: a CDK stepper (the
 * headless base `MatStepper` itself extends) with its own header, so the
 * layout is ours instead of Material's internals.
 *
 * - Each step header is a real `role="tab"` button (`CdkStepHeader`), named
 *   by the step's label, with the base class's keyboard handling (arrows,
 *   Home/End, Enter/Space). Non-linear: any step can be opened at any time.
 * - Desktop (720px and up): numbered tabs with their labels, in a row.
 *   Narrower: the same tabs as numbers only (44px targets, the label stays
 *   the tab's accessible name), and each step opens with "Passo 2 de 5"
 *   over its title, so a phone never scrolls sideways.
 * - A step's content is built the first time the step opens and then stays in the
 *   DOM (hidden when not selected), so what the person did in it (a placed roll, a
 *   search) survives a trip to another step. The form controls live in the page's
 *   `FormGroup`, not in the DOM, so a step never opened still has its values, and
 *   its panel (the tab's `aria-controls`) is always there. A step child that
 *   writes something the save depends on (the table's ways of making scores
 *   sets `incomplete`) must be listed in `eager`, or it is not there to say so
 *   until someone opens the step. Building five steps at
 *   once made every open of the editor, and every spec of it, pay for four steps
 *   nobody had looked at yet.
 * - Each step ends with "previous/next step" buttons; moving with them
 *   focuses the new step's title, which scrolls it into view.
 *
 * Use with `<cdk-step label="...">` children; `[hasError]` on a step marks
 * its tab with an error icon and "(com erro)".
 */
@Component({
  selector: 'app-editor-stepper',
  imports: [NgTemplateOutlet, CdkStepHeader, MatButtonModule, MatIconModule],
  templateUrl: './editor-stepper.html',
  styleUrl: './editor-stepper.scss',
  providers: [{ provide: CdkStepper, useExisting: EditorStepper }],
})
export class EditorStepper extends CdkStepper {
  /** The headers live in this component's own view, not in its content, so
   * the base class's key manager reads them from a view query (the same
   * override `MatStepper` makes). */
  @ViewChildren(CdkStepHeader) override _stepHeader: QueryList<CdkStepHeader> = undefined!;

  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);

  /** Selects step `index` and moves focus to its title (the "Anterior" /
   * "Próximo" buttons, and a submit that jumps to a step with an error). */
  goTo(index: number): void {
    if (index < 0 || index >= this.steps.length) {
      return;
    }
    this.selectedIndex = index;
    afterNextRender(
      () => {
        const title = this.host.nativeElement.querySelector<HTMLElement>(
          `#${this._getStepContentId(index)} .stepper__title`,
        );
        title?.focus();
      },
      { injector: this.injector },
    );
  }

  /** Steps (by label) whose content is built from the start, without waiting to be opened: one whose content
   * decides something the page needs before the save, such as "Habilidades" under the table's ways of making
   * scores (an unplaced roll must stop the save even if nobody opened the step). */
  readonly eager = input<readonly string[]>([]);

  /** The steps opened so far: their content is built and kept. The first is open from the start. */
  private readonly visited = signal<ReadonlySet<number>>(new Set([0]));

  constructor() {
    super();
    // Every way of opening a step (a tab, "Próximo", a refused save) ends in `selectionChange`.
    this.selectionChange.subscribe((e) => this.visited.update((v) => (v.has(e.selectedIndex) ? v : new Set([...v, e.selectedIndex]))));
  }

  /** Whether step `index` has its content: it was opened at least once, or it is eager. */
  protected rendered(index: number): boolean {
    return this.visited().has(index) || this.eager().includes(this.labelAt(index));
  }

  protected labelAt(index: number): string {
    return this.steps.get(index)?.label ?? '';
  }
}
