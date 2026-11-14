import { propertyClash } from './weapon-draft';

const p = (...keys: string[]) => keys.map((k) => `weapon-property:${k}`);

describe('the property combinations a weapon never has (#580)', () => {
  it('says nothing for a combination the SRD has', () => {
    expect(propertyClash(false, p('finesse', 'light', 'thrown'))).toBe('');
  });
  it.each([
    [false, p('loading'), 'Recarga só vale para uma arma com Munição.'],
    [
      false,
      p('ammunition'),
      'Munição é de arma à distância: uma arma corpo a corpo arremessada usa Arremesso.',
    ],
    [true, p('reach'), 'Alcance é de arma corpo a corpo.'],
    [
      false,
      p('two-handed', 'versatile'),
      'Versátil já diz que a arma vai em uma ou duas mãos: tire Duas mãos ou tire Versátil.',
    ],
    [
      false,
      p('two-handed', 'light'),
      'Leve é de arma usada em cada mão: ela não pode exigir Duas mãos.',
    ],
  ])('says it in Portuguese (%s, %j)', (ranged, props, text) => {
    expect(propertyClash(ranged, props)).toBe(text);
  });
});
