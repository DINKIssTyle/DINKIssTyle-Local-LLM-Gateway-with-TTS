"""Build a compact read-only keyword index from a Wikimedia namespace-0 title dump.
Usage: python3 scripts/build-hint-lexicon.py /path/all-titles-in-ns0.gz
No model, article body, or user transcript is downloaded by this script.
"""
import gzip,hashlib,json,re,sys
from pathlib import Path
source=Path(sys.argv[1]);root=Path(__file__).resolve().parents[1];out=root/'internal/core/data'
index={}
def add(title):
 title=title.replace('_',' ').strip()
 key=''.join(c for c in title.lower() if c.isalnum())
 if not 2<=len(key)<=40 or key.isdecimal() or re.fullmatch(r'\d+[년월일세기]',key):return
 old=index.get(key)
 if old is None or len(title)<len(old):index[key]=title
with gzip.open(source,'rt',encoding='utf-8') as f:
 for line in f:
  title=line.rstrip('\n')
  if title in ('page_title','title'):continue
  add(title)
  if ' (' in title:add(title.split(' (',1)[0])
# Product/engineering names and STT spelling aliases absent from older dumps.
for term in ['EmbeddingGemma 2','JEV','Whisper','RAG','MCP','LLM','STT','TTS','SQLite','맥북 에어 M4','맥북 에어','청동기 시대','피라미드','묵시적 갱신','유동성커버리지비율','EBITDA','LTV','온디바이스 AI','양자 컴퓨팅','생성형 AI','트랜스포머','실리콘밸리','Qwen','스마트글래스']:
 add(term)
rows=''.join(key+'\t'+index[key]+'\n' for key in sorted(index)).encode()
out.mkdir(exist_ok=True)
(out/'hint-keywords.tsv.gz').write_bytes(gzip.compress(rows,compresslevel=9,mtime=0))
info={'source':'https://dumps.wikimedia.org/kowiki/latest/kowiki-latest-all-titles-in-ns0.gz','source_sha256':hashlib.sha256(source.read_bytes()).hexdigest(),'entries':len(index),'uncompressed_bytes':len(rows),'format':'normalized key TAB display title LF; UTF-8, sorted; article titles only'}
(out/'hint-keywords.json').write_text(json.dumps(info,ensure_ascii=False,indent=2)+'\n')
print(json.dumps(info,ensure_ascii=False))
