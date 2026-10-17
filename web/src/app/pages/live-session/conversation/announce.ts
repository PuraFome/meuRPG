import type { SessionMessage } from '../../../../gen/meurpg/play/v1/session_messages_pb';

/**
 * The lines that came at the end since `previous`: what a screen reader is told.
 * The first read, older lines put before ("Carregar anteriores") and a line that
 * slots into the middle by time are history, not news, and are never announced.
 */
export function newTail(
  previous: readonly SessionMessage[],
  next: readonly SessionMessage[],
): SessionMessage[] {
  const last = previous.at(-1);
  if (!last) {
    return [];
  }
  const at = next.findIndex((m) => m.id === last.id);
  return at < 0 ? [] : next.slice(at + 1);
}
