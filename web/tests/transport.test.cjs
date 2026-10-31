'use strict';
const test=require('node:test');const assert=require('node:assert/strict');
const fs=require('node:fs');const vm=require('node:vm');
const source=fs.readFileSync('web/runtime.js','utf8');
function harness(extension){
 const events=[];const store={};const origin=extension?'chrome-extension:':'http:';
 const context={location:{protocol:origin},Headers,URL,setTimeout,Promise,document:{documentElement:{classList:{add:()=>{}}}},
  addEventListener:()=>{},
  fetch:async(url,opts={})=>{events.push({url,headers:new Headers(opts.headers||{}),method:opts.method||'GET'});return {ok:true,status:200,json:async()=>({status:'ok'}),text:async()=>'',blob:async()=>new Blob(['image'],{type:'image/png'})}},
  chrome:{storage:{local:{get:async key=>({[key]:store[key]}),set:async data=>Object.assign(store,data),remove:async key=>delete store[key]}}},
 };
 context.self=context;context.top=context;context.globalThis=context;vm.runInNewContext(source,context);return {transport:context.WardrobeTransport,events,store};
}
test('Web host uses existing same-origin route, no extension key',async()=>{
 const h=harness(false);assert.equal(h.transport.host,'web');
 await h.transport.request('/api/products');
 assert.equal(h.events[0].url,'/api/products');
 assert.equal(h.events[0].headers.has('X-Wardrobe-Import-Key'),false);
});
test('shared localhost transport removes manual pairing',async()=>{
 const h=harness(false);
 await h.transport.request('/api/products');
 assert.equal(h.events.at(-1).url,'/api/products');
 assert.equal(typeof h.transport.pair,'undefined');
});
