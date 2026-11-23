import {pxTruncate,getTextWidth} from '@evenrealities/pretext';
import {stripSourcesForGlasses} from './hud-status.mjs';

export function parseTranslationDisplay(raw) {
  const text=stripSourcesForGlasses(raw).replace(/\*\*(\[[^\]]+\])\*\*/g,'$1').trim();
  const result={translation:'',positive:'',negative:''};
  const json=text.replace(/^```(?:json)?\s*/i,'').replace(/\s*```$/,'');
  if(json.startsWith('{')){
    try{
      const data=JSON.parse(json);
      for(const key of Object.keys(result))if(typeof data[key]==='string')result[key]=data[key].trim();
    }catch{
      // Decode each string independently while a JSON object is streaming.
      // An incomplete escape stays private until the next chunk arrives.
      for(const match of json.matchAll(/"(translation|positive|negative)"\s*:\s*"((?:\\.|[^"\\])*)/gu)){
        let value=match[2].replace(/\\(?:u[0-9a-f]{0,3})?$/iu,'');
        try{result[match[1]]=JSON.parse('"'+value+'"').trim();}catch{}
      }
    }
    return result;
  }
  const labels=[...text.matchAll(/\[(번역|긍정|부정|중립|translation|positive|negative|neutral)\][ \t]*[:：]?[ \t]*/giu)];
  const keys={번역:'translation',translation:'translation',긍정:'positive',positive:'positive',부정:'negative',negative:'negative'};
  for(let i=0;i<labels.length;i++){
    const key=keys[labels[i][1].toLowerCase()];if(!key)continue;
    result[key]=text.slice(labels[i].index+labels[i][0].length,labels[i+1]?.index ?? text.length)
      .replace(/\[(?:번|번역|긍|긍정|부|부정|중|중립)?$/u,'').trim();
  }
  // Legacy labeled output remains readable for existing models.
  if(!labels.length && !text.includes('[') && !text.startsWith('`'))result.translation=text;
  return result;
}
export function translationReplyText(result,maxWidth=null) {
  return [['긍정',result.positive],['부정',result.negative]].map(([label,value])=>{
    const parts=String(value || '').split(/\r?\n|[ \t]+—[ \t]+/u);
    const first=parts.shift()?.trim() || '…';
    const second=parts.join(' ').replace(/\s+/gu,' ').trim();
    const prefix=`${label}: `;
    const indent=' '.repeat(Math.round(getTextWidth(prefix)/getTextWidth(' ')));
    const lines=[prefix+first,indent+second];
    return lines.map(line=>maxWidth===null?line:pxTruncate(line,maxWidth)).join('\n');
  }).join('\n');
}
export function translationPhoneText(result){return [result.translation,translationReplyText(result)].filter(Boolean).join('\n');}


export function hasCompleteTranslationReplies(value){
  return ['positive','negative'].every(key=>typeof value[key]==='string' && value[key].trim() && !['…','...'].includes(value[key].trim()));
}
export function mergeTranslationDisplay(previous,next,{commitReplies=false}={}){
  const result={...previous,translation:next.translation || previous.translation || ''};
  if(commitReplies && hasCompleteTranslationReplies(next)){
    result.positive=next.positive;result.negative=next.negative;
  }
  return result;
}


// Reject malformed rows and native-script source text before displaying replies.
// Script checks deliberately avoid guessing between languages that share Latin script.
export function translationRepliesMatchLanguages(value,source='auto',target='ko') {
  const script={ko:/[가-힣]/u,ja:/[ぁ-ゖァ-ヺ一-龯]/u,zh:/[一-龯]/u,th:/[ก-๛]/u};
  const native=lang=>script[lang];
  return ['positive','negative'].every(key=>{
    const lines=String(value[key] || '').split(/\r?\n|[ \t]+—[ \t]+/u).map(line=>line.trim()).filter(Boolean);
    if(lines.length!==2)return false;
    const [spoken,meaning]=lines;
    // Every first line uses Latin letters: English itself or source romanization.
    if(!/\p{Script=Latin}/u.test(spoken) || Array.from(spoken).some(char=>/\p{L}/u.test(char) && !/\p{Script=Latin}/u.test(char)))return false;
    if(native(target) && !native(target).test(meaning))return false;
    if(target==='en' && (!/[A-Za-z]/u.test(meaning) || /[가-힣ぁ-ゖァ-ヺ一-龯ก-๛]/u.test(meaning)))return false;
    return true;
  });
}
