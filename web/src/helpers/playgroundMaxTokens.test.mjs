import assert from 'node:assert/strict';

import {
  normalizeMaxTokensValue,
  normalizePlaygroundInputValue,
  sanitizePlaygroundInputs,
} from './playgroundMaxTokens.js';

assert.equal(normalizeMaxTokensValue(8192), 8192);
assert.equal(normalizeMaxTokensValue('8192'), 8192);
assert.equal(normalizeMaxTokensValue(' 8192 '), 8192);
assert.equal(normalizeMaxTokensValue(''), null);
assert.equal(normalizeMaxTokensValue('abc'), null);
assert.equal(normalizeMaxTokensValue(-1), null);
assert.equal(normalizeMaxTokensValue(1.9), 1);

assert.equal(normalizePlaygroundInputValue('max_tokens', '2048'), 2048);
assert.equal(normalizePlaygroundInputValue('max_tokens', 'bad'), null);
assert.equal(normalizePlaygroundInputValue('seed', 'bad'), 'bad');

assert.deepEqual(
  sanitizePlaygroundInputs({
    model: 'gpt-4o',
    max_tokens: '2048',
  }),
  {
    model: 'gpt-4o',
    max_tokens: 2048,
  },
);

assert.deepEqual(
  sanitizePlaygroundInputs({
    model: 'gpt-4o',
    max_tokens: 'bad',
  }),
  {
    model: 'gpt-4o',
    max_tokens: null,
  },
);

console.log('playground max_tokens tests passed');
