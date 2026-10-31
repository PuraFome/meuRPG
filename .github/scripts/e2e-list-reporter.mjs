// A Playwright reporter for `playwright test --list`: prints the tests as JSON, with the serial group of each one.
//
//   playwright test --list --reporter=<this file>
//
// The JSON reporter does not say which tests are in a `test.describe.configure({ mode: 'serial' })` group, and a group
// must run whole in one shard (docs/ci.md, "End to end"). The mode is read from Playwright's suite tree
// (`_parallelMode`, not public API); a Playwright that no longer has it fails the run instead of splitting groups.
import process from 'node:process';

export default class ListReporter {
  printsToStdio() {
    return true;
  }

  onBegin(_config, root) {
    const tests = [];
    const walk = (suite, group) => {
      if (suite.type === 'describe') {
        if (!('_parallelMode' in suite)) throw new Error('Playwright no longer has Suite._parallelMode: update .github/scripts/e2e-list-reporter.mjs');
        // The outermost serial describe is the group; the ones inside it belong to it.
        if (group === null && suite._parallelMode === 'serial') group = suite.titlePath().join(' › ');
      }
      for (const child of suite.suites) walk(child, group);
      for (const test of suite.tests) {
        const [, project, file, ...titles] = test.titlePath();
        tests.push({ project, file, titles, group });
      }
    };
    walk(root, null);
    process.stdout.write(JSON.stringify({ tests }) + '\n');
  }
}
