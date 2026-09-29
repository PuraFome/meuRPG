import { Component } from '@angular/core';
import { MatCardModule } from '@angular/material/card';

/**
 * "Minhas campanhas" (guarded by authGuard — see app.routes.ts).
 *
 * The campaigns module's API isn't there yet, so this is only the
 * placeholder the route needs to exist behind sign-in. It becomes the real
 * campaign list once that module ships.
 */
@Component({
  selector: 'app-campaigns',
  imports: [MatCardModule],
  templateUrl: './campaigns.html',
  styleUrl: './campaigns.scss',
})
export class Campaigns {}
