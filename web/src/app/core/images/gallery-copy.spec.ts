import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import { ImageInUseSchema } from '../../../gen/meurpg/maps/v1/gallery_pb';
import { deleteRefusal, imageNameError, renameErrorMessage } from './gallery-copy';

describe('imageNameError', () => {
  it('accepts 1 to 80 characters on one line, spaces trimmed', () => {
    expect(imageNameError('Taverna do Javali')).toBeNull();
    expect(imageNameError('  Taverna  ')).toBeNull();
    expect(imageNameError('a'.repeat(80))).toBeNull();
  });

  it('says what is wrong otherwise', () => {
    expect(imageNameError('')).toBe('Dê um nome à imagem.');
    expect(imageNameError('   ')).toBe('Dê um nome à imagem.');
    expect(imageNameError('a'.repeat(81))).toBe('Use até 80 caracteres.');
    expect(imageNameError('Taverna\ndo Javali')).toBe('Use um nome numa linha só.');
  });
});

describe('renameErrorMessage', () => {
  it('maps the codes RenameGalleryImage returns', () => {
    expect(renameErrorMessage(new ConnectError('x', Code.InvalidArgument))).toBe(
      'Use um nome de 1 a 80 caracteres, numa linha só.',
    );
    expect(renameErrorMessage(new ConnectError('x', Code.NotFound))).toContain(
      'não está mais na galeria',
    );
    expect(renameErrorMessage(new TypeError('fetch failed'))).toContain('Tente de novo');
  });
});

describe('deleteRefusal with the ImageInUse detail', () => {
  function inUse(...names: string[]): ConnectError {
    return new ConnectError('in use', Code.FailedPrecondition, undefined, [
      {
        desc: ImageInUseSchema,
        value: create(ImageInUseSchema, { maps: names.map((name, i) => ({ id: `m${i}`, name })) }),
      },
    ]);
  }

  it('names the map that uses the image', () => {
    expect(deleteRefusal(inUse('Mirathel e arredores'))).toEqual({
      gone: false,
      message:
        'Essa imagem é o fundo de Mirathel e arredores. Troque a imagem do mapa antes de apagá-la.',
    });
  });

  it('names every map when there are several', () => {
    const refusal = deleteRefusal(inUse('A', 'B', 'C'));
    expect(refusal.gone === false && refusal.message).toBe(
      'Essa imagem é o fundo dos mapas A, B e C. Troque a imagem deles antes de apagá-la.',
    );
  });
});

describe('deleteRefusal', () => {
  it('says a map uses the image, even without a detail naming it', () => {
    expect(deleteRefusal(new ConnectError('in use', Code.FailedPrecondition))).toEqual({
      gone: false,
      message: 'Essa imagem está num mapa. Troque a imagem do mapa antes de apagá-la.',
    });
  });

  it('treats an image already deleted as gone', () => {
    expect(deleteRefusal(new ConnectError('x', Code.NotFound))).toEqual({ gone: true });
  });

  it('falls back to the server message for anything else', () => {
    const refusal = deleteRefusal(new ConnectError('x', Code.Unavailable));
    expect(refusal.gone).toBe(false);
    expect(refusal.gone === false && refusal.message).toContain('Tente de novo');
  });
});
