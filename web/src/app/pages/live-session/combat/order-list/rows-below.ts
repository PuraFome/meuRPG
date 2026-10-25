/** How many rows of a scrolling list lie wholly below the room it has: "Mais 6 abaixo". */
export function rowsBelow(el: HTMLElement | undefined): number {
  if (!el || el.scrollHeight <= el.clientHeight + 1) {
    return 0;
  }
  const bottom = el.scrollTop + el.clientHeight;
  return Array.from(el.querySelectorAll<HTMLElement>('li.row')).filter(
    (row) => row.offsetTop - el.offsetTop >= bottom,
  ).length;
}
