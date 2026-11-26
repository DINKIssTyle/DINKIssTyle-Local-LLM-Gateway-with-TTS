// Use the same labels and values as the phone settings. SDK menus have no submenus,
// so Level 1 exposes quick actions + hub entries and Level 2/3 render native list pickers.
import {ROLE_MODEL_KEYS,normalizeRoleModels,roleModelOptions} from './role-models.mjs';

export const DISPLAY_MENU_ID=10;
export const ROLE_MODEL_MENU_ID=11;
export const SETTINGS_HUB_MENU_ID=12;
export const LANGUAGE_MENU_ID=13;
export const ROLE_MODEL_LABELS={
  chat:'AI 채팅',
  hintDecision:'AI 힌트 판별',
  hint:'AI 힌트',
  translation:'번역',
  translationReply:'번역 답변',
};

export const SETTINGS_MENUS={
  10:{title:'전사문 숨기기',key:'hideTranscript',options:[[false,'3줄 · 최대 3줄 표시'],[true,'숨김 · 전사와 저장은 계속']]},
  15:{title:'AI모드 상단 레이아웃',key:'aiHeaderLayout',options:[['standard','기본 · 호출어와 상태, 시각·배터리'],['status-icon','간결 · 상태 아이콘 하나']]},
  16:{title:'번역모드 레이아웃',key:'translationLayout',options:[['standard','기본 · 번역과 예상 답변'],['subtitles','간결 · 번역문만 표시']]},
  17:{title:'자막 표시 줄 수',key:'subtitleLines',options:[[1,'1줄'],[2,'2줄'],[3,'3줄'],[4,'4줄'],[5,'5줄']]},
  4:{title:'인식 언어',key:'translateSourceLang',options:[['auto','자동 (상대 언어)'],['en','영어 (English)'],['ko','한국어 (Korean)'],['ja','일본어 (Japanese)'],['zh','중국어 (Chinese)'],['es','스페인어 (Spanish)'],['fr','프랑스어 (French)'],['de','독일어 (German)'],['vi','베트남어 (Vietnamese)'],['th','태국어 (Thai)']]},
  5:{title:'번역 언어',key:'translateTargetLang',options:[['ko','한국어 (Korean)'],['en','영어 (English)'],['ja','일본어 (Japanese)'],['zh','중국어 (Chinese)'],['es','스페인어 (Spanish)'],['fr','프랑스어 (French)'],['de','독일어 (German)'],['vi','베트남어 (Vietnamese)'],['th','태국어 (Thai)']]},
  6:{title:'번역 엔진',key:'translationEngine',options:[['llm','LLM 번역'],['chrome','Chrome 로컬 번역'],['apple','Apple 로컬 번역 · macOS 26+']]},
  7:{title:'음성 인식 엔진',key:'sttEngine',options:[['whisper','Whisper'],['qwen3-asr','Qwen3-ASR · Apple Silicon'],['chrome-online','Chrome 온라인 STT · Google로 음성 전송'],['apple','Apple 로컬 STT · macOS 26+']]},
  8:{title:'내 목소리 제외',key:'translateExcludeSelf',options:[[true,'켬 · 번역 중 내 목소리 제외'],[false,'끔 · 모든 목소리 번역']]},
  9:{title:'안경 입력 감지 민감도',key:'glassesInput',subKey:'sensitivity',options:[['standard','표준 · 기존 감지 기준'],['high','높음 · 작은 목소리 감지'],['very_high','매우 높음 · 조용한 환경'],['auto','자동 · 주변 음량에 맞춤']]},
  14:{title:'안경 입력 증폭',key:'glassesInput',subKey:'gain',options:[['auto','자동 · 1~4배'],[1,'없음 · 1배'],[2,'2배 · 약 +6 dB'],[4,'4배 · 약 +12 dB']]},
};

const DISPLAY_HUB_IDS=[10,15,16,17];
const LANGUAGE_HUB_IDS=[4,5,8];
const SETTINGS_HUB_IDS=[6,7,9,14];
const MAX_LIST_ITEMS=20;
const MAX_LIST_LABEL_CHARS=60;

function clampPickerLabel(text,max=MAX_LIST_LABEL_CHARS){
  const chars=Array.from(String(text || '').trim());
  return chars.length<=max?chars.join(''):`${chars.slice(0,max-1).join('')}…`;
}

export function settingCurrentValue(id,values={}){
  const menu=SETTINGS_MENUS[id];
  if(!menu)return undefined;
  return menu.subKey?values[menu.key]?.[menu.subKey]:values[menu.key];
}

function settingSummaryValue(id,values={}){
  const menu=SETTINGS_MENUS[id];
  if(!menu)return '';
  const current=settingCurrentValue(id,values);
  if(id===8)return current?'제외함':'제외 안함';
  const match=menu.options.find(([value])=>value===current || String(value)===String(current));
  if(!match)return '';
  const label=match[1];
  const short=label.split(' · ')[0].replace(/\s*\([^)]*\)$/, '').trim();
  return short || label;
}

export function settingsMenuItems(){
  return [
    {itemID:LANGUAGE_MENU_ID,itemName:'언어 설정'},
    {itemID:DISPLAY_MENU_ID,itemName:'화면 표시'},
    {itemID:ROLE_MODEL_MENU_ID,itemName:'역할별 LLM 모델'},
    {itemID:SETTINGS_HUB_MENU_ID,itemName:'설정'},
  ];
}

export function displayHubPicker(values={}){
  const options=DISPLAY_HUB_IDS.map(id=>{
    const summary=settingSummaryValue(id,values);
    const label=summary?`${SETTINGS_MENUS[id].title} - ${summary}`:SETTINGS_MENUS[id].title;
    return [id,clampPickerLabel(label)];
  });
  const sig=DISPLAY_HUB_IDS.map(id=>String(settingCurrentValue(id,values) ?? '')).join('|');
  return {
    kind:'display-hub',
    title:'화면 표시',
    help:'탭: 변경/선택 · 더블탭: 돌아가기',
    options,
    index:0,
    pageKey:`display-hub:${sig}`,
  };
}

export function languageHubPicker(values={}){
  const options=LANGUAGE_HUB_IDS.map(id=>{
    const summary=settingSummaryValue(id,values);
    const label=summary?`${SETTINGS_MENUS[id].title} - ${summary}`:SETTINGS_MENUS[id].title;
    return [id,clampPickerLabel(label)];
  });
  const sig=LANGUAGE_HUB_IDS.map(id=>String(values[SETTINGS_MENUS[id].key] ?? '')).join('|');
  return {
    kind:'language-hub',
    title:'언어 설정',
    help:'탭: 선택 · 더블탭: 돌아가기',
    options,
    index:0,
    pageKey:`language-hub:${sig}`,
  };
}

export function settingsHubPicker(glasses=true,values={}){
  const ids=SETTINGS_HUB_IDS.filter(id=>glasses || (id!==9 && id!==14));
  const options=ids.map(id=>{
    const summary=settingSummaryValue(id,values);
    const label=summary?`${SETTINGS_MENUS[id].title} - ${summary}`:SETTINGS_MENUS[id].title;
    return [id,clampPickerLabel(label)];
  });
  const sig=ids.map(id=>String(settingCurrentValue(id,values) ?? '')).join('|');
  return {
    kind:'settings-hub',
    title:'설정',
    help:'탭: 항목 선택 · 더블탭: 돌아가기',
    options,
    index:0,
    pageKey:`settings-hub:${glasses?'g':'p'}:${sig}`,
  };
}

function roleModelDisplayName(role,selected,models,baseModel,jevOnly){
  if(role==='hintDecision' && selected===jevOnly)return '로컬 후보 판별';
  if(!selected){
    const baseLabel=models.find(m=>m.id===baseModel)?.label || baseModel || '미선택';
    return `기본 (${baseLabel})`;
  }
  return models.find(m=>m.id===selected)?.label || selected;
}

export function roleModelHubPicker({roleModels={},baseModel='',models=[],jevOnly='__jev__'}={}){
  const normalized=normalizeRoleModels(roleModels);
  const options=ROLE_MODEL_KEYS.map(role=>{
    const current=roleModelDisplayName(role,normalized[role],models,baseModel,jevOnly);
    return [role,clampPickerLabel(`${ROLE_MODEL_LABELS[role]} - ${current}`)];
  });
  const sig=ROLE_MODEL_KEYS.map(role=>normalized[role] || '-').join('|');
  return {
    kind:'role-models',
    title:'역할별 LLM 모델',
    help:'탭: 역할 선택 · 더블탭: 돌아가기',
    options,
    index:0,
    pageKey:`role-models:${baseModel}:${sig}`,
  };
}

export function roleModelSelectPicker(role,{roleModels={},baseModel='',models=[],jevOnly='__jev__'}={}){
  const normalized=normalizeRoleModels(roleModels);
  const selected=normalized[role] || '';
  const rawOptions=roleModelOptions(models,baseModel,selected).filter(option=>role!=='hintDecision' || option.id!==jevOnly);
  if(role==='hintDecision' && !rawOptions.some(option=>option.id===jevOnly)){
    rawOptions.splice(1,0,{id:jevOnly,label:'로컬 후보 판별'});
  }
  let capped=rawOptions;
  if(capped.length>MAX_LIST_ITEMS){
    const keep=capped.slice(0,MAX_LIST_ITEMS);
    if(!keep.some(option=>option.id===selected)){
      const currentOption=capped.find(option=>option.id===selected);
      if(currentOption)keep[MAX_LIST_ITEMS-1]=currentOption;
    }
    capped=keep;
  }
  return {
    kind:'role-model-select',
    role,
    parent:'role-models',
    title:`역할별 LLM · ${ROLE_MODEL_LABELS[role] || role}`,
    help:'탭: 적용 · 더블탭: 이전 메뉴',
    options:capped.map(option=>[option.id,clampPickerLabel(option.label)]),
    value:selected,
    index:0,
    pageKey:`role-model:${role}:${selected}:${capped.length}`,
  };
}

export function settingsPickerPage(picker){
  const hasValue=Object.prototype.hasOwnProperty.call(picker,'value');
  const names=picker.options.map(([value,label])=>clampPickerLabel(`${hasValue && value===picker.value?'✓ ':''}${label}`,64));
  return {containerTotalNum:3,listObject:[{containerID:30,containerName:'settings-list',xPosition:0,yPosition:30,width:576,height:228,isEventCapture:1,itemContainer:{itemCount:names.length,itemWidth:0,isItemSelectBorderEn:1,itemName:names}}],
    textObject:[{containerID:31,containerName:'settings-title',xPosition:0,yPosition:0,width:576,height:27,content:picker.title,isEventCapture:0},
      {containerID:32,containerName:'settings-help',xPosition:0,yPosition:261,width:576,height:27,content:picker.help || '탭: 선택 · 더블탭: 돌아가기',isEventCapture:0}],
    menuObject:{menuItems:[{itemID:90,itemName:'돌아가기'}]}};
}
