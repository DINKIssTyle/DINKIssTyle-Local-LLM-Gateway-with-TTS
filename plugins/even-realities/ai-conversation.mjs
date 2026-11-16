export function routeAIUtterance(data,{waiting=false,name='AI'}={}){
  const text=String(data.text || '').trim();
  if(!text)return {kind:'ambient',waiting};
  if(data.triggered){
    const question=String(data.prompt || '').trim().replace(/^[야아][,.!?]?$/u,'');
    return question?{kind:'chat',prompt:question,waiting:false}:{kind:'ack',waiting:true,ack:`${name}: 듣는 중...`};
  }
  return waiting?{kind:'chat',prompt:text,waiting:false}:{kind:'ambient',waiting:false};
}

// Only explicit AI questions and their successful answers enter this history.
export class ChatMemory {
  constructor({maxTurns=8,maxChars=8000}={}){Object.assign(this,{maxTurns,maxChars});this.scope='';this.turns=[];}
  context(scope){if(this.scope!==scope){this.scope=scope;this.turns=[];}return this.turns.map(t=>`질문: ${t.question}\n답변: ${t.answer}`).join('\n\n');}
  remember(scope,question,answer){
    if(scope!==this.scope || !question?.trim() || !answer?.trim())return;
    this.turns.push({question:question.trim().slice(0,1600),answer:answer.trim().slice(0,3000)});
    while(this.turns.length>this.maxTurns || this.context(scope).length>this.maxChars)this.turns.shift();
  }
}
