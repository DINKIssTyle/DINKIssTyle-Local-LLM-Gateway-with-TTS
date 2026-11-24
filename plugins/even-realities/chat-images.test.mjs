import assert from 'node:assert/strict';
import test from 'node:test';
import {chatImageContainers, fitChatImage, splitChatImages} from './chat-images.mjs';

const screenshot = suffix => '/api/computer/screenshots/0123456789ab-' + suffix.repeat(24) + '.png';

test('extracts only protected Gateway screenshots, deduplicating and limiting the preview', () => {
  const result = splitChatImages([
    'Answer',
    `![first](${screenshot('a')}) ![duplicate](${screenshot('a')})`,
    `![second](${screenshot('b')}) ![third](${screenshot('c')})`,
    '![external](https://example.invalid/x.png)',
    '![relative](/api/computer/screenshots/invalid.png)',
  ].join('\n'));
  assert.deepEqual(result.paths, [screenshot('a'), screenshot('b')]);
  assert.match(result.text, /^Answer/);
  assert.match(result.text, /https:\/\/example\.invalid/);
  assert.doesNotMatch(result.text, /third/);
});

test('letterboxes captures within SDK dimensions and centers one or two containers', () => {
  assert.deepEqual(fitChatImage(1920, 1080), {x: 16, y: 0, width: 256, height: 144});
  assert.deepEqual(fitChatImage(720, 1440), {x: 108, y: 0, width: 72, height: 144});
  assert.deepEqual(fitChatImage(2400, 600), {x: 0, y: 36, width: 288, height: 72});
  assert.deepEqual(chatImageContainers(1).map(c => c.xPosition), [144]);
  assert.deepEqual(chatImageContainers(2).map(c => [c.xPosition, c.width, c.height]), [[0, 288, 144], [288, 288, 144]]);
});
