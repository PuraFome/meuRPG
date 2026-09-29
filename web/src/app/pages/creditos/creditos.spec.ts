import { TestBed } from '@angular/core/testing';

import { Creditos, SRD_ATTRIBUTION } from './creditos';

describe('Creditos', () => {
  it('shows the exact CC-BY-4.0 attribution text for the SRD 5.1', () => {
    TestBed.configureTestingModule({ imports: [Creditos] });
    const fixture = TestBed.createComponent(Creditos);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;

    const blockquote = el.querySelector('.srd-attribution');
    expect(blockquote?.textContent?.trim()).toBe(SRD_ATTRIBUTION);
  });

  it('never mentions the "D&D" or "Dungeons & Dragons" trademark', () => {
    TestBed.configureTestingModule({ imports: [Creditos] });
    const fixture = TestBed.createComponent(Creditos);
    fixture.detectChanges();
    const text = (fixture.nativeElement as HTMLElement).textContent ?? '';

    expect(text).not.toContain('D&D');
    expect(text).not.toContain('Dungeons & Dragons');
    expect(text).not.toContain('Dungeons and Dragons');
  });

  it('carries the exact attribution string, unmodified', () => {
    expect(SRD_ATTRIBUTION).toBe(
      'This work includes material taken from the System Reference Document 5.1 ("SRD 5.1") ' +
        'by Wizards of the Coast LLC and available at ' +
        'https://dnd.wizards.com/resources/systems-reference-document. The SRD 5.1 is licensed ' +
        'under the Creative Commons Attribution 4.0 International License available at ' +
        'https://creativecommons.org/licenses/by/4.0/legalcode.',
    );
  });
});
