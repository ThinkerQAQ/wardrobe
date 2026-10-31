'use strict';
// The UI and MCP both invoke the Go lookbook compositor.
(function(){
const $=s=>document.querySelector(s),keys=['outer','mid','pants','shoes'];
const canvas=$('#card'),ctx=canvas.getContext('2d');
let products=[],looks=[],hair=[],index={},lookIndex={},hairIndex={},latest=null;
function status(s,err=false){$('#status').textContent=s;$('#status').style.color=err?'#b24736':'#345548'}
function options(sel,rows,empty){const e=$(sel);e.replaceChildren(new Option(empty,''));for(const x of rows)e.add(new Option(x.id+' · '+String(x.name||x.scene||'未命名').slice(0,32),x.id))}
function updateLook(){const l=lookIndex[$('#look').value];if(!l)return;for(const k of keys){const id=l.slots?.[k]||(k==='mid'?l.slots?.inner:'');$('#'+k).value=index[id]?id:''}$('#hair').value=hairIndex[l.slots?.hair]?l.slots.hair:''}
async function portrait(){
 const f=$('#portrait').files[0];
 if(f){if(f.size>8*1024*1024||!['image/jpeg','image/png'].includes(f.type))throw Error('只支持不超过 8 MiB 的 JPEG/PNG 人物图');
   const b=await new Promise((res,rej)=>{const r=new FileReader();r.onload=()=>res(r.result);r.onerror=()=>rej(Error('读取失败'));r.readAsDataURL(f)});
   return {portraitImageBase64:b};
 }
 const l=lookIndex[$('#look').value];
 if(l?.photo&&/^\/media\/photos\/LOOK-[a-zA-Z0-9_-]+\.jpg$/.test(l.photo))return {portraitLookId:l.id};
 const p=new URLSearchParams(location.search).get('photo');
 if(p&&/^LOOK-[a-zA-Z0-9_-]+$/.test(p))return {portraitLookId:p};
 throw Error('上传人物图，或选择有归档照片的搭配');
}
async function render(){
 $('#generate').disabled=true;$('#download').disabled=true;latest=null;status('由本地 Go 服务合成固定画册…');
 try{
  const input={...await portrait(),lookId:$('#look').value,slots:Object.fromEntries(keys.map(k=>[k,$('#'+k).value])),hairId:$('#hair').value};
  const r=await fetch('/api/lookbook/render',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(input)});
  if(!r.ok){let s=await r.text();try{s=JSON.parse(s).error||s}catch{};throw Error(s)}
  const blob=await r.blob(),url=URL.createObjectURL(blob),img=new Image();
  try{
   await new Promise((res,rej)=>{img.onload=res;img.onerror=()=>rej(Error('图片解码失败'));img.src=url});
   if(img.width!==1200||img.height!==1800)throw Error('输出尺寸不正确');
   ctx.clearRect(0,0,1200,1800);ctx.drawImage(img,0,0);latest=blob;$('#download').disabled=false;
   status('本地固定版式已完成；人物身份、服饰与站姿仍需人工核验。');
  }finally{URL.revokeObjectURL(url)}
 }catch(e){status(e.message,true)}finally{$('#generate').disabled=false}
}
function download(){if(!latest)return;const u=URL.createObjectURL(latest),a=document.createElement('a');a.href=u;a.download='wardrobe-lookbook-card-v1.png';a.click();setTimeout(()=>URL.revokeObjectURL(u),1500)}
async function start(){try{
 const load=async k=>{const r=await fetch('/api/'+k);if(!r.ok)throw Error('读取 '+k+' 失败');return r.json()};
 [products,looks,hair]=await Promise.all(['products','looks','hairstyles'].map(load));
 index=Object.fromEntries(products.map(x=>[x.id,x]));lookIndex=Object.fromEntries(looks.map(x=>[x.id,x]));hairIndex=Object.fromEntries(hair.map(x=>[x.id,x]));
 options('#look',looks,'手动选择');for(const k of keys)options('#'+k,products.filter(x=>x.image),'请选择');options('#hair',hair,'无发型');
 const p=new URLSearchParams(location.search);
 if(lookIndex[p.get('lookId')]){$('#look').value=p.get('lookId');updateLook()}
 for(const k of [...keys,'hair']){const id=p.get(k);if(id&&((k==='hair'?hairIndex:index)[id]))$('#'+k).value=id}
 $('#look').addEventListener('change',updateLook);$('#generate').addEventListener('click',render);$('#download').addEventListener('click',download);
 status('已读取本地衣橱。上传单幅人物图后即可生成。');if(p.has('lookId')||keys.every(k=>p.has(k)))await render();
 }catch(e){status(e.message,true)}}
 document.addEventListener('DOMContentLoaded',start);
})();
