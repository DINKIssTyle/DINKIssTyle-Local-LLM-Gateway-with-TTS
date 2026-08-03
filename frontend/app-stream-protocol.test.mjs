import assert from 'node:assert/strict';
import test from 'node:test';

globalThis.window = globalThis;
await import('./app-stream-protocol.js');

const { SSEParser } = globalThis.DKSTStreamProtocol;
const encoder = new TextEncoder();

test('parses a CRLF boundary split across byte chunks', () => {
    const parser = new SSEParser();
    assert.deepEqual(parser.parse(encoder.encode('event: message.delta\r\ndata: {"content":"안녕"}\r')), []);
    assert.deepEqual(parser.parse(encoder.encode('\n\r\n')), [{ type: 'message.delta', content: '안녕' }]);
});

test('accepts mixed line endings at an event boundary', () => {
    const parser = new SSEParser();
    assert.deepEqual(parser.parse(encoder.encode('data: {"type":"message.delta","content":"mixed"}\n\r\n')), [
        { type: 'message.delta', content: 'mixed' }
    ]);
});

test('joins multiline data fields and preserves JSON newlines', () => {
    const parser = new SSEParser();
    const events = parser.parse(encoder.encode('data: {"content":"line 1",\ndata: "type":"message.delta"}\n\n'));
    assert.deepEqual(events, [{ content: 'line 1', type: 'message.delta' }]);
});

test('flushes a valid final event without a trailing blank line', () => {
    const parser = new SSEParser();
    parser.parse(encoder.encode('data: {"type":"message.delta","content":"끝"}'));
    assert.deepEqual(parser.finish(), [{ type: 'message.delta', content: '끝' }]);
});

test('turns malformed non-error data into a canonical parse error', () => {
    const parser = new SSEParser({ parseErrorMessage: '스트림 오류' });
    const [event] = parser.parse(encoder.encode('data: not-json\n\n'));
    assert.equal(event.type, 'stream.parse_error');
    assert.equal(event.error.message, '스트림 오류');
    assert.equal(event.error.detail, 'not-json');
});

test('keeps a plain error event as an error instead of a parse error', () => {
    const parser = new SSEParser();
    assert.deepEqual(parser.parse(encoder.encode('event: error\ndata: upstream failed\n\n')), [
        { type: 'error', error: { message: 'upstream failed' } }
    ]);
});

test('normalizes a JSON named error event to the canonical error field', () => {
    const parser = new SSEParser();
    assert.deepEqual(parser.parse(encoder.encode('event: error\ndata: {"message":"upstream failed"}\n\n')), [
        { type: 'error', message: 'upstream failed', error: { message: 'upstream failed' } }
    ]);
});
