import { NgTemplateOutlet } from '@angular/common';
import {
  Component,
  ElementRef,
  QueryList,
  ViewChildren,
  afterNextRender,
  inject,
  Injector,
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
 * - Every step's content stays in the DOM (hidden when not selected), like
 *   `MatStepper`: the form controls of all steps exist from the start.
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

  protected labelAt(index: number): string {
    return this.steps.get(index)?.label ?? '';
  }
}
