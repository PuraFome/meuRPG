import { Code, ConnectError } from '@connectrpc/connect';

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
