## Task 3 Decisions
1. StoreService: BehaviorSubject + Map per collection, not module-scoped, providedIn: root
2. Repositories: plain classes (not @Injectable) instantiated per-collection with key
3. PersistenceService: subscribes to store events$, auto-persists to localStorage
4. Routes: loadChildren ? feature .routes.ts ? loadComponent within
5. Testing: @testing-library/angular render() needs routes + initialRoute for RouterOutlet
6. No NgRx/Signals — pure BehaviorSubject + RxJS
