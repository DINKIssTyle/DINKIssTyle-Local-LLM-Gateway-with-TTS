import assert from 'node:assert/strict';
import * as p from './prompts.mjs';
const original='새로운 유전자를 등록합니다.',context={text:'[1] '+original};
for(const [target,name] of [['ko','Korean'],['en','English'],['ja','Japanese'],['zh','Chinese'],['es','Spanish'],['fr','French'],['de','German'],['vi','Vietnamese'],['th','Thai']]){
 assert.equal(p.outputLanguageName(target),name);
 const plan={topic:'유전자',source:'internal',kind:'term',question:'유전자 뜻',knownFacts:[original]};
 const chat=p.buildModePrompts('ai',{targetLang:target,prompt:original,conversationContext:'이전 발화'});
 const hint=p.hintAnswerPrompts(plan,'',target);
 for(const result of [chat,hint]){assert.ok(result.systemPrompt.includes('visible answer in '+name));assert.doesNotMatch(result.systemPrompt,/[가-힣]/u);assert.ok(result.userPrompt.includes(original),'data is never translated before selection/generation');}
 assert.ok(p.judgePrompts(context,{targetLang:target}).systemPrompt.includes('information card will be written in '+name));
 const reply=p.translationReplyPrompts(original,'translation','auto',target);
 assert.ok(reply.userPrompt.includes('Target language: '+name));assert.match(reply.systemPrompt,/two lines/);assert.match(reply.systemPrompt,/original speech language/);
 assert.ok(p.streamingTranslationPrompts({source:'auto',target,prompt:original}).userPrompt.includes(original));
}
assert.equal(p.outputLanguageName('auto'),'Korean');assert.equal(p.outputLanguageName('invalid'),'Korean');
assert.match(p.hintAnswerPrompts({source:'conversation',kind:'recap',topic:'일정',question:'요약'}).systemPrompt,/Return HINT_SKIP/);
assert.match(p.hintAnswerPrompts({source:'web',kind:'term',topic:'용어',question:'뜻',knownFacts:[]},'','en').systemPrompt,/exact source copying overrides the display language/);
assert.equal(p.judgePrompts(context).userPrompt,context.text);
console.log('PASS: English-only instructions, nine target languages, unchanged ASR/quotes, structured enums and bilingual replies');

const summaryPrompt=p.hintAnswerPrompts({source:'conversation',kind:'recap',topic:'정리',question:'요약'}).systemPrompt;
assert.match(summaryPrompt,/Return HINT_SKIP/);assert.match(summaryPrompt,/Do not relabel a recap as key/);
assert.match(p.judgePrompts(context).systemPrompt,/Do not output recaps/);
assert.match(p.hintPlanPrompts(original,'','background').systemPrompt,/conversation and recap are disabled/);

const activePolicy=p.judgePrompts(context,{sensitivity:'active'}).systemPrompt;
assert.match(activePolicy,/A single mention is enough/);
assert.match(activePolicy,/different useful facet/);
assert.match(activePolicy,/Still skip unclear ASR/);
assert.doesNotMatch(p.judgePrompts(context,{sensitivity:'balanced'}).systemPrompt,/low intervention threshold/);

const application=p.hintAnswerPrompts({topic:'물리학',kind:'term',source:'internal',question:'고고학에서 물리학이 활용되는 구체적 사례',knownFacts:[]});
assert.match(application.systemPrompt,/application, comparison or mechanism tasks take priority/);
assert.match(application.systemPrompt,/return only the selection integer/);
assert.match(application.userPrompt,/고고학에서 물리학이 활용되는 구체적 사례/);
