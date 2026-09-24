import { Module } from '@nestjs/common';
import { DatabaseModule } from './database.module';
import { UsersRepository } from './users.repository';
import { AuthTokensRepository } from './auth-tokens.repository';
import { CharactersRepository } from './characters.repository';
import { MapsRepository } from './maps.repository';

@Module({
  imports: [DatabaseModule],
  providers: [
    UsersRepository,
    AuthTokensRepository,
    CharactersRepository,
    MapsRepository,
  ],
  exports: [
    UsersRepository,
    AuthTokensRepository,
    CharactersRepository,
    MapsRepository,
  ],
})
export class RepositoriesModule {}