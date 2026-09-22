import type { Request } from 'express';

export function extractBearerToken(request: Request): string | null {
  const authorization = request.headers.authorization;
  if (!authorization) {
    return null;
  }
  const match = authorization.match(/^Bearer\s+(.+)$/i);
  return match ? match[1] : null;
}