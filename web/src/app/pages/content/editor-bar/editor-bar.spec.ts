import { TestBed } from '@angular/core/testing';

import { EntrySaver } from '../../../core/content/entry-saver';
import { EditorAlerts } from './editor-bar';

describe('EditorAlerts: the stale entry', () => {
  it('is said by an alert region that was there first, the notice without a role', () => {
    const saver = new EntrySaver({ aOne: 'uma magia' }, 'a magia');
    const fixture = TestBed.createComponent(EditorAlerts);
    fixture.componentRef.setInput('saver', saver);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const live = el.querySelector('.mr-live[role="alert"]')!;
    expect(live.textContent?.trim()).toBe('');
    saver.stale.set(true);
    fixture.detectChanges();
    const notice = live.querySelector('.mr-notice--danger')!;
    expect(notice.textContent).toContain('Esta entrada mudou enquanto você editava.');
    expect(notice.hasAttribute('role')).toBe(false);
  });
});
