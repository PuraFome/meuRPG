import { plain } from './gallery-testing';
import {
  DEFAULT_LIMITS,
  type UploadFailureKind,
  precheckImageFile,
  uploadFailureFromResponse,
  uploadFailureMessage,
} from './upload-errors';

const MB = 1024 * 1024;
const emptyGallery = { imageCount: 0, byteCount: 0, ...DEFAULT_LIMITS };

describe('uploadFailureFromResponse', () => {
  it('reads the reason first (gallery.proto, POST /uploads/images)', () => {
    const cases: [number, string, UploadFailureKind][] = [
      [
        400,
        '{"code":"invalid_argument","reason":"UNSUPPORTED_TYPE","message":"x"}',
        'UNSUPPORTED_TYPE',
      ],
      [413, '{"code":"invalid_argument","reason":"TOO_LARGE","message":"x"}', 'TOO_LARGE'],
      [400, '{"code":"invalid_argument","reason":"DIMENSIONS","message":"x"}', 'DIMENSIONS'],
      [400, '{"code":"invalid_argument","reason":"CORRUPT","message":"x"}', 'CORRUPT'],
      [
        400,
        '{"code":"invalid_argument","reason":"MALFORMED_REQUEST","message":"x"}',
        'MALFORMED_REQUEST',
      ],
      [429, '{"code":"resource_exhausted","reason":"QUOTA","message":"x"}', 'QUOTA'],
      [429, '{"code":"resource_exhausted","reason":"RATE_LIMITED","message":"x"}', 'RATE_LIMITED'],
    ];
    for (const [status, body, kind] of cases) {
      expect(uploadFailureFromResponse(status, body)).toBe(kind);
    }
  });

  it('falls back to the Connect code, then the HTTP status, never to the message', () => {
    expect(uploadFailureFromResponse(403, '{"code":"permission_denied","message":"CORRUPT"}')).toBe(
      'PERMISSION_DENIED',
    );
    expect(uploadFailureFromResponse(404, '{"code":"not_found"}')).toBe('NOT_FOUND');
    expect(uploadFailureFromResponse(401, '{"code":"unauthenticated"}')).toBe('UNAUTHENTICATED');
    expect(uploadFailureFromResponse(503, '{"code":"unavailable"}')).toBe('UNAVAILABLE');
    // The CSRF guard's plain-text 403, and a proxy's HTML page.
    expect(uploadFailureFromResponse(403, 'cross-origin request detected')).toBe(
      'PERMISSION_DENIED',
    );
    expect(uploadFailureFromResponse(502, '<html>Bad gateway</html>')).toBe('UNKNOWN');
    // A reason the app does not know yet.
    expect(uploadFailureFromResponse(400, '{"code":"invalid_argument","reason":"NEW_RULE"}')).toBe(
      'UNKNOWN',
    );
  });
});

describe('uploadFailureMessage', () => {
  it('says each refusal in Portuguese, with the server limits', () => {
    expect(plain(uploadFailureMessage('UNSUPPORTED_TYPE'))).toBe(
      'Esse arquivo não é uma imagem JPEG, PNG ou WebP.',
    );
    expect(plain(uploadFailureMessage('TOO_LARGE'))).toBe('A imagem passa de 10 MB.');
    expect(plain(uploadFailureMessage('DIMENSIONS'))).toBe(
      'A imagem é grande demais (mais de 8192 pixels de lado ou 40 megapixels).',
    );
    expect(plain(uploadFailureMessage('CORRUPT'))).toBe(
      'Não deu para ler essa imagem. Ela pode estar corrompida.',
    );
    expect(plain(uploadFailureMessage('QUOTA'))).toBe(
      'A galeria está cheia: 300 imagens ou 500 MB.',
    );
    expect(plain(uploadFailureMessage('NETWORK'))).toBe(
      'A conexão caiu durante o envio. Tente de novo.',
    );
  });

  it('follows the limits the server sent', () => {
    const limits = { maxImages: 100, maxBytes: 200 * MB, maxImageBytes: 5 * MB };
    expect(plain(uploadFailureMessage('TOO_LARGE', limits))).toBe('A imagem passa de 5 MB.');
    expect(plain(uploadFailureMessage('QUOTA', limits))).toBe(
      'A galeria está cheia: 100 imagens ou 200 MB.',
    );
  });

  it('never shows an internal name', () => {
    const kinds: UploadFailureKind[] = [
      'MALFORMED_REQUEST',
      'UNKNOWN',
      'UNAVAILABLE',
      'PERMISSION_DENIED',
    ];
    for (const kind of kinds) {
      expect(uploadFailureMessage(kind)).not.toMatch(/[A-Z]{2,}_/);
    }
  });
});

describe('precheckImageFile', () => {
  const file = (type: string, size = 1000) => ({ type, size });

  it('accepts JPEG, PNG and WebP, and leaves an unknown type to the server', () => {
    for (const type of ['image/jpeg', 'image/png', 'image/webp', '']) {
      expect(precheckImageFile(file(type), emptyGallery)).toBeNull();
    }
  });

  it('refuses GIF, SVG and anything else by type', () => {
    for (const type of ['image/gif', 'image/svg+xml', 'text/plain', 'application/pdf']) {
      expect(precheckImageFile(file(type), emptyGallery)).toBe('UNSUPPORTED_TYPE');
    }
  });

  it('refuses a file over the per-image limit', () => {
    expect(precheckImageFile(file('image/jpeg', 10 * MB), emptyGallery)).toBeNull();
    expect(precheckImageFile(file('image/jpeg', 10 * MB + 1), emptyGallery)).toBe('TOO_LARGE');
  });

  it('refuses by count (counting the files already in line), and by bytes only when already full', () => {
    const almost = { ...emptyGallery, imageCount: 298 };
    expect(precheckImageFile(file('image/png'), almost, 1)).toBeNull();
    expect(precheckImageFile(file('image/png'), almost, 2)).toBe('QUOTA');
    // 499 MB used: a 5 MB file may still fit once encoded again, so the
    // server decides.
    expect(
      precheckImageFile(file('image/png', 5 * MB), { ...emptyGallery, byteCount: 499 * MB }),
    ).toBeNull();
    expect(precheckImageFile(file('image/png'), { ...emptyGallery, byteCount: 500 * MB })).toBe(
      'QUOTA',
    );
  });
});
