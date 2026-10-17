/** Where one write stands: not started, in flight, or refused with the message to show. */
export type SavingState =
  { status: 'idle' } | { status: 'saving' } | { status: 'error'; message: string };

export const IDLE_SAVING: SavingState = { status: 'idle' };
