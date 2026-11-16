import assert from 'node:assert/strict';
import {TranslationDirection,pronunciationText} from './translation-direction.mjs';
const d=new TranslationDirection();
const route=(detected,text='',source='auto',at=0)=>d.route({source,target:'ko',detected,text,engine:'whisper',startedAt:new Date(at).toISOString()});
assert.equal(route('ko','안녕하세요'),null); // Never guess a partner language.
assert.deepEqual(route('ja','こんにちは'),{source:'ja',target:'ko',changed:false});
assert.deepEqual(route('ko','안녕하세요','auto',100),{source:'ko',target:'ja',changed:true});
assert.deepEqual(route('ko','반갑습니다','auto',200),{source:'ko',target:'ja',changed:false});
assert.equal(route('ja','','auto',150),null); // Late older audio cannot erase the new direction.
assert.deepEqual(route('ja','','auto',300),{source:'ja',target:'ko',changed:true});
d.reset();assert.deepEqual(route('ko','안녕하세요','zh'),{source:'ko',target:'zh',changed:false});
assert.equal(pronunciationText('你好','니 하오','zh'),'你好\n발음: 니 하오');
assert.equal(pronunciationText('Hello','헬로','en'),'Hello');
assert.equal(pronunciationText('안녕','안녕','ko'),'안녕');
assert.deepEqual(d.route({source:'en',target:'ko',detected:'ko',engine:'apple'}),{source:'en',target:'ko',changed:true});
console.log('PASS: bilingual routing, partner selection, direction changes, stale audio, pronunciation');
