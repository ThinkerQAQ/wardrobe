'use strict';
const test=require('node:test');const assert=require('node:assert/strict');const fs=require('node:fs');const path=require('node:path');
const manifest=JSON.parse(fs.readFileSync('browser-extension/manifest.json','utf8'));
const html=fs.readFileSync('web/index.html','utf8');
test('browser toolbar opens BlogCTL-like full Wardrobe side panel, not order-only popup',()=>{
 assert.equal(manifest.name,'Wardrobe');
 assert.equal(manifest.side_panel.default_path,'panel.html');
 assert.ok(manifest.permissions.includes('sidePanel'));
 assert.equal('default_popup' in manifest.action,false);
 assert.equal(fs.existsSync('browser-extension/popup.html'),false);
 const script=fs.readFileSync('browser-extension/background.js','utf8');
 assert.match(script,/setPanelBehavior/);assert.match(script,/openPanelOnActionClick:true/);
});
test('Go web and packaged extension share the byte-identical UI',()=>{
 for(const n of ['index.html','style.css','runtime.js','locale.js','app.js','orders.js','extension-controls.js']){
  assert.deepEqual(fs.readFileSync(path.join('web',n)),fs.readFileSync(path.join('browser-extension/app',n)),n);
 }
 assert.match(html,/id="ordersTab"/);assert.match(html,/id="wardrobeTab"/);assert.match(html,/id="hairTab"/);assert.match(html,/id="bodyTab"/);assert.match(html,/id="looksTab"/);
});
test('shared UI supports manual Chinese/English and auto locale through local API',()=>{
 assert.match(html,/id="wardrobeUiLocale"/);
 assert.match(html,/<option value="zh-CN">/);
 assert.match(html,/<option value="en">English/);
 const locale=fs.readFileSync('web/locale.js','utf8');
 assert.match(locale,/WardrobeTransport\.request\('\/api\/ui-locale'/);
 assert.match(locale,/MutationObserver/);
 assert.match(locale,/orderInfo/); // user product data must not be rewritten
});
test('shared transport uses local origin; optional source bridges are build-time additions',()=>{
 const runtime=fs.readFileSync('web/runtime.js','utf8');
 assert.match(runtime,/http:\/\/127\.0\.0\.1:17936/);
 assert.match(runtime,/same-origin/);
 assert.doesNotMatch(runtime,/chrome\.storage\.local/);
 assert.match(runtime,/Non-local API URL blocked/);
 const popup=fs.readFileSync('web/extension-controls.js','utf8');
 if(manifest.content_scripts.length)assert.match(popup,/syncPlatform/);
 else assert.doesNotMatch(popup,/syncPlatform/);
 assert.doesNotMatch(popup,/wardrobe:paired/);
});
