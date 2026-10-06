/** A `data:` URL for a small PNG the server sent as bytes (the drawing of a request): the app's CSP allows images from `data:`. */
export function pngDataUrl(bytes: Uint8Array, contentType = 'image/png'): string {
  let binary = '';
  const chunk = 0x8000;
  for (let i = 0; i < bytes.length; i += chunk) {
    binary += String.fromCharCode(...bytes.subarray(i, i + chunk));
  }
  return `data:${contentType};base64,${btoa(binary)}`;
}

/** A key for one try of a request (1 to 64 characters): a retry with the same key never generates twice. */
export function newRequestKey(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  return `k-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`;
}
