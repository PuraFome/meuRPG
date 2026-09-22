import type { Request } from 'express';
import type { AuthUser } from './session.guard';

declare global {
  namespace Express {
    interface Request {
      authUser?: AuthUser;
    }
  }
}

export {};