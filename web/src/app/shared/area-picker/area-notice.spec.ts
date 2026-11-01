import { TestBed } from '@angular/core/testing';

import { AreaNotice } from './area-notice';

describe('AreaNotice', () => {
  it('draws the warning with no live role: the screen puts it in a status region', () => {
    const fixture = TestBed.createComponent(AreaNotice);
    fixture.componentRef.setInput('notice', {
      strong: 'Toren está na área.',
      rest: 'Mude o local.',
    });
    fixture.detectChanges();
    const notice = (fixture.nativeElement as HTMLElement).querySelector('.mr-notice--warning')!;
    expect(notice.hasAttribute('role')).toBe(false);
    expect(notice.textContent).toContain('Toren está na área. Mude o local.');
  });
});
