import { expect, type Locator, type Page } from '@playwright/test';

import { callRPC } from './support';

// Helpers for the gallery's tests (MR-019): images built inside the test,
// so no binary fixture lives in the repository, and the upload done the way
// the master does it, through "Enviar imagem".

/**
 * A small real JPEG (64 × 48), drawn by the browser on a canvas. Any page
 * of the app works: the canvas never leaves the page, so the CSP does not
 * care.
 */
export async function canvasJpeg(page: Page, color = '#8a5a2b'): Promise<Buffer> {
  const base64 = await page.evaluate((fill) => {
    const canvas = document.createElement('canvas');
    canvas.width = 64;
    canvas.height = 48;
    const ctx = canvas.getContext('2d')!;
    ctx.fillStyle = fill;
    ctx.fillRect(0, 0, 64, 48);
    ctx.fillStyle = '#e8ddc2';
    ctx.fillRect(8, 8, 24, 16);
    return canvas.toDataURL('image/jpeg', 0.9).split(',')[1];
  }, color);
  return Buffer.from(base64, 'base64');
}

/**
 * The same JPEG with an EXIF block (APP1) right after its start marker,
 * holding a GPS position (23° 33' S, 46° 38' W, São Paulo): what a phone
 * photo carries. The block is a minimal, valid TIFF structure (checked with
 * `exiftool`, which reads "GPS Position: 23 deg 33' 0.00" S, 46 deg 38'
 * 0.00" W"). The server must store the image without it.
 */
export function withExifGps(jpeg: Buffer): Buffer {
  expect(jpeg.subarray(0, 2).equals(Buffer.from([0xff, 0xd8])), 'a JPEG starts with SOI').toBe(true);

  const tiff = Buffer.alloc(128);
  // Header: little-endian ("II"), 42, first IFD at 8.
  tiff.write('II', 0, 'ascii');
  tiff.writeUInt16LE(42, 2);
  tiff.writeUInt32LE(8, 4);
  // IFD0 at 8: one entry, GPSInfo (0x8825, LONG) pointing at the GPS IFD.
  tiff.writeUInt16LE(1, 8);
  entry(tiff, 10, 0x8825, 4, 1, 26);
  tiff.writeUInt32LE(0, 22);
  // GPS IFD at 26: latitude and longitude, with their references.
  tiff.writeUInt16LE(4, 26);
  entry(tiff, 28, 0x0001, 2, 2, Buffer.from('S\0\0\0', 'ascii').readUInt32LE(0));
  entry(tiff, 40, 0x0002, 5, 3, 80);
  entry(tiff, 52, 0x0003, 2, 2, Buffer.from('W\0\0\0', 'ascii').readUInt32LE(0));
  entry(tiff, 64, 0x0004, 5, 3, 104);
  tiff.writeUInt32LE(0, 76);
  // The rationals: 23/1 33/1 0/1 and 46/1 38/1 0/1.
  [23, 33, 0, 46, 38, 0].forEach((value, i) => {
    tiff.writeUInt32LE(value, 80 + i * 8);
    tiff.writeUInt32LE(1, 84 + i * 8);
  });

  const payload = Buffer.concat([Buffer.from('Exif\0\0', 'ascii'), tiff]);
  const header = Buffer.alloc(4);
  header.writeUInt16BE(0xffe1, 0);
  header.writeUInt16BE(payload.length + 2, 2);
  return Buffer.concat([jpeg.subarray(0, 2), header, payload, jpeg.subarray(2)]);
}

function entry(tiff: Buffer, at: number, tag: number, type: number, count: number, value: number): void {
  tiff.writeUInt16LE(tag, at);
  tiff.writeUInt16LE(type, at + 2);
  tiff.writeUInt32LE(count, at + 4);
  tiff.writeUInt32LE(value, at + 8);
}

/** A campaign of the master's own, through the API (the tests here are
 * about the gallery, not about creating campaigns). */
export async function newCampaign(page: Page, name: string): Promise<string> {
  const res = await callRPC(page, 'meurpg.campaigns.v1.CampaignService/CreateCampaign', {
    name,
    xpMode: 'XP_MODE_ENEMIES',
  });
  expect(res.ok()).toBeTruthy();
  return (await res.json()).campaign.id as string;
}

/** Uploads files the way the master does: "Enviar imagem", then the
 * system's file picker. */
export async function uploadThroughPicker(
  page: Page,
  files: { name: string; mimeType: string; buffer: Buffer }[],
): Promise<void> {
  const chooser = page.waitForEvent('filechooser');
  await page.getByRole('button', { name: 'Enviar imagem' }).click();
  await (await chooser).setFiles(files);
}

/** A gallery card, by the image's name (the card is an article labelled by
 * it). */
export function galleryCard(page: Page, name: string): Locator {
  return page.getByRole('article', { name, exact: true });
}

/** The image's ID, read from its card's thumbnail (`/images/<id>/thumb`). */
export async function imageIdOf(card: Locator): Promise<string> {
  const src = await card.getByRole('img').getAttribute('src');
  const id = src?.match(/^\/images\/([^/]+)\/thumb$/)?.[1];
  expect(id, `thumbnail src ${src}`).toBeTruthy();
  return id!;
}
