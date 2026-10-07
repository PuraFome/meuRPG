/**
 * The session link (RN-07, D3): `/campaigns/<id>/session` on this site. It
 * carries no secret: the server decides who gets in, on every call.
 */
export function sessionLink(campaignId: string, origin: string = location.origin): string {
  return `${origin}/campaigns/${campaignId}/session`;
}

/**
 * Copies `text` with the Clipboard API. Resolves `false` when the browser
 * has no Clipboard API or refuses (no permission, an insecure origin): the
 * caller then selects the link in a read-only field for the person to copy
 * by hand.
 */
export async function copyText(text: string): Promise<boolean> {
  try {
    if (!navigator.clipboard?.writeText) {
      return false;
    }
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    return false;
  }
}

/** How long the button says "Link copiado" before it goes back. */
export const COPIED_FOR_MS = 4000;
