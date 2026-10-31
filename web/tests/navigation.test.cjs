'use strict';
const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs');
const html=fs.readFileSync('web/index.html','utf8'),js=fs.readFileSync('web/app.js','utf8');
test('desktop and extension use ordered icon navigation',()=>{
 const nav=html.match(/<nav class="appNav"[^>]*>(.*?)<\/nav>/s)?.[1];assert.ok(nav);
 const ids=[...nav.matchAll(/id="([a-zA-Z]+Tab)"/g)].map(m=>m[1]);
 assert.deepEqual(ids,['ordersTab','wardrobeTab','hairTab','bodyTab','looksTab','skillsTab']);
 assert.equal((nav.match(/class="navIcon"/g)||[]).length,6);
 assert.equal((nav.match(/viewBox="0 0 24 24"/g)||[]).length,6);
 assert.match(html,/class="appShell"/);assert.match(html,/class="workspaceMain"/);
});
test('orders is the default route with no manual pairing',()=>{
 assert.match(js,/tab='orders'/);assert.match(html,/id="ordersTab" class="navButton active"/);
 assert.match(js,/if\(tab==='orders'\)await window\.orderUI\?\.load\(\)/);
 assert.doesNotMatch(html,/id="extensionPairKey"|id="ordersPair"|连接本地应用/);
});
test('website sync is collapsed by default and no file-install tutorial exists',()=>{
 assert.match(html,/id="ordersSyncToggle"[^>]*aria-expanded="false"/);
 assert.match(html,/id="ordersSyncPanel"[^>]*hidden/);
 assert.doesNotMatch(html,/ordersExtensionGuide|ordersFileName|ordersImportToggle/);
 assert.doesNotMatch(html,/[A-Z]:\\(?:Users|software)\\/);
});
