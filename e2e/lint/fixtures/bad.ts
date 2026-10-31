import { combatRPC, getEncounterRPC, rollSceneRPC, waitForEncounterRevision, type Page } from './support';

// Four errors: a click straight after the call, a click in an outer block, a wait that comes too late, and a wait on
// the master's page before a click on the player's.
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

export async function waitsOnTheOtherPage(m: Page, p: Page): Promise<void> {
  const enc = await combatRPC(m, 'EndTurn');
  await waitForEncounterRevision(m, enc);
  await p.getByRole('button').click();
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
  // A move is patched into the page with no new revision: there is nothing to wait for.
  await combatRPC(m, 'MoveCombatant');
  await m.getByRole('button').click();
}
