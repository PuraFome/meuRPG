import { TestBed } from '@angular/core/testing';

import { DescriptionLanguage, chooseText } from './description-language';

describe('DescriptionLanguage', () => {
  it('starts in Portuguese and flips for the whole app, with no Web Storage', () => {
    const local = vi.spyOn(Storage.prototype, 'setItem');
    const service = TestBed.inject(DescriptionLanguage);
    expect(service.english()).toBe(false);
    service.toggle();
    expect(service.english()).toBe(true);
    expect(TestBed.inject(DescriptionLanguage)).toBe(service);
    service.toggle();
    expect(service.english()).toBe(false);
    expect(local).not.toHaveBeenCalled();
    local.mockRestore();
  });
});

describe('chooseText', () => {
  const both = { pt: ['um'], en: ['one'], ptMissing: false };

  it('gives Portuguese by default and English when asked, with the toggle on', () => {
    expect(chooseText(both, false)).toEqual({
      paragraphs: ['um'],
      lang: 'pt',
      missing: false,
      canToggle: true,
    });
    expect(chooseText(both, true)).toEqual({
      paragraphs: ['one'],
      lang: 'en',
      missing: false,
      canToggle: true,
    });
  });

  it('falls back to the English, flagged, when the Portuguese is missing (no toggle)', () => {
    const missing = { pt: [], en: ['one'], ptMissing: true };
    expect(chooseText(missing, false)).toEqual({
      paragraphs: ['one'],
      lang: 'en',
      missing: true,
      canToggle: false,
    });
    expect(chooseText({ ...both, pt: [] }, false).missing).toBe(true);
  });

  it('keeps a Portuguese-only text in Portuguese whatever the choice', () => {
    const ours = { pt: ['um'], en: ['um'], ptMissing: false, ptOnly: true };
    expect(chooseText(ours, true)).toEqual({
      paragraphs: ['um'],
      lang: 'pt',
      missing: false,
      canToggle: false,
    });
  });
});
