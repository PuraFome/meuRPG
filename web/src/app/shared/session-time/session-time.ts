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
