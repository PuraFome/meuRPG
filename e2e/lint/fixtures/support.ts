// Just the shapes the rule reads: a helper that answers with a revisioned Encounter, one that answers with nothing.
export interface Encounter {
  id: string;
  revision: number;
}
interface Locator {
  click(): Promise<void>;
}
export interface Page {
  getByRole(role: string): Locator;
  goto(url: string): Promise<void>;
}
export declare function combatRPC(page: Page, method: string): Promise<Encounter>;
export declare function getEncounterRPC(page: Page): Promise<Encounter>;
export declare function rollSceneRPC(page: Page): Promise<void>;
export declare function waitForEncounterRevision(page: Page, expected: Encounter): Promise<void>;
