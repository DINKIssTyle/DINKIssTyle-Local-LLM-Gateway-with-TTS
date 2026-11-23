import assert from 'node:assert/strict';
import {TranslationDirection} from './translation-direction.mjs';
const d=new TranslationDirection();
const route=(detected,text='',source='auto',at=0)=>d.route({source,target:'ko',detected,text,engine:'whisper',startedAt:new Date(at).toISOString()});
assert.equal(route('ko','안녕하세요'),null); // Never guess a partner language.
assert.deepEqual(route('ja','こんにちは'),{source:'ja',target:'ko',changed:false});
assert.deepEqual(route('ko','안녕하세요','auto',100),{source:'ko',target:'ja',changed:true});
assert.deepEqual(route('ko','반갑습니다','auto',200),{source:'ko',target:'ja',changed:false});
assert.equal(route('ja','','auto',150),null); // Late older audio cannot erase the new direction.
assert.deepEqual(route('ja','','auto',300),{source:'ja',target:'ko',changed:true});
d.reset();assert.deepEqual(route('ko','안녕하세요','zh'),{source:'ko',target:'zh',changed:false});
assert.deepEqual(d.route({source:'en',target:'ko',detected:'ko',engine:'apple'}),{source:'en',target:'ko',changed:true});
console.log('PASS: bilingual routing, partner selection, direction changes, stale audio');

const anyTarget=new TranslationDirection();
assert.deepEqual(anyTarget.route({source:'auto',target:'en',detected:'ko-KR',engine:'whisper'}),{source:'ko',target:'en',changed:false});
assert.deepEqual(anyTarget.route({source:'auto',target:'en',detected:'ja',engine:'whisper'}),{source:'ja',target:'en',changed:true});
assert.deepEqual(anyTarget.route({source:'auto',target:'ja',detected:'en',engine:'apple'}),{source:'en',target:'ja',changed:true});
assert.deepEqual(anyTarget.route({source:'ko',target:'en',detected:'en',engine:'apple'}),{source:'ko',target:'en',changed:true});

const qwen=new TranslationDirection();
assert.equal(qwen.route({source:'auto',target:'ko',detected:'ko',engine:'qwen3-asr'}),null,'never guess a partner');
assert.deepEqual(qwen.route({source:'auto',target:'ko',detected:'ja',engine:'qwen3-asr'}),{source:'ja',target:'ko',changed:false});
assert.deepEqual(qwen.route({source:'auto',target:'ko',detected:'ko',engine:'qwen3-asr'}),{source:'ko',target:'ja',changed:true});
assert.deepEqual(qwen.route({source:'auto',target:'en',detected:'ko',engine:'qwen3-asr'}),{source:'ko',target:'en',changed:true});
console.log('PASS: Qwen detected language and bidirectional translation routing');
