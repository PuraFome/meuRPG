// @ts-check
// A spec that changes the server over RPC waits until the page shows the change before its next page action.
//
// Several calls in a row need one wait, after the last: only the last call before an action is reported.
//
// The rule looks at `await <helper>RPC(...)` whose answer carries a numeric `revision` (an `Encounter`): the helper
// says which revision the server reached. After it, in the same block or in the blocks around it, the first thing that
// is either a page action (click, fill, press, ...), a revision wait (`waitForEncounterRevision`) or a page load (`goto`, `openSessionPage`) decides: an action
// first is an error. `goto` and `reload` are not actions: a fresh load reads the state itself.
//
// Limits, on purpose: only Playwright's own action methods count as actions (a helper that clicks inside is not seen);
// a loop's next turn is not followed; the wait is not matched to the page it waits on.

const ACTIONS = new Set([
  'click',
  'dblclick',
  'fill',
  'press',
  'pressSequentially',
  'check',
  'uncheck',
  'setChecked',
  'selectOption',
  'type',
  'hover',
  'focus',
  'tap',
  'dragTo',
  'setInputFiles',
  'clear',
]);
// A wait, or a load: a page that loads after the call reads the state itself.
const WAIT_NAMES = new Set(['waitForEncounterRevision', 'openSessionPage', 'openPlayerMap', 'gotoCampaignTab']);
// A read changes nothing on the server: there is no change for the page to show.
const READS = /^(get|list|read)[A-Z]\w*RPC$/;
const LOAD_METHODS = new Set(['goto', 'reload']);

/** @param {any} node */
const isFunction = (node) =>
  node.type === 'ArrowFunctionExpression' || node.type === 'FunctionDeclaration' || node.type === 'FunctionExpression';

/** @param {any} parent */
const listOf = (parent) => {
  if (parent.type === 'BlockStatement' || parent.type === 'Program' || parent.type === 'StaticBlock') return parent.body;
  if (parent.type === 'SwitchCase') return parent.consequent;
  return null;
};

/** @type {import('eslint').Rule.RuleModule} */
module.exports = {
  meta: {
    type: 'problem',
    schema: [],
    docs: { description: 'Wait for the page to show the revision an RPC returned before acting on the page.' },
    messages: {
      wait: '`{{name}}` returns a revision the page may not show yet: await `waitForEncounterRevision(page, ...)` before the next page action (docs/testing.md).',
    },
  },
  create(context) {
    const sourceCode = context.sourceCode;
    const services = sourceCode.parserServices;
    if (!services?.program || !services.esTreeNodeToTSNodeMap) {
      throw new Error('rpc-revision-wait needs type information (parserOptions.project).');
    }
    const checker = services.program.getTypeChecker();

    /** Whether the awaited value has a numeric `revision`. @param {any} node */
    const hasRevision = (node) => {
      const type = checker.getTypeAtLocation(services.esTreeNodeToTSNodeMap.get(node));
      const prop = type.getProperty('revision');
      return Boolean(prop);
    };

    /** An awaited `*RPC` call whose answer carries a revision. @param {any} node */
    const isTracked = (node) => {
      const call = node.argument;
      return (
        call.type === 'CallExpression' &&
        call.callee.type === 'Identifier' &&
        /RPC$/.test(call.callee.name) &&
        !READS.test(call.callee.name) &&
        hasRevision(node)
      );
    };

    /** First action and first wait inside a statement, by source position (function bodies are not entered). @param {any} root */
    const scan = (root) => {
      let action = Infinity;
      let wait = Infinity;
      let rpc = Infinity;
      /** @param {any} node */
      const walk = (node) => {
        if (!node || typeof node.type !== 'string') return;
        if (isFunction(node)) return;
        if (node.type === 'AwaitExpression' && isTracked(node)) {
          rpc = Math.min(rpc, node.range[0]);
        }
        if (node.type === 'CallExpression') {
          const callee = node.callee;
          if (callee.type === 'MemberExpression' && !callee.computed && ACTIONS.has(callee.property.name)) {
            action = Math.min(action, node.range[0]);
          }
          if (
            (callee.type === 'Identifier' && WAIT_NAMES.has(callee.name)) ||
            (callee.type === 'MemberExpression' && !callee.computed && LOAD_METHODS.has(callee.property.name))
          ) {
            wait = Math.min(wait, node.range[0]);
          }
        }
        const keys = sourceCode.visitorKeys[node.type] ?? [];
        for (const key of keys) {
          const child = node[key];
          if (Array.isArray(child)) child.forEach(walk);
          else walk(child);
        }
      };
      walk(root);
      return { action, wait, rpc };
    };

    return {
      /** @param {any} node */
      AwaitExpression(node) {
        if (!isTracked(node)) return;
        const call = node.argument;
        // `await waitForEncounterRevision(page, await someRPC(...))`: the call is waited on in the same breath.
        for (let up = node.parent; up; up = up.parent) {
          if (up.type === 'CallExpression' && up.callee.type === 'Identifier' && WAIT_NAMES.has(up.callee.name)) return;
          if (isFunction(up)) break;
        }
        /** @type {any} */
        let cur = node;
        while (cur.parent && !isFunction(cur.parent)) {
          const parent = cur.parent;
          const list = listOf(parent);
          if (list) {
            const at = list.indexOf(cur);
            for (const next of list.slice(at + 1)) {
              const { action, wait, rpc } = scan(next);
              // A later call that changes the server before any action takes over: the wait it needs covers this one too.
              if (rpc < action && rpc < wait) return;
              if (wait < action) return;
              if (action < Infinity) {
                context.report({ node: call, messageId: 'wait', data: { name: call.callee.name } });
                return;
              }
              if (wait < Infinity) return;
            }
          }
          cur = parent;
        }
      },
    };
  },
};
