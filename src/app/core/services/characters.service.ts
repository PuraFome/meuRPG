import { Injectable, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import type { Observable } from 'rxjs';
import { environment } from '../../../environments/environment';
import type { Character } from '../models/character';

export interface JoinTokenInfo {
  token: string;
  expiresAt: string;
  type: 'player';
}

/** Fields the API returns but rejects on create/update. */
const SERVER_MANAGED_FIELDS = ['userId', 'masterUserId', 'createdAt', 'updatedAt'] as const;

@Injectable({
  providedIn: 'root',
})
export class CharactersService {
  private readonly http = inject(HttpClient);
  private readonly base = environment.apiBaseUrl;

  /** List all characters, optionally filtered by type. */
  list(type?: string): Observable<Character[]> {
    const url = type
      ? `${this.base}/api/characters?type=${type}`
      : `${this.base}/api/characters`;
    return this.http.get<Character[]>(url);
  }

  /** Get a single character by id. */
  getById(id: string): Observable<Character> {
    return this.http.get<Character>(`${this.base}/api/characters/${id}`);
  }

  /** Create a new character. */
  create(c: Character): Observable<Character> {
    return this.http.post<Character>(`${this.base}/api/characters`, this.toPayload(c));
  }

  /** Update an existing character. */
  update(id: string, c: Partial<Character>): Observable<Character> {
    return this.http.patch<Character>(`${this.base}/api/characters/${id}`, this.toPayload(c));
  }

  /** Delete a character. */
  remove(id: string): Observable<void> {
    return this.http.delete<void>(`${this.base}/api/characters/${id}`);
  }

  /** Create a join token for sharing a player character. */
  createJoinToken(): Observable<JoinTokenInfo> {
    return this.http.post<JoinTokenInfo>(`${this.base}/api/characters/join-tokens`, {});
  }

  /** Validate a join token. */
  validateJoinToken(token: string): Observable<{ valid: boolean; expiresAt: string }> {
    return this.http.get<{ valid: boolean; expiresAt: string }>(
      `${this.base}/api/characters/join/${token}`,
    );
  }

  /** Join via token, creating a player character. */
  join(token: string, c: Character): Observable<Character> {
    return this.http.post<Character>(
      `${this.base}/api/characters/join/${token}`,
      this.toPayload(c),
    );
  }

  /**
   * The server DTO is whitelisted (forbidNonWhitelisted), so server-managed
   * fields must be stripped: they come back on every API read (and live in the
   * store), are omitted from the create/update DTOs, and would otherwise
   * trigger a 400 Bad Request. Ownership (`userId`, `masterUserId`) is set by
   * the server from the session and must never be sent by the client.
   */
  private toPayload(c: Character | Partial<Character>): unknown {
    const payload: Record<string, unknown> = { ...c };
    for (const key of SERVER_MANAGED_FIELDS) {
      delete payload[key];
    }
    return payload;
  }
}
