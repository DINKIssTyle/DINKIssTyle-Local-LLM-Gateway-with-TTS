import assert from 'node:assert/strict';
import {validateEvenHubPageContainer} from '@evenrealities/even_hub_sdk';
import {ROLE_MODEL_KEYS} from './role-models.mjs';
import {DISPLAY_MENU_ID,LANGUAGE_MENU_ID,ROLE_MODEL_MENU_ID,SETTINGS_HUB_MENU_ID,SETTINGS_MENUS,displayHubPicker,languageHubPicker,roleModelHubPicker,roleModelSelectPicker,settingsHubPicker,settingsMenuItems,settingsPickerPage} from './settings-menu.mjs';

const topItems=settingsMenuItems();
for(const menu of topItems)assert.ok(new TextEncoder().encode(menu.itemName).length<=32);
assert.ok(topItems.length+2<=10);
assert.deepEqual(topItems.map(item=>item.itemID),[LANGUAGE_MENU_ID,DISPLAY_MENU_ID,ROLE_MODEL_MENU_ID,SETTINGS_HUB_MENU_ID]);

// Display hub (Level 2) previews current values inline.
const dispHub=displayHubPicker({hideTranscript:false,aiHeaderLayout:'standard',translationLayout:'subtitles',subtitleLines:3});
assert.deepEqual(dispHub.options.map(([,label])=>label),['전사문 숨기기 - 3줄','AI모드 상단 레이아웃 - 기본','번역모드 레이아웃 - 간결','자막 표시 줄 수 - 3줄']);
assert.equal(validateEvenHubPageContainer(settingsPickerPage(dispHub)).valid,true);

// Language hub (Level 2) previews current values inline.
const langHub=languageHubPicker({translateSourceLang:'en',translateTargetLang:'ko',translateExcludeSelf:false});
assert.deepEqual(langHub.options.map(([,label])=>label),['인식 언어 - 영어','번역 언어 - 한국어','내 목소리 제외 - 제외 안함']);
assert.equal(validateEvenHubPageContainer(settingsPickerPage(langHub)).valid,true);
const langHubExcluded=languageHubPicker({translateSourceLang:'auto',translateTargetLang:'ja',translateExcludeSelf:true});
assert.deepEqual(langHubExcluded.options.map(([,label])=>label),['인식 언어 - 자동','번역 언어 - 일본어','내 목소리 제외 - 제외함']);

// Settings hub (Level 2) previews current values and hides glasses-only sensitivity/gain on phone input.
const settingsGlasses=settingsHubPicker(true,{translationEngine:'llm',sttEngine:'whisper',glassesInput:{sensitivity:'high',gain:2}});
assert.deepEqual(settingsGlasses.options.map(([,label])=>label),['번역 엔진 - LLM 번역','음성 인식 엔진 - Whisper','안경 입력 감지 민감도 - 높음','안경 입력 증폭 - 2배']);
assert.ok(!settingsHubPicker(false,{translationEngine:'llm',sttEngine:'whisper'}).options.some(([id])=>id===9 || id===14));
for(const glasses of [true,false]){
  const page=settingsPickerPage(settingsHubPicker(glasses,{translationEngine:'llm',sttEngine:'whisper',glassesInput:{sensitivity:'standard',gain:1}}));
  const result=validateEvenHubPageContainer(page);
  assert.equal(result.valid,true,JSON.stringify(result));
}

// Role model hub (Level 2) & per-role selector (Level 3) pass SDK validation even with >20 models.
const manyModels=Array.from({length:25},(_,i)=>({id:`model-${i+1}`,label:`Very Long Local LLM Model Name Number ${i+1} With Extra Metadata`}));
const roleModels={chat:'model-25',hintDecision:'__jev__',hint:'',translation:'model-2',translationReply:''};
const roleHub=roleModelHubPicker({roleModels,baseModel:'model-1',models:manyModels});
assert.equal(roleHub.options.length,ROLE_MODEL_KEYS.length);
assert.equal(validateEvenHubPageContainer(settingsPickerPage(roleHub)).valid,true);

for(const role of ROLE_MODEL_KEYS){
  const selectPicker=roleModelSelectPicker(role,{roleModels,baseModel:'model-1',models:manyModels});
  const page=settingsPickerPage(selectPicker);
  const result=validateEvenHubPageContainer(page);
  assert.equal(result.valid,true,JSON.stringify(result));
  assert.ok(selectPicker.options.length<=20);
  if(role==='chat')assert.ok(selectPicker.options.some(([id])=>id==='model-25'),'current selection must be retained when capping at 20');
  if(role==='hintDecision')assert.ok(selectPicker.options.some(([id])=>id==='__jev__'));
  for(const label of page.listObject[0].itemContainer.itemName)assert.ok(Array.from(label).length<=64);
}

for(const menu of Object.values(SETTINGS_MENUS)){
  const page=settingsPickerPage({...menu,value:menu.options[0][0]});
  const result=validateEvenHubPageContainer(page);
  assert.equal(result.valid,true,JSON.stringify(result));
  assert.equal(page.listObject[0].isEventCapture,1);
  assert.ok(menu.options.length<=20);
  for(const [,label] of menu.options)assert.ok(Array.from(label).length+2<=64);
}
console.log('PASS: existing menu names, SDK menu byte/count limits, language/role/settings hubs with value previews and geometry');
