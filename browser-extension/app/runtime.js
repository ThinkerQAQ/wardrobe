'use strict';
// The extension-bundled UI stays available when the local Go process is down.
// Connected data views still run in the loopback origin, preserving the existing
// same-origin API and SQLite access rules without exposing a pairing secret.
(function(root){
 const extensionPage=root.location.protocol==='chrome-extension:';
 const embedded=root.self!==root.top;
 const base='http://127.0.0.1:17936';
 if(embedded)document.documentElement.classList.add('extension-host');
 async function request(path,init={}){
  if(typeof path!=='string'||!path.startsWith('/api/'))throw Error('Non-local API URL blocked');
  if(extensionPage)throw Error('Local Wardrobe service is offline');
  return fetch(path,{...init,credentials:'same-origin'});
 }
 async function check(){try{const r=await request('/api/health');return r.ok}catch{return false}}
 async function media(path){
  if(typeof path!=='string')return '';
  if(path.startsWith('/media/')&&/^\/media\/(images|photos|hairstyles)\/[A-Za-z0-9_.-]+\.(jpg|png|webp)(\?[^#]*)?$/.test(path))return path;
  if(path.startsWith('data:image/'))return path;
  if(path.startsWith('https://'))return path;
  return '';
 }
 async function exportBackup(){
  const r=await request('/api/export');if(!r.ok)throw Error('Local backup unavailable');
  const obj=URL.createObjectURL(await r.blob());const a=document.createElement('a');
  a.href=obj;a.download='wardrobe-backup.json';document.body.appendChild(a);a.click();a.remove();
  setTimeout(()=>URL.revokeObjectURL(obj),45000);
 }
 root.WardrobeTransport=Object.freeze({host:extensionPage?'extension':embedded?'embedded':'web',request,check,media,exportBackup,base});
 if(embedded)root.addEventListener('load',()=>root.parent.postMessage({app:'wardrobe-ui',type:'ready'},'*'));
})(globalThis);
