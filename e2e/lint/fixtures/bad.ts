import { combatRPC, getEncounterRPC, rollSceneRPC, waitForEncounterRevision, type Page } from './support';

// Three errors: a click straight after the call, a click in an outer block, and a wait that comes too late.
export async function clickRightAfter(m: Page): Promise<void> {
  await combatRPC(m, 'EndTurn');
  await m.getByRole('button').click();
}

export async function clickInOuterBlock(m: Page, ok: boolean): Promise<void> {
  if (ok) {
    await combatRPC(m, 'EndTurn');
  }
  await m.getByRole('button').click();
}

export async function waitTooLate(m: Page): Promise<void> {
  const enc = await combatRPC(m, 'EndTurn');
  await m.getByRole('button').click();
  await waitForEncounterRevision(m, enc);
}

// Not errors: a read, a void call (no revision to wait for), a load after the call.
export async function notTracked(m: Page): Promise<void> {
  await getEncounterRPC(m);
  await m.getByRole('button').click();
  await rollSceneRPC(m);
  await m.getByRole('button').click();
  await combatRPC(m, 'EndTurn');
  await m.goto('/');
  await m.getByRole('button').click();
}
