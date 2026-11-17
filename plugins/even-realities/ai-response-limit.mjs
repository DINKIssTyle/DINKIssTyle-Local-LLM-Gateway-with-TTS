// Last resort for models that ignore the requested limit. Prefer a whole sentence.
export function limitHUDResponse(text,max=500){
 const chars=Array.from(String(text || '').trim());if(chars.length<=max)return chars.join('');
 const head=chars.slice(0,max-1).join('');
 const ends=[...head.matchAll(/[.!?。！？](?=\s|$)|\n/gu)];
 const end=ends.at(-1);return (end && end.index>=Math.floor(max*.6)?head.slice(0,end.index+end[0].length).trimEnd():head.trimEnd())+'…';
}
