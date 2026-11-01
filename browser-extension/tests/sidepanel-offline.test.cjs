'use strict';
const test=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const vm=require('node:vm');
const extension=path.resolve(__dirname,'..');
const panel=fs.readFileSync(path.join(extension,'panel.html'),'utf8');

test('the first visible side panel frame is packaged and usable without Core',()=>{
 assert.match(panel,/<iframe id="offlineFrame"[^>]+src="app\/index\.html\?offline=1"/);
 assert.match(panel,/<iframe id="onlineFrame"[^>]+src="http:\/\/127\.0\.0\.1:17936\/\?sidepanel=1" hidden/);
 assert.doesNotMatch(panel,/id="connectionError"/);
 const manifest=JSON.parse(fs.readFileSync(path.join(extension,'manifest.json'),'utf8'));
 assert.match(manifest.content_security_policy.extension_pages,/frame-src 'self' http:\/\/127\.0\.0\.1:17936/);
 const app=fs.readFileSync(path.join(extension,'app/index.html'),'utf8');
 assert.match(app,/id="offlineNotice"/);
 assert.match(app,/id="ordersTab"/);
 assert.match(app,/id="skillsTab"/);
 const runtime=fs.readFileSync(path.join(extension,'app/runtime.js'),'utf8');
 assert.match(runtime,/if\(extensionPage\)throw Error/);
});
test('side panel preserves the bundled UI until a validated local frame is ready',()=>{
 const offlineFrame={hidden:false,contentWindow:{}};
 const onlineFrame={hidden:true,contentWindow:{}};
 const listeners=new Map();
 const intervals=[];
 const context={
  document:{getElementById:id=>({offlineFrame,onlineFrame})[id]},
  location:{origin:'chrome-extension://aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'},
  window:{addEventListener:(name,fn)=>listeners.set(name,fn)},
  chrome:{runtime:{onMessage:{addListener:()=>{}}}},
  Date:{now:()=>12345},
  setInterval:fn=>intervals.push(fn)
 };
 vm.runInNewContext(fs.readFileSync(path.join(extension,'panel.js'),'utf8'),context);
 const emit=(source,origin,data)=>listeners.get('message')({source,origin,data});
 assert.equal(offlineFrame.hidden,false);
 assert.equal(onlineFrame.hidden,true);
 emit(onlineFrame.contentWindow,'https://evil.example',{app:'wardrobe-ui',type:'ready'});
 assert.equal(onlineFrame.hidden,true,'never trust forged ready');
 emit(onlineFrame.contentWindow,'http://127.0.0.1:17936',{app:'wardrobe-ui',type:'ready'});
 assert.equal(onlineFrame.hidden,false);
 assert.equal(offlineFrame.hidden,true);
 intervals[0]();
 assert.equal(onlineFrame.hidden,false,'connected panel remains visible');
 assert.equal(offlineFrame.hidden,true);
});
test('offline retry does not obscure the packaged UI',()=>{
 const offlineFrame={hidden:false,contentWindow:{}};
 const onlineFrame={hidden:true,contentWindow:{}};
 let handler;
 let ticker;
 const context={
  document:{getElementById:id=>({offlineFrame,onlineFrame})[id]},
  location:{origin:'chrome-extension://aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'},
  window:{addEventListener:(_name,fn)=>handler=fn},
  chrome:{runtime:{onMessage:{addListener:()=>{}}}},
  Date:{now:()=>12345},
  setInterval:fn=>ticker=fn
 };
 vm.runInNewContext(fs.readFileSync(path.join(extension,'panel.js'),'utf8'),context);
 handler({source:offlineFrame.contentWindow,origin:context.location.origin,
  data:{app:'wardrobe-panel',type:'retry'}});
 assert.match(onlineFrame.src,/sidepanel=1&v=12345/);
 assert.equal(offlineFrame.hidden,false,'retry must not hide UI');
 ticker();
 assert.equal(offlineFrame.hidden,false,'automatic retry must not hide UI');
});
