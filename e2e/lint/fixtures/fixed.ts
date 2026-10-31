import { combatRPC, waitForEncounterRevision, type Page } from './support';

export async function clickRightAfter(m: Page): Promise<void> {
  const enc = await combatRPC(m, 'EndTurn');
  await waitForEncounterRevision(m, enc);
  await m.getByRole('button').click();
}

export async function clickInOuterBlock(m: Page, ok: boolean): Promise<void> {
  let enc = await combatRPC(m, 'Start');
  if (ok) {
    enc = await combatRPC(m, 'EndTurn');
  }
  await waitForEncounterRevision(m, enc);
  await m.getByRole('button').click();
}

// Several calls in a row need one wait, after the last.
export async function waitTooLate(m: Page): Promise<void> {
  await combatRPC(m, 'Start');
  const enc = await combatRPC(m, 'EndTurn');
  await waitForEncounterRevision(m, enc);
  await m.getByRole('button').click();
}
