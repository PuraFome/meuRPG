/** The "Sessão" menu of the session bar lives in the overlay: these open it and find its items for the specs. */
export function openSessionMenu(root: HTMLElement): void {
  root.querySelector<HTMLButtonElement>('.bar__menu')!.click();
}

/** An item of the open menu, by its words. */
export function sessionMenuItem(text: string): HTMLButtonElement | undefined {
  return Array.from(document.querySelectorAll<HTMLButtonElement>('.mat-mdc-menu-item')).find((b) =>
    b.textContent?.includes(text),
  );
}
