import {
  Injectable,
  CanActivate,
  ExecutionContext,
  UnauthorizedException,
} from '@nestjs/common';
import type { Request } from 'express';
import { AuthTokensRepository } from '../db/auth-tokens.repository';
import { extractBearerToken } from './bearer';

export interface AuthUser {
  id: string;
  sub: string;
  email: string;
  name: string;
}

@Injectable()
export class SessionGuard implements CanActivate {
  constructor(private readonly tokens: AuthTokensRepository) {}

  async canActivate(context: ExecutionContext): Promise<boolean> {
    const request = context.switchToHttp().getRequest<Request>();
    const token = extractBearerToken(request);
    if (!token) {
      throw new UnauthorizedException();
    }
    const user = await this.tokens.findUserByToken(token);
    if (!user) {
      throw new UnauthorizedException();
    }
    request.authUser = user;
    return true;
  }
}
