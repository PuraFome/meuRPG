import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { CdkStep } from '@angular/cdk/stepper';

import { EditorStepper } from './editor-stepper';

@Component({
  imports: [EditorStepper, CdkStep],
  template: `
    <app-editor-stepper>
      <cdk-step label="Básico" [hasError]="basicoHasError()"><p>conteúdo básico</p></cdk-step>
      <cdk-step label="Habilidades"><p>conteúdo habilidades</p></cdk-step>
      <cdk-step label="Equipamento"><p>conteúdo equipamento</p></cdk-step>
    </app-editor-stepper>
  `,
})
class Host {
  readonly basicoHasError = signal(false);
}

describe('EditorStepper', () => {
  async function render() {
    TestBed.configureTestingModule({ imports: [Host] });
    const fixture = TestBed.createComponent(Host);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const tabs = () => Array.from(el.querySelectorAll<HTMLButtonElement>('[role="tab"]'));
    const panels = () => Array.from(el.querySelectorAll<HTMLElement>('[role="tabpanel"]'));
    return { fixture, el, tabs, panels };
  }

  it('renders one tab per step, named by its label, the first one selected', async () => {
    const { tabs, panels } = await render();

    expect(tabs().map((t) => t.textContent?.replace(/\s+/g, ' ').trim())).toEqual([
      '1 Básico',
      '2 Habilidades',
      '3 Equipamento',
    ]);
    expect(tabs().map((t) => t.getAttribute('aria-selected'))).toEqual(['true', 'false', 'false']);
    // Every step's content is in the DOM; only the selected one shows.
    expect(panels().map((p) => p.hidden)).toEqual([false, true, true]);
    expect(panels()[0].getAttribute('aria-labelledby')).toBe(tabs()[0].id);
  });

  it('opens each step with "Passo N de M" and its title', async () => {
    const { panels } = await render();

    expect(panels()[1].querySelector('.stepper__count')?.textContent).toContain('Passo 2 de 3');
    expect(panels()[1].querySelector('h2')?.textContent).toContain('Habilidades');
  });

  it('moves with the tabs and with the previous/next buttons', async () => {
    const { fixture, tabs, panels } = await render();

    tabs()[2].click();
    fixture.detectChanges();
    expect(panels().map((p) => p.hidden)).toEqual([true, true, false]);

    const prev = panels()[2].querySelector<HTMLButtonElement>('.stepper__prev');
    expect(prev?.textContent).toContain('Passo anterior: Habilidades');
    expect(panels()[2].querySelector('.stepper__next')).toBeNull();
    prev?.click();
    fixture.detectChanges();
    expect(tabs()[1].getAttribute('aria-selected')).toBe('true');

    const next = panels()[1].querySelector<HTMLButtonElement>('.stepper__next');
    expect(next?.textContent).toContain('Próximo passo: Equipamento');
  });

  it('marks a step with an error in words, not only in colour', async () => {
    const { fixture, tabs } = await render();

    fixture.componentInstance.basicoHasError.set(true);
    fixture.detectChanges();

    expect(tabs()[0].textContent).toContain('(com erro)');
    expect(tabs()[1].textContent).not.toContain('(com erro)');
  });
});
