import type { GalleryUsage } from '../../../gen/meurpg/maps/v1/gallery_pb';

/** The shape of `GalleryImage` these helpers read (the generated message
 * fits it), so a caller can pass a plain object in a test. */
export interface ImageSize {
  readonly width: number;
  readonly height: number;
  readonly byteSize: number;
}

const MB = 1024 * 1024;
const KB = 1024;
/** A no-break space: a number never ends a line without its unit. */
const NBSP = '\u00a0';

/**
 * A byte count the way the screens show it, in Portuguese: "1,5 MB",
 * "500 MB", "38 KB" (with a no-break space before the unit). The server counts in MiB (10 MiB per image, 500 MiB
 * per campaign) and the product calls them MB, so 1 MB here is 1024 × 1024
 * bytes: "10 MB" on screen is exactly the server's limit.
 */
export function formatBytes(bytes: number): string {
  if (bytes < 0.1 * MB) {
    // Small images (a token, an icon) in whole KB, never "0,0 MB".
    return `${Math.max(1, Math.round(bytes / KB))}${NBSP}KB`;
  }
  // One decimal, dropped when it is ",0" ("6 MB", not "6,0 MB") and from
  // 100 MB up.
  const mb = Math.round((bytes / MB) * 10) / 10;
  const digits = mb >= 100 || Number.isInteger(mb) ? 0 : 1;
  return `${mb.toLocaleString('pt-BR', { minimumFractionDigits: digits, maximumFractionDigits: digits })}${NBSP}MB`;
}

/** "2000 × 1400 px", kept on one line. */
export function formatDimensions(image: Pick<ImageSize, 'width' | 'height'>): string {
  return `${image.width}${NBSP}×${NBSP}${image.height}${NBSP}px`;
}

/** The card's second line: "2000 × 1400 px, 1,5 MB". */
export function imageMeta(image: ImageSize): string {
  return `${formatDimensions(image)}, ${formatBytes(image.byteSize)}`;
}

/** "1 imagem" / "5 imagens". */
export function imageCountLabel(count: number): string {
  return count === 1 ? '1 imagem' : `${count} imagens`;
}

/** The gallery's quota line: "5 imagens · 5,8 MB de 500 MB" (E5-20). */
export function usageLine(usage: GalleryUsage, separator = ' · '): string {
  return `${imageCountLabel(usage.imageCount)}${separator}${formatBytes(usage.byteCount)} de ${formatBytes(usage.maxBytes)}`;
}

/** From 90% of either limit (images or bytes), the quota line turns into a
 * warning (README-B, "Quota line"). */
export function quotaNearlyFull(usage: GalleryUsage): boolean {
  const byCount = usage.maxImages > 0 && usage.imageCount >= 0.9 * usage.maxImages;
  const byBytes = usage.maxBytes > 0 && usage.byteCount >= 0.9 * usage.maxBytes;
  return byCount || byBytes;
}

/** "A", "A e B", "A, B e C": names in a sentence, in Portuguese. */
export function joinNames(names: readonly string[]): string {
  if (names.length <= 1) {
    return names[0] ?? '';
  }
  return `${names.slice(0, -1).join(', ')} e ${names[names.length - 1]}`;
}

/** The card's use line (E5-20): "Usada em Mirathel e arredores" or "Ainda
 * não usada", from `GalleryImage.used_in_maps`. */
export function usedInLine(maps: readonly { readonly name: string }[]): string {
  return maps.length === 0 ? 'Ainda não usada' : `Usada em ${joinNames(maps.map((m) => m.name))}`;
}
