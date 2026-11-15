import assert from 'node:assert/strict';
import {TranslationMetrics} from './translation-metrics.mjs';
let last;const m=new TranslationMetrics({limit:2,onUpdate:x=>last=x});
m.record('one',{stt_http_ms:12.4,stt_server_ms:4,prompt:'private',translation_http_ms:NaN});assert.deepEqual(last,{stt_http_ms:12,stt_server_ms:4});
m.record('one',{upload_bytes:300,audio_bytes:3000});assert.equal(last.upload_bytes,300);m.record('two',{});m.record('three',{});assert.equal(m.entries.size,2);assert.equal(m.entries.has('one'),false);
console.log('PASS: bounded numeric-only translation diagnostics');
