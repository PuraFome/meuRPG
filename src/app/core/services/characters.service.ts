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
    return this.http.post<Character>(`${this.base}/api/characters`, c);
  }

  /** Update an existing character. */
  update(id: string, c: Partial<Character>): Observable<Character> {
    return this.http.patch<Character>(`${this.base}/api/characters/${id}`, c);
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
    return this.http.post<Character>(`${this.base}/api/characters/join/${token}`, c);
  }
}
