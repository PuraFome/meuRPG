import { effect, signal } from '@angular/core';

import type { SheetHandle } from './sheet-host';

/**
 * The save of a one-thing sheet: a request in the air locks the sheet, its refusal is said in words and the sheet stays open,
 * its success closes it with `true`. `run` answers the refusal, or an empty text when it is saved.
 * Call it in an injection context (a field of the sheet).
 */
export function saveFlow(sheet: SheetHandle<unknown, boolean>, run: () => Promise<string>) {
  const busy = signal(false);
  const error = signal('');
  // A save in the air cannot be dismissed: its answer is always shown.
  effect(() => sheet.lock(busy()));
  return {
    busy,
    error,
    async save(): Promise<void> {
      if (busy()) {
        return;
      }
      busy.set(true);
      error.set('');
      try {
        const why = await run();
        if (why) {
          error.set(why);
        } else {
          sheet.close(true);
        }
      } finally {
        busy.set(false);
      }
    },
    cancel(): void {
      if (!busy()) {
        sheet.close();
      }
    },
  };
}
