## Task 3: Core Infrastructure

### Created

**Models** – character.ts, campaign.ts, gallery.ts, map.ts, session.ts, rules.ts, shared.ts + barrel export (src/app/core/models/)

**Store** – src/app/core/store/store.service.ts: generic StoreService<T> with BehaviorSubject map, events$ Subject, methods get/getAll/set/update/patch/delete/subscribe/snapshot

**Repositories** – base-repository.ts (interface), local-storage-repository.ts (Generic JSON serializer), indexed-db-file-repository.ts (binary files via IndexedDB)

**Services** – search.service.ts (BehaviorSubject-based index), sidebar.service.ts (sidebar items state), persistence.service.ts (store ? localStorage sync + search index)

**Routes** – app.routes.ts: / (HomeComponent), /personagens, /campanha, /galeria, /regras, /sessao, /mapa (all lazy via loadChildren), ** redirect

**Updated** – app.config.ts (provideRouter with withComponentInputBinding + withRouterConfig), app.ts (RouterOutlet)

### Verification
- npx ng build: SUCCESS (6 lazy chunks)
- npx vitest run: PASS (2/2)
