import {getTextWidth} from '@evenrealities/pretext';
function fit(text,maxWidth,suffix=''){
 if(getTextWidth(text+suffix)<=maxWidth)return text+suffix;
 const chars=Array.from(text);while(chars.length && getTextWidth(chars.join('')+'…'+suffix)>maxWidth)chars.pop();return chars.join('')+'…'+suffix;
}
// Three 24px bitmaps share the existing 27px header. Clock starts at x=400.
export function headerLayout(translate,{trigger='',source='auto',target='ko'}={}){
 source=String(source || 'auto');target=String(target || 'ko');trigger=String(trigger || '');
 const prefix={x:0,width:Math.ceil(getTextWidth('['))+2,content:'['};
 const modeIconX=prefix.width+2;
 const tail=translate?fit(`] ${source.toUpperCase()}→${target.toUpperCase()}`,170):fit(trigger,156,']');
 const wake={x:modeIconX+28,width:Math.max(20,Math.ceil(getTextWidth(tail))+2),content:tail};
 const micIconX=wake.x+wake.width+4,stateIconX=micIconX+28,x=stateIconX+28,width=396-x;
 return {prefix,wake,modeIconX,micIconX,stateIconX,x,width,key:JSON.stringify([translate,tail,modeIconX,micIconX,stateIconX])};
}
