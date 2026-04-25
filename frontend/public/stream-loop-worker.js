const LOOP_DETECTION_TAIL_CHARS = 1200;

function getTail(text, minLength = 0) {
    const source = String(text || '');
    if (source.length < minLength) return '';
    return source.slice(-LOOP_DETECTION_TAIL_CHARS);
}

function detectMessageLoop(text) {
    const tail = getTail(text, 100);
    if (!tail) return null;

    const shortLoopMatch = tail.match(/([\s\S]{5,}?)\1{9,}/);
    const longLoopMatch = tail.match(/([\s\S]{50,}?)\1{5,}/);
    const loopMatch = shortLoopMatch || longLoopMatch;
    if (!loopMatch || !loopMatch[1] || loopMatch[1].length < 4) return null;

    const snippet = loopMatch[1];
    const isToolLog = snippet.includes('Tool Call')
        || snippet.includes('Tool Finished')
        || snippet.includes('🛠️')
        || snippet.includes('✅');
    if (isToolLog) return null;

    return {
        snippet: snippet.slice(0, 120),
        repetitions: Math.max(2, Math.floor(loopMatch[0].length / loopMatch[1].length)),
        source: 'message-loop'
    };
}

function countRunes(value = '') {
    return Array.from(String(value || '')).length;
}

function normalizeReasoningSegment(value = '') {
    return String(value || '').replace(/\s+/g, ' ').trim();
}

function detectRepeatedReasoningSection(text) {
    const seen = new Map();
    const addCandidate = (candidate) => {
        const segment = normalizeReasoningSegment(candidate);
        if (countRunes(segment) < 36) return null;
        const key = segment.toLocaleLowerCase();
        const nextCount = (seen.get(key) || 0) + 1;
        seen.set(key, nextCount);
        if (nextCount >= 4) {
            return {
                snippet: segment.slice(0, 160),
                source: 'repeated-section'
            };
        }
        return null;
    };

    for (const line of String(text || '').split(/\n+/)) {
        const match = addCandidate(line);
        if (match) return match;
    }

    const normalized = normalizeReasoningSegment(text);
    const sentences = normalized.match(/[^.!?。！？]+[.!?。！？]+/g) || [];
    for (const sentence of sentences) {
        const match = addCandidate(sentence);
        if (match) return match;
    }

    return null;
}

function detectReasoningLoop(text) {
    const normalized = getTail(text, 140).replace(/\s+/g, ' ').trim();
    if (normalized.length < 140) return null;

    const shortLoopMatch = normalized.match(/([\s\S]{6,}?)\1{8,}/);
    if (shortLoopMatch && shortLoopMatch[1]) {
        return {
            snippet: shortLoopMatch[1].slice(0, 80),
            source: 'chunk-loop'
        };
    }

    const sentenceLoopMatch = normalized.match(/(.{12,}?[.!?"])(?:\s+\1){5,}/i);
    if (sentenceLoopMatch && sentenceLoopMatch[1]) {
        return {
            snippet: sentenceLoopMatch[1].slice(0, 120),
            source: 'sentence-loop'
        };
    }

    const sectionLoopMatch = normalized.match(/([\s\S]{240,}?)\1{2,}/);
    if (sectionLoopMatch && sectionLoopMatch[1]) {
        return {
            snippet: sectionLoopMatch[1].slice(0, 160),
            source: 'section-loop'
        };
    }

    const repeatedSectionMatch = detectRepeatedReasoningSection(String(text || '').slice(-6000));
    if (repeatedSectionMatch) return repeatedSectionMatch;

    const wordLoopMatch = normalized.match(/\b([^\s]{2,30})\b(?:\s+\1){11,}/i);
    if (wordLoopMatch && wordLoopMatch[1]) {
        return {
            snippet: wordLoopMatch[1],
            source: 'word-loop'
        };
    }

    return null;
}

self.onmessage = (event) => {
    const data = event?.data || {};
    const id = Number(data.id || 0);
    const kind = String(data.kind || 'message');
    const text = String(data.text || '');

    let result = null;
    if (kind === 'reasoning') {
        result = detectReasoningLoop(text);
    } else {
        result = detectMessageLoop(text);
    }

    self.postMessage({ id, result });
};
