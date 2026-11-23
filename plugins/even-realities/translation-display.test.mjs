import assert from 'node:assert/strict';
import {getTextWidth} from '@evenrealities/pretext';
import {parseTranslationDisplay,translationReplyText,translationPhoneText} from './translation-display.mjs';
import {buildModePrompts} from './test-support/gateway-prompts.mjs';

const parsed=parseTranslationDisplay('**[번역]** 커피 드시겠어요?\n한 잔 더 있어요.\n[긍정] Yes, please. — 네, 주세요.\n[부정] No, thank you. — 괜찮습니다.\n[중립] Maybe later.');
assert.equal(parsed.translation,'커피 드시겠어요?\n한 잔 더 있어요.');
assert.equal(parsed.positive,'Yes, please. — 네, 주세요.');
assert.equal(parsed.negative,'No, thank you. — 괜찮습니다.');
assert.doesNotMatch(translationPhoneText(parsed),/중립|Maybe later|\[번역\]/);
assert.deepEqual(parseTranslationDisplay('[번역] 안녕하세요.\n[긍'),{translation:'안녕하세요.',positive:'',negative:''});
assert.equal(parseTranslationDisplay('[번역] 안녕하세요.\n[긍정] Hello!\n[부정').positive,'Hello!');
assert.equal(parseTranslationDisplay('[번').translation,'');
assert.equal(parseTranslationDisplay('안녕하세요.').translation,'안녕하세요.');
assert.deepEqual(parseTranslationDisplay('```json\n{"translation":"번역문","positive":"좋아요","negative":"아니요","neutral":"보류"}\n```'),{translation:'번역문',positive:'좋아요',negative:'아니요'});
const long={translation:'번역문',positive:'Yes, I would like that. '.repeat(30)+'\n'+'좋습니다. '.repeat(30),negative:'No, thank you. '.repeat(30)+'\n'+'괜찮습니다. '.repeat(30)};
const lines=translationReplyText(long,576).split('\n');
assert.equal(lines.length,4);
assert.match(lines[0],/^긍정: /);
assert.match(lines[2],/^부정: /);
assert.match(lines[1],/^ +좋습니다/);assert.match(lines[3],/^ +괜찮습니다/);
for(const line of lines){assert.ok(getTextWidth(line)<=576);assert.match(line,/\.\.\.$/);}
assert.ok(translationPhoneText(long).includes('좋습니다. '.repeat(20)));
assert.equal(translationReplyText({}).split('\n').length,4);
const prompt=buildModePrompts('translate',{prompt:'Would you like coffee?'}).systemPrompt;
assert.match(prompt,/rather than repeating or translating the utterance/);
assert.match(prompt,/translation[\s\S]*positive[\s\S]*negative/);
assert.match(prompt,/if already in that language/);
const coffee={translation:'커피 드시겠어요?',positive:'Yes, please. — 네, 주세요.',negative:'No, thank you. — 아니요, 괜찮습니다.'};
const raw=JSON.stringify(coffee);
for(let i=1;i<=raw.length;i++){
  const frame=parseTranslationDisplay(raw.slice(0,i));
  assert.ok(coffee.translation.startsWith(frame.translation));
  assert.ok(coffee.positive.startsWith(frame.positive));
  assert.ok(coffee.negative.startsWith(frame.negative));
}
assert.deepEqual(parseTranslationDisplay(raw),coffee);
assert.equal(parseTranslationDisplay('{"translation":"안녕\\uD5').translation,'안녕');
assert.equal(parseTranslationDisplay('{"translation":"번역문","positive":"[긍정] 괜찮아요","negative":"아니요"}').translation,'번역문');
console.log('PASS: translation and reply semantics, incremental JSON fields, legacy sections, four pixel-bounded bilingual reply rows');

const bilingual={translation:'Would you like coffee?',positive:'Yes, please.\n네, 주세요.',negative:'No, thank you.\n아니요, 괜찮습니다.'};
const rows=translationReplyText(bilingual,576).split('\n');
assert.equal(rows[0],'긍정: Yes, please.');assert.equal(rows[1].trim(),'네, 주세요.');
assert.equal(rows[2],'부정: No, thank you.');assert.equal(rows[3].trim(),'아니요, 괜찮습니다.');
assert.match(buildModePrompts('translate',{targetLang:'en'}).systemPrompt,/two lines: a short reply in the original speech language/);

const {mergeTranslationDisplay}=await import('./translation-display.mjs');
const previous={translation:'이전 번역',positive:'Yes.\n네.',negative:'No.\n아니요.'};
assert.deepEqual(mergeTranslationDisplay(previous,{translation:'새 임시 번역',positive:'',negative:''}),{...previous,translation:'새 임시 번역'});
assert.deepEqual(mergeTranslationDisplay(previous,{translation:'',positive:'새 긍정',negative:''},{commitReplies:true}),previous,'incomplete pair must not replace either previous reply');
assert.deepEqual(mergeTranslationDisplay(previous,{translation:'',positive:'새 긍정',negative:'새 부정'}),previous,'streaming pair stays private until completion');
assert.deepEqual(mergeTranslationDisplay(previous,{translation:'새 확정 번역',positive:'새 긍정',negative:'새 부정'},{commitReplies:true}),{translation:'새 확정 번역',positive:'새 긍정',negative:'새 부정'});
assert.deepEqual(mergeTranslationDisplay(previous,{translation:'',positive:'…',negative:'...'},{commitReplies:true}),previous);
console.log('PASS: previous reply pair survives drafts/partial/empty results; complete replacements commit together');

const {translationRepliesMatchLanguages:validReplies}=await import('./translation-display.mjs');
const cases=[
 {source:'en',target:'ko',translation:'커피 드시겠어요?',positive:'Yes, please.\n네, 주세요.',negative:'No, thanks.\n아니요, 괜찮습니다.'},
 {source:'ko',target:'en',translation:'Would you like coffee?',positive:'Ne, juseyo.\nYes, please.',negative:'Aniyo, gwaenchanseumnida.\nNo, thanks.'},
 {source:'en',target:'ja',translation:'コーヒーはいかがですか？',positive:'Yes, please.\nはい、お願いします。',negative:'No, thanks.\nいいえ、結構です。'},
 {source:'ja',target:'ko',translation:'커피 드시겠어요?',positive:'Hai, onegai shimasu.\n네, 부탁합니다.',negative:'Iie, kekkou desu.\n아니요, 괜찮습니다.'},
 {source:'ko',target:'ja',translation:'コーヒーはいかがですか？',positive:'Ne, juseyo.\nはい、お願いします。',negative:'Aniyo, gwaenchanseumnida.\nいいえ、結構です。'},
];
for(const sample of cases){
 assert.ok(validReplies(sample,sample.source,sample.target),sample.source+'>'+sample.target);
 const display=mergeTranslationDisplay({},sample,{commitReplies:true});
 assert.equal(display.translation,sample.translation);
 const rows=translationReplyText(display).split('\n');
 assert.equal(rows.length,4);
 assert.equal(rows[0],'긍정: '+sample.positive.split('\n')[0]);
 assert.equal(rows[1].trim(),sample.positive.split('\n')[1]);
 assert.equal(rows[2],'부정: '+sample.negative.split('\n')[0]);
 assert.equal(rows[3].trim(),sample.negative.split('\n')[1]);
}
assert.equal(validReplies({positive:'네, 좋아요.\nSounds great!',negative:'지금은 어려워요.\nI cannot right now.'},'ko','en'),false,'Korean source must be romanized too');
assert.equal(validReplies({positive:'Sounds great!\nThat works well.',negative:'I cannot right now.\nMaybe another time.'},'en','ko'),false,'target Korean cannot be replaced by English alternatives');
assert.equal(validReplies({positive:'はい。\n네.',negative:'いいえ。\n아니요.'},'ja','ko'),false);
assert.equal(validReplies({positive:'Hai.\nYes.',negative:'Iie.\nNo.'},'ja','ko'),false);
assert.equal(validReplies({positive:'Yes.\nHai.',negative:'No.\nIie.'},'en','ja'),false,'target Japanese stays in native writing');
assert.equal(validReplies({positive:'네 (Ne).\nYes.',negative:'아니요 (Aniyo).\nNo.'},'ko','en'),false,'first line must not retain native-script source');
assert.ok(validReplies({positive:'Ne.\n네.',negative:'Aniyo.\n아니요.'},'auto','ko'),'auto source follows the same Latin-only first-line rule');
const legacyPronunciation=parseTranslationDisplay(JSON.stringify({...cases[4],pronunciation:'코히와 이카가데스카'}));
assert.equal(legacyPronunciation.pronunciation,undefined);
assert.doesNotMatch(translationPhoneText({...cases[4],pronunciation:'코히와 이카가데스카'}),/발음:|코히와/);
console.log('PASS: all four requested directions, Korean romanization, native target rows and no translation pronunciation');
