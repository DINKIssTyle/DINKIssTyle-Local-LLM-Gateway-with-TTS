import {getTextWidth} from '@evenrealities/pretext';

function fit(text,maxWidth,suffix=''){
  if(getTextWidth(text+suffix)<=maxWidth)return text+suffix;
  const chars=Array.from(text);
  while(chars.length && getTextWidth(chars.join('')+'…'+suffix)>maxWidth)chars.pop();
  return chars.join('')+'…'+suffix;
}
// Separate native text/image containers allow inline icons without changing
// body height. Positions use the firmware font metrics, not character counts.
export function headerLayout(translate,{trigger='',source='auto',target='ko'}={}){
  source=String(source || 'auto');target=String(target || 'ko');trigger=String(trigger || '');
  const prefixText=translate?fit(`[번역] ${source.toUpperCase()}→${target.toUpperCase()}`,210):'[AI';
  const prefix={x:0,width:Math.ceil(getTextWidth(prefixText))+2,content:prefixText};
  const wakeIconX=prefix.width+4;
  const triggerText=translate?'':fit(trigger,156,']');
  const wake=translate?null:{x:wakeIconX+28,width:Math.max(20,Math.ceil(getTextWidth(triggerText))+2),content:triggerText};
  const micIconX=translate?prefix.width+4:wake.x+wake.width+4;
  const x=micIconX+28,width=396-x;
  return {prefix,wake,wakeIconX,micIconX,x,width,key:JSON.stringify([translate,prefixText,triggerText,micIconX])};
}
