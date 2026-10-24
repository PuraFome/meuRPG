import { ChangeDetectionStrategy, Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { settle } from '../../../../test-settle';
import { PanelHead } from './panel-head';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [PanelHead],
  template: `
    <app-panel-head
      icon="group"
      title="Membros"
      titleId="members-heading"
      [count]="count"
      [foldable]="foldable()"
      [(open)]="open"
    >
      <span class="extra">1 pendente</span>
    </app-panel-head>
  `,
})
class Host {
  count = '4 na mesa';
  readonly foldable = signal(false);
  readonly open = signal(false);
}

describe('PanelHead', () => {
  function setup() {
    const fixture = TestBed.createComponent(Host);
    fixture.detectChanges();
    return { fixture, host: fixture.componentInstance, el: fixture.nativeElement as HTMLElement };
  }

  it("draws the icon (decoration), the title as the panel's h2, the count and what is projected", () => {
    const { el } = setup();
    const icon = el.querySelector('mat-icon')!;
    expect(icon.textContent).toBe('group');
    expect(icon.getAttribute('aria-hidden')).toBe('true');
    const title = el.querySelector('h2')!;
    expect(title.id).toBe('members-heading');
    expect(title.textContent?.trim()).toBe('Membros');
    expect(el.querySelector('.head__count')?.textContent).toBe('4 na mesa');
    expect(el.querySelector('.extra')?.textContent).toBe('1 pendente');
    expect(el.querySelector('button')).toBeNull();
  });

  it('folds: the title and its count become a button that says if the panel is open, and opens it', async () => {
    const { fixture, host, el } = setup();
    host.foldable.set(true);
    await settle(fixture);
    const button = el.querySelector<HTMLButtonElement>('h2 button')!;
    expect(button.textContent?.replace(/\s+/g, ' ').trim()).toBe('Membros · 4 na mesa');
    expect(button.getAttribute('aria-expanded')).toBe('false');
    // The count is in the button's words, not twice.
    expect(el.querySelector('.head__count')).toBeNull();
    // What is projected stays beside it.
    expect(el.querySelector('.extra')).not.toBeNull();

    button.click();
    await settle(fixture);
    expect(host.open()).toBe(true);
    expect(button.getAttribute('aria-expanded')).toBe('true');
  });
});
