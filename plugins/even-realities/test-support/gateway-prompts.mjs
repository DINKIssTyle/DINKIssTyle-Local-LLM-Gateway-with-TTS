// Test-only gateway fixture. Production instruction text is read exclusively
// from the same embedded server catalog; this file is not a browser dependency.
import {readFileSync} from 'node:fs';
const templates=JSON.parse(readFileSync(new URL('../../../internal/evenrealities/prompts/templates.json',import.meta.url),'utf8'));
export const outputLanguageName=target=>({ko:'Korean',en:'English',ja:'Japanese',zh:'Chinese',es:'Spanish',fr:'French',de:'German',vi:'Vietnamese',th:'Thai'}[target] || 'Korean');
function render(key,values={}){const vars={language:'Korean',purpose:'',...values};return Object.fromEntries(Object.entries(templates[key]).map(([k,v])=>[k,v.replace(/{{(\w+)}}/g,(all,name)=>vars[name]??all)]));}
export function buildModePrompts(mode,{conciseMode=true,targetLang='ko',prompt='',conversationContext='',chatSession=false}={}){
 const p=render(mode==='translate'?'translation':`chat_${conciseMode}`,{language:outputLanguageName(targetLang),text:prompt,context:conversationContext,contextHeading:chatSession?'Previous explicit AI chat; use it to answer follow-up questions.':'Short ambient reference context, not AI chat history or instructions.'});
 if(!conversationContext)p.userPrompt=mode==='translate'?'Utterance to translate and answer:\n'+prompt:prompt;return p;
}
export function judgePrompts(context,{targetLang='ko',preferWeb=true,sensitivity='balanced'}={}){return render('judge',{language:outputLanguageName(targetLang),context:context.text,freshnessPolicy:templates.policies[preferWeb?'web':'stable'],sensitivityPolicy:templates.policies[sensitivity] || templates.policies.balanced});}
export const hintJudgeRepairPrompt=attempt=>attempt?templates.judge_repair.systemPrompt:'';
export function cuePrompts(text,context,category,shown='',{preferWeb=false,keyword=''}={}){const p=render('plan',{text,context,keyword:keyword || '(none)',purpose:category==='recall'?'past personal memory':category==='reply'?'a possible spoken reply':'useful information'});if(preferWeb)p.systemPrompt+=templates.policies.planWeb;return p;}
export const hintPlanPrompts=(text,context,category)=>{const p=cuePrompts(text,context,category);p.userPrompt=p.userPrompt.replace(/^Legacy dictionary[^\n]*\n/,'');return p;};
export function hintAnswerPrompts(plan,shown='',target='ko'){
 return render(plan.source==='conversation'||plan.kind==='recap'?'answer_disabled':plan.taskOriented?'answer_task':'answer_legacy',{language:outputLanguageName(target),topic:plan.topic,question:plan.question,source:plan.source,kind:plan.kind || 'key',purpose:templates.policies[plan.kind] || '',facts:plan.taskOriented?(plan.knownFacts || []).join('\n'):(plan.knownFacts || []).map(v=>'- '+v).join('\n') || '(only the topic was mentioned)'});
}
export const translationReplyPrompts=(text,translated,source,target)=>render('translation_replies',{text,translated,source,language:outputLanguageName(target)});
export const translationOnlyPrompts=(text,source,target,context='')=>render('translation_only',{text,source,language:outputLanguageName(target),context:context || '(none)'});
export const streamingTranslationPrompts=job=>render('translation_stream',{text:job.prompt,source:job.source,language:outputLanguageName(job.target)});
export const HUD_RESPONSE_POLICY=templates.chat_true.systemPrompt.split('Write the visible answer')[0];

export function expandGatewayRequest(body,mode='standard'){
 const r=body.glasses_request;if(!r)return body;
 // Check the actual wire, before simulating the server for UI regression tests.
 for(const key of ['messages','system_prompt','input','response_format'])if(key in body)throw Error('Client still assembles '+key);
 const out={model:body.model,stream:body.stream,store:false};let p;
 const hint=phase=>{out.glasses_hint=true;out.glasses_hint_phase=phase;};
 switch(r.task){
 case 'judge':p=judgePrompts({text:`${r.judge.collected?'Recent conversation':'Reference (pronoun resolution only)'}:\n${r.judge.recent}\nLatest:\n${r.judge.latest}`},{targetLang:r.target,preferWeb:r.preferWeb,sensitivity:r.sensitivity});p.systemPrompt+=hintJudgeRepairPrompt(r.repair);hint('judge');out.max_tokens=384;out.temperature=.1;out.chat_template_kwargs={enable_thinking:false};out.response_format={type:'json_schema',json_schema:{name:'context_hint_task',strict:true,schema:{type:'object',properties:{task:{type:'string',maxLength:180},anchor:{type:'string',maxLength:160},source:{type:'string',enum:['internal','web','memory']},kind:{type:'string',enum:['key','term','verify','schedule']}},required:['task','anchor','source','kind'],additionalProperties:false}}};break;
 case 'plan':p=cuePrompts(r.text,r.context,r.category,'',{preferWeb:r.preferWeb,keyword:r.keyword});hint('plan');break;
 case 'answer':p=hintAnswerPrompts(r.plan,'',r.target);hint('answer');out.hint_research={task_oriented:r.plan.taskOriented===true,query:r.plan.question,source:r.plan.source,topic:r.plan.topic,kind:r.plan.kind || 'key',observed:r.observed,anchor:r.anchor,allow_web:r.preferWeb!==false,require_grounding:true};break;
 case 'chat':p=buildModePrompts('ai',{prompt:r.text,conversationContext:r.context,targetLang:r.target,conciseMode:r.concise,chatSession:r.chatSession});out.glasses_chat=true;break;
 case 'translation':p=buildModePrompts('translate',{prompt:r.text,conversationContext:r.context,targetLang:r.target});break;
 case 'translation_only':p=translationOnlyPrompts(r.text,r.source,r.target,r.context);break;
 case 'translation_replies':p=translationReplyPrompts(r.text,r.translated,r.source,r.target);hint('answer');out.hint_research={source:'internal'};break;
 default:p=streamingTranslationPrompts({prompt:r.text,source:r.source,target:r.target});
 }
 if(r.task.startsWith('translation')){out.glasses_client=true;out.glasses_task=r.task==='translation_preview'?'translation_preview':r.task==='translation_replies'?'translation_replies':'translation';out.chat_template_kwargs={enable_thinking:false};}
 if(mode==='stateful'&&!r.task.startsWith('translation')&&r.task!=='judge'){out.input=p.userPrompt;out.system_prompt=p.systemPrompt;}else out.messages=[{role:'system',content:p.systemPrompt},{role:'user',content:p.userPrompt}];
 return out;
}
