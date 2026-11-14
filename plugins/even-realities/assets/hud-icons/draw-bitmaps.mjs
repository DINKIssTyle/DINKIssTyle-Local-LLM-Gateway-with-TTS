// Editable pixel drawing source. Running this overwrites the five PNGs.
import {writeFile} from 'node:fs/promises';
import {deflateSync} from 'node:zlib';
const W=24,H=24;
function crc32(data){let crc=0xffffffff;for(const byte of data){crc^=byte;for(let i=0;i<8;i++)crc=(crc>>>1)^((crc&1)?0xedb88320:0);}return (crc^0xffffffff)>>>0;}
function chunk(type,data){const name=Buffer.from(type),size=Buffer.alloc(4),sum=Buffer.alloc(4);size.writeUInt32BE(data.length);sum.writeUInt32BE(crc32(Buffer.concat([name,data])));return Buffer.concat([size,name,data,sum]);}
function png(pixels){
 const ihdr=Buffer.alloc(13);ihdr.writeUInt32BE(W,0);ihdr.writeUInt32BE(H,4);ihdr[8]=1; // 1-bit grayscale, black transparent
 const rows=Buffer.alloc(H*(1+W/8));for(let y=0;y<H;y++)for(let x=0;x<W;x++)if(pixels[y*W+x])rows[y*4+1+(x>>3)]|=1<<(7-x%8);
 return Buffer.concat([Buffer.from([137,80,78,71,13,10,26,10]),chunk('IHDR',ihdr),chunk('tRNS',Buffer.from([0,0])),chunk('IDAT',deflateSync(rows)),chunk('IEND',Buffer.alloc(0))]);
}
function canvas(){
 const p=new Uint8Array(W*H);const dot=(x,y,c=1)=>{if(x>=0&&x<W&&y>=0&&y<H)p[y*W+x]=c;};
 function line(x0,y0,x1,y1,c=1){const dx=Math.abs(x1-x0),dy=-Math.abs(y1-y0),sx=x0<x1?1:-1,sy=y0<y1?1:-1;let e=dx+dy;while(true){dot(x0,y0,c);if(x0===x1&&y0===y1)break;const e2=2*e;if(e2>=dy){e+=dy;x0+=sx;}if(e2<=dx){e+=dx;y0+=sy;}}}
 function path(points){for(let i=1;i<points.length;i++)line(...points[i-1],...points[i]);}
 return {p,dot,line,path};
}
function mic(c,off){
 c.path([[18,5],[20,5],[21,6],[21,12],[20,13],[18,13],[17,12],[17,6],[18,5]]);
 c.path([[15,10],[15,12],[16,15],[18,16],[20,16],[22,14],[23,12],[23,10]]);
 c.line(19,16,19,21);c.line(16,21,22,21);
 if(off){for(let y=4;y<=21;y++){const x=Math.round(14+(y-4)*9/17);for(let d=-1;d<=1;d++)c.dot(x+d,y,0);}c.line(14,4,23,21);c.line(15,4,23,20);}
}
function glasses(c){c.path([[1,10],[3,7],[11,7],[13,10]]);c.path([[1,11],[6,11],[6,15],[2,15],[1,11]]);c.path([[8,11],[13,11],[12,15],[8,15],[8,11]]);c.line(6,12,8,12);}
function phone(c){c.path([[3,2],[10,2],[11,3],[11,20],[10,21],[3,21],[2,20],[2,3],[3,2]]);c.line(5,4,8,4);c.line(5,19,8,19);}
for(const source of ['glasses','phone'])for(const state of ['on','off']){const c=canvas();(source==='phone'?phone:glasses)(c);mic(c,state==='off');await writeFile(new URL(`${source}-mic-${state}.png`,import.meta.url),png(c.p));}
const ai=canvas();ai.path([[3,4],[20,4],[20,17],[10,17],[5,21],[5,17],[3,17],[3,4]]);ai.line(7,9,7,12);ai.line(11,7,11,14);ai.line(15,9,15,12);await writeFile(new URL('ai-trigger.png',import.meta.url),png(ai.p));
