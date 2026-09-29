import { TestBed } from '@angular/core/testing';

import { Campaigns } from './campaigns';

describe('Campaigns', () => {
  it('shows the "coming soon" placeholder', () => {
    TestBed.configureTestingModule({ imports: [Campaigns] });
    const fixture = TestBed.createComponent(Campaigns);
    fixture.detectChanges();

    const text = (fixture.nativeElement as HTMLElement).textContent ?? '';
    expect(text).toContain('Minhas campanhas');
    expect(text).toContain('Em breve');
  });
});
