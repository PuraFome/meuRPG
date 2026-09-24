import { Module } from '@nestjs/common';
import { AuthController } from './auth.controller';
import { AuthTokensRepository } from '../db/auth-tokens.repository';
import { UsersRepository } from '../db/users.repository';
import { DatabaseModule } from '../db/database.module';

@Module({
  imports: [DatabaseModule],
  controllers: [AuthController],
  providers: [AuthTokensRepository, UsersRepository],
})
export class AuthModule {}