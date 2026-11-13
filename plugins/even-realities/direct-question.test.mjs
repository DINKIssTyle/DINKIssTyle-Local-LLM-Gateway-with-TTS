import assert from 'node:assert/strict';
import {directQuestionIntent} from './direct-question.mjs';
for(const text of ['청동기 시대는 언제부터 언제까지인가요','그가 피라미드라고 말했어. 그게 무엇인가요','피라미드는 누가 지었어','언제 시작했어','몇 시야','이 두 제품은 뭐가 달라?','12만원에서 15퍼센트 할인하면 얼마지','그 뜻을 알려줘','그 내용을 검색 해 줘','이걸 설명해 주세요','그 부분을 말해줘','What is a pyramid?','Tell me about pyramids'])assert.equal(directQuestionIntent(text).direct,true,text);
for(const text of ['피라미드 영상이 나오네','청동기 시대 얘기를 하고 있어','응, 오늘도 수고했어','진짜?','밥 먹었어?','오늘 날씨 어때?','내일 갈까?','그거 괜찮아요?','그가 검색해줘라고 말했어','그 사람이 언제냐고 물었어','이 기능은 검색해줘라는 버튼이 있어','이 내용은 검색해줘야 해','What a nice day'])assert.equal(directQuestionIntent(text).direct,false,text);
assert.equal(directQuestionIntent('최근 뉴스 검색해줘').search,true);
assert.equal(directQuestionIntent('인터넷에서 찾아봐줘').search,true);
assert.equal(directQuestionIntent('저장된 기억에서 찾아줘').search,false);
console.log('PASS: direct questions, information requests, explicit web search, reported speech and statements excluded');
