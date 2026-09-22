import { Module } from '@nestjs/common';
import { DatabaseModule } from './database.module';
import { UsersRepository } from './users.repository';
import { AuthTokensRepository } from './auth-tokens.repository';

@Module({
  imports: [DatabaseModule],
  providers: [UsersRepository, AuthTokensRepository],
  exports: [UsersRepository, AuthTokensRepository],
})
export class RepositoriesModule {}