import { Injectable, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import type { Observable } from 'rxjs';
import { environment } from '../../../environments/environment';
import type { MapData } from '../models/map';

@Injectable({
  providedIn: 'root',
})
export class MapsService {
  private readonly http = inject(HttpClient);
  private readonly base = environment.apiBaseUrl;

  /** List all maps owned by the current user. */
  list(): Observable<MapData[]> {
    return this.http.get<MapData[]>(`${this.base}/api/maps`);
  }

  /** Create a new map. */
  create(map: MapData): Observable<MapData> {
    return this.http.post<MapData>(`${this.base}/api/maps`, this.toPayload(map));
  }

  /** Update an existing map. */
  update(id: string, map: Partial<MapData>): Observable<MapData> {
    return this.http.patch<MapData>(
      `${this.base}/api/maps/${id}`,
      this.toPayload(map),
    );
  }

  /** Delete a map. */
  remove(id: string): Observable<void> {
    return this.http.delete<void>(`${this.base}/api/maps/${id}`);
  }

  /**
   * The server DTO is whitelisted (forbidNonWhitelisted), so client-managed
   * timestamps must be stripped: they are omitted from the create/update DTOs
   * and would otherwise trigger a 400 Bad Request.
   */
  private toPayload(map: MapData | Partial<MapData>): unknown {
    const payload: Partial<MapData> = { ...map };
    delete payload.createdAt;
    delete payload.updatedAt;
    return payload;
  }
}
