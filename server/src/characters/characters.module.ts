import { Module } from '@nestjs/common';
import { RepositoriesModule } from '../db/repositories.module';
import { CharactersController } from './characters.controller';

@Module({
  imports: [RepositoriesModule],
  controllers: [CharactersController],
})
export class CharactersModule {}
