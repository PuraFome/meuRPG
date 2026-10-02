import {
  applyEdit,
  bodyBytes,
  bytesNotice,
  insertImage,
  insertLink,
  toggleHeading,
  toggleList,
  wrap,
} from './edit-actions';

const ID = '99999999-8888-4777-8666-555555555555';

describe('edit actions', () => {
  it('makes a heading of the line, and takes it off again', () => {
    const t = 'um\ndois';
    const e = toggleHeading(t, 4);
    expect(applyEdit(t, e)).toBe('um\n## dois');
    expect(applyEdit('um\n## dois', toggleHeading('um\n## dois', 6))).toBe('um\ndois');
    expect(applyEdit('# a', toggleHeading('# a', 0))).toBe('## a');
  });

  it('wraps the selection in bold, or leaves a placeholder selected', () => {
    const t = 'uma palavra aqui';
    const e = wrap(t, 4, 11, '**');
    expect(applyEdit(t, e)).toBe('uma **palavra** aqui');
    expect(applyEdit(t, e).slice(e.selStart, e.selEnd)).toBe('palavra');
    const empty = wrap('x ', 2, 2, '*');
    expect(applyEdit('x ', empty)).toBe('x *texto*');
    expect(applyEdit('x ', empty).slice(empty.selStart, empty.selEnd)).toBe('texto');
  });

  it('puts "- " on each line of the selection, and takes it off', () => {
    const t = 'a\nb\n\nc';
    const e = toggleList(t, 0, 3);
    expect(applyEdit(t, e)).toBe('- a\n- b\n\nc');
    const t2 = '- a\n- b';
    expect(applyEdit(t2, toggleList(t2, 0, 7))).toBe('a\nb');
  });

  it('writes a link with the selection as its label, or the name', () => {
    const t = 'veja Mirathel agora';
    const e = insertLink(t, 5, 13, 'Outro nome', 'mapa:abc');
    expect(applyEdit(t, e)).toBe('veja [Mirathel](mapa:abc) agora');
    expect(applyEdit('x ', insertLink('x ', 2, 2, 'Capitão [Goblin]', 'ficha:abc'))).toBe(
      'x [Capitão Goblin](ficha:abc)',
    );
  });

  it('puts an image on a paragraph of its own', () => {
    const t = 'antes depois';
    const e = insertImage(t, 6, 6, 'A Taverna', ID);
    expect(applyEdit(t, e)).toBe(`antes \n\n![A Taverna](imagem:${ID})\n\ndepois`);
    expect(applyEdit('', insertImage('', 0, 0, 'x', ID))).toBe(`![x](imagem:${ID})\n\n`);
    expect(applyEdit('a\n\n', insertImage('a\n\n', 3, 3, 'x', ID))).toBe(`a\n\n![x](imagem:${ID})\n\n`);
  });

  it('counts bytes of UTF-8 after normalising line breaks, and warns from 90%', () => {
    expect(bodyBytes('a\r\nb')).toBe(3);
    expect(bodyBytes('é')).toBe(2);
    expect(bodyBytes('😀')).toBe(4);
    expect(bytesNotice(100)).toBeNull();
    expect(bytesNotice(204_800 * 0.9)).toBe('180 KB de 200 KB');
  });
});
