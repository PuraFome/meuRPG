import { MATERIAL_ANIMATIONS } from '@angular/material/core';

/**
 * Providers every unit test gets (the `providersFile` of the `test` target in angular.json).
 * Material animations are off: overlays, dialogs and menus then open and close at once, so a
 * spec never waits real time for an animation to end (that wait was what timed out on a loaded
 * machine). Nothing in a spec asserts on an animation.
 */
export default [{ provide: MATERIAL_ANIMATIONS, useValue: { animationsDisabled: true } }];
