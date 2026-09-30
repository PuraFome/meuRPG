import type { GalleryUsage } from '../../../gen/meurpg/maps/v1/gallery_pb';
import {
  formatBytes,
  formatDimensions,
  imageCountLabel,
  imageMeta,
  quotaNearlyFull,
  usageLine,
} from './image-format';

const MB = 1024 * 1024;
/** The helpers keep numbers and units together with no-break spaces;
 * these tests read them as plain spaces. */
const plain = (text: string) => text.replace(/\u00a0/g, ' ');

function usage(partial: Partial<GalleryUsage> = {}): GalleryUsage {
  return {
    $typeName: 'meurpg.maps.v1.GalleryUsage',
    imageCount: 5,
    maxImages: 300,
    byteCount: Math.round(5.8 * MB),
    maxBytes: 500 * MB,
    maxImageBytes: 10 * MB,
    ...partial,
  };
}

describe('formatBytes', () => {
  it('writes MB with a decimal comma, and whole numbers without one', () => {
    expect(plain(formatBytes(1.5 * MB))).toBe('1,5 MB');
    expect(plain(formatBytes(0.4 * MB))).toBe('0,4 MB');
    expect(plain(formatBytes(10 * MB))).toBe('10 MB');
    expect(plain(formatBytes(500 * MB))).toBe('500 MB');
  });

  it('keeps the number and its unit on one line', () => {
    expect(formatBytes(10 * MB)).toBe('10\u00a0MB');
    expect(formatDimensions({ width: 800, height: 1000 })).toBe('800\u00a0×\u00a01000\u00a0px');
  });

  it('writes small images in KB, never "0,0 MB"', () => {
    expect(plain(formatBytes(49 * 1024))).toBe('49 KB');
    expect(plain(formatBytes(200))).toBe('1 KB');
  });
});

describe('image lines', () => {
  it('writes the size in pixels and the card line', () => {
    expect(plain(formatDimensions({ width: 2000, height: 1400 }))).toBe('2000 × 1400 px');
    expect(plain(imageMeta({ width: 2000, height: 1400, byteSize: 1.5 * MB }))).toBe(
      '2000 × 1400 px, 1,5 MB',
    );
  });

  it('counts images in Portuguese, singular and plural', () => {
    expect(imageCountLabel(1)).toBe('1 imagem');
    expect(imageCountLabel(0)).toBe('0 imagens');
    expect(imageCountLabel(5)).toBe('5 imagens');
  });

  it('writes the quota line as E5-20 and E5-09 draw it', () => {
    expect(plain(usageLine(usage()))).toBe('5 imagens · 5,8 MB de 500 MB');
    expect(plain(usageLine(usage(), ', '))).toBe('5 imagens, 5,8 MB de 500 MB');
  });
});

describe('quotaNearlyFull', () => {
  it('turns on from 90% of either limit', () => {
    expect(quotaNearlyFull(usage())).toBe(false);
    expect(quotaNearlyFull(usage({ imageCount: 269 }))).toBe(false);
    expect(quotaNearlyFull(usage({ imageCount: 270 }))).toBe(true);
    expect(quotaNearlyFull(usage({ byteCount: 450 * MB }))).toBe(true);
  });
});
