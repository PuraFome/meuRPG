const pad = (n: number) => String(n).padStart(2, '0');

/**
 * "30/09 às 20:05", in local time: when a live session started (the
 * campaign's "Sessão" panel, the master's status line). No year: an open
 * session is from today or yesterday. The spaces around "às" don't break.
 */
export function formatDayAt(date: Date): string {
  return `${pad(date.getDate())}/${pad(date.getMonth() + 1)} às ${formatClock(date)}`;
}

/** "21:14", in local time ("Última atualização às 21:14"). */
export function formatClock(date: Date): string {
  return `${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

/** "Em andamento desde 20:05" for a session that started today, with the day
 * ("… desde 30/09 às 20:05") when it started on another one (timeline.md,
 * shared decision 8): the line under "Sessão 5" for the master and the
 * players alike. */
export function sessionSince(startedAt: Date, now: Date = new Date()): string {
  const today =
    startedAt.getFullYear() === now.getFullYear() &&
    startedAt.getMonth() === now.getMonth() &&
    startedAt.getDate() === now.getDate();
  return `Em andamento desde ${today ? formatClock(startedAt) : formatDayAt(startedAt)}`;
}
