import { DOCUMENT, DestroyRef, Injector, inject } from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';
import { ActivatedRoute, Router } from '@angular/router';

import { AuthService } from '../../core/auth/auth.service';
import { CastingClient } from '../../core/casting/casting-client';
import { CombatClient } from '../../core/combat/combat-client';
import { SpellCatalog } from '../../core/combat/spell-catalog';
import { TurnAlert } from '../../core/combat/turn-alert';
import { MapsClient } from '../../core/maps/maps-client';
import { NotesClient } from '../../core/notes/notes-client';
import { FamiliarEyesClient } from '../../core/play/familiar-eyes';
import { LiveSessionSource } from '../../core/play/live-session.types';
import { OpenSessions } from '../../core/play/open-sessions';
import { SceneClient } from '../../core/play/scene-client';
import { XpChanges } from '../../core/progression/xp-changes';
import { PuzzlesClient } from '../../core/puzzles/puzzles-client';
import { TrapsClient } from '../../core/traps/traps-client';
import { SessionNotes } from '../../shell/session-notes/session-notes';

/** What the session page injects, handed to the plain classes it builds (they are not in the injection context). */
export interface LiveSessionServices {
  readonly source: LiveSessionSource;
  readonly auth: AuthService;
  readonly xpChanges: XpChanges;
  readonly spellCatalog: SpellCatalog;
  readonly router: Router;
  readonly route: ActivatedRoute;
  readonly document: Document;
  readonly dialog: MatDialog;
  readonly bottomSheet: MatBottomSheet;
  readonly mapsApi: MapsClient;
  readonly combatApi: CombatClient;
  readonly castingApi: CastingClient;
  readonly sceneApi: SceneClient;
  readonly notesApi: NotesClient;
  readonly trapsApi: TrapsClient;
  readonly puzzlesApi: PuzzlesClient;
  readonly sessionNotes: SessionNotes;
  readonly openSessions: OpenSessions;
  readonly destroyRef: DestroyRef;
  /** The adjust sheet needs this route's `LiveSessionSource`: MatDialog and MatBottomSheet live in the root injector,
   * which doesn't have it. */
  readonly injector: Injector;
  readonly turnAlert: TurnAlert;
  readonly familiarEyes: FamiliarEyesClient;
}

/** Reads every service of the page; call it in an injection context (a field initializer of the component). */
export function injectLiveSessionServices(): LiveSessionServices {
  return {
    source: inject(LiveSessionSource),
    auth: inject(AuthService),
    xpChanges: inject(XpChanges),
    spellCatalog: inject(SpellCatalog),
    router: inject(Router),
    route: inject(ActivatedRoute),
    document: inject(DOCUMENT),
    dialog: inject(MatDialog),
    bottomSheet: inject(MatBottomSheet),
    mapsApi: inject(MapsClient),
    combatApi: inject(CombatClient),
    castingApi: inject(CastingClient),
    sceneApi: inject(SceneClient),
    notesApi: inject(NotesClient),
    trapsApi: inject(TrapsClient),
    puzzlesApi: inject(PuzzlesClient),
    sessionNotes: inject(SessionNotes),
    openSessions: inject(OpenSessions),
    destroyRef: inject(DestroyRef),
    injector: inject(Injector),
    turnAlert: inject(TurnAlert),
    familiarEyes: inject(FamiliarEyesClient),
  };
}
