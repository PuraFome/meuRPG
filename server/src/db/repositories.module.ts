import { Module } from '@nestjs/common';
import { DatabaseModule } from './database.module';
import { UsersRepository } from './users.repository';
import { AuthTokensRepository } from './auth-tokens.repository';
import { CharactersRepository } from './characters.repository';

@Module({
  imports: [DatabaseModule],
  providers: [UsersRepository, AuthTokensRepository, CharactersRepository],
  exports: [UsersRepository, AuthTokensRepository, CharactersRepository],
})
export class RepositoriesModule {}