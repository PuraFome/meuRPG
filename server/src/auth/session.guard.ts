import { Injectable, CanActivate, ExecutionContext } from '@nestjs/common';
import type { Request } from 'express';
import { AuthTokensRepository } from '../db/auth-tokens.repository';
import { extractBearerToken } from './bearer';

export interface AuthUser {
  sub: string;
  email: string;
  name: string;
}

export class SessionGuard implements CanActivate {
  constructor(private readonly tokens: AuthTokensRepository) {}

  async canActivate(context: ExecutionContext): Promise<boolean> {
    const request = context.switchToHttp().getRequest<Request>();
    const token = extractBearerToken(request);
    if (!token) {
      return false;
    }
    const user = await this.tokens.findUserByToken(token);
    if (!user) {
      return false;
    }
    request.authUser = user;
    return true;
  }
}