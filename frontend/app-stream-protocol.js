/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

(function attachDKSTStreamProtocol(global) {
    function findEventBoundary(buffer) {
        const source = String(buffer || '');
        const lineEndingLengthAt = (index) => {
            if (source[index] === '\r') return source[index + 1] === '\n' ? 2 : 1;
            return source[index] === '\n' ? 1 : 0;
        };

        for (let index = 0; index < source.length; index += 1) {
            const firstLength = lineEndingLengthAt(index);
            if (!firstLength) continue;
            const secondLength = lineEndingLengthAt(index + firstLength);
            if (secondLength) {
                return { index, length: firstLength + secondLength };
            }
            index += firstLength - 1;
        }
        return null;
    }

    function parseEventBlock(rawBlock, parseErrorMessage) {
        const lines = String(rawBlock || '').split(/\r\n|\n|\r/);
        let eventName = 'message';
        const dataLines = [];

        for (const line of lines) {
            if (!line || line.startsWith(':')) continue;

            const separator = line.indexOf(':');
            const field = separator >= 0 ? line.slice(0, separator) : line;
            let value = separator >= 0 ? line.slice(separator + 1) : '';
            if (value.startsWith(' ')) value = value.slice(1);

            if (field === 'event') {
                eventName = value.trim() || 'message';
            } else if (field === 'data') {
                dataLines.push(value);
            } else if (separator < 0 && line.trimStart().startsWith('{')) {
                // A few OpenAI-compatible servers send raw JSON inside an SSE
                // response. Keep this compatibility at the protocol boundary.
                dataLines.push(line.trim());
            }
        }

        const data = dataLines.join('\n').trim();
        if (!data) return null;
        if (data === '[DONE]') return { type: 'stream.done' };

        try {
            const parsed = JSON.parse(data);
            if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
                throw new TypeError('stream payload must be a JSON object');
            }
            if (eventName === 'error' && !parsed.error) {
                return {
                    ...parsed,
                    type: 'error',
                    error: { message: String(parsed.message || data) }
                };
            }
            if (!parsed.type && eventName !== 'message') {
                return { ...parsed, type: eventName };
            }
            return parsed;
        } catch (error) {
            if (eventName === 'error') {
                return { type: 'error', error: { message: data } };
            }
            return {
                type: 'stream.parse_error',
                error: {
                    message: parseErrorMessage || 'The response stream contained an invalid event.',
                    detail: data.slice(0, 240)
                }
            };
        }
    }

    class SSEParser {
        constructor(options = {}) {
            this.decoder = new TextDecoder();
            this.buffer = '';
            this.finished = false;
            this.parseErrorMessage = String(options.parseErrorMessage || '');
        }

        parse(value) {
            if (this.finished) return [];
            if (value) this.buffer += this.decoder.decode(value, { stream: true });
            return this.consume(false);
        }

        finish() {
            if (this.finished) return [];
            this.finished = true;
            this.buffer += this.decoder.decode();
            return this.consume(true);
        }

        consume(flushRemainder) {
            const events = [];
            let boundary = findEventBoundary(this.buffer);
            while (boundary) {
                const rawBlock = this.buffer.slice(0, boundary.index);
                this.buffer = this.buffer.slice(boundary.index + boundary.length);
                const event = parseEventBlock(rawBlock, this.parseErrorMessage);
                if (event) events.push(event);
                boundary = findEventBoundary(this.buffer);
            }

            if (flushRemainder && this.buffer.trim()) {
                const event = parseEventBlock(this.buffer, this.parseErrorMessage);
                if (event) events.push(event);
                this.buffer = '';
            }
            return events;
        }
    }

    global.DKSTStreamProtocol = {
        SSEParser,
        parseEventBlock
    };
})(window);
