'use strict';
const test=require('node:test');const assert=require('node:assert/strict');const fs=require('node:fs');
const html=fs.readFileSync('web/index.html','utf8');
const js=fs.readFileSync('web/orders.js','utf8');
const controls=fs.readFileSync('web/extension-controls.js','utf8');
const hasAdapter=JSON.parse(fs.readFileSync('browser-extension/manifest.json','utf8')).content_scripts.length>0;
const app=fs.readFileSync('web/app.js','utf8');
const css=fs.readFileSync('web/style.css','utf8');
const section=html.match(/<section id="ordersPanel"[\s\S]*?<section id="skillsPanel"/)?.[0];
test('orders toolbar focuses on website synchronization, not uploading files',()=>{
 assert.ok(section);
 const position=id=>section.indexOf('id="'+id+'"');
 assert.ok(position('ordersSyncToggle')<position('ordersSyncPanel'));
 assert.ok(position('ordersSyncPanel')<position('ordersStats'));
 assert.ok(position('ordersStats')<position('ordersSearch'));
 assert.ok(position('ordersSearch')<position('ordersStatus'));
 assert.ok(position('ordersStatus')<position('ordersList'));
 assert.match(section,/id="ordersSyncPanel"[^>]*hidden/);
 assert.match(section,/id="ordersSyncToggle"[^>]*aria-expanded="false"/);
 assert.doesNotMatch(section,/ordersImportPlatform|ordersFile|ordersImportPanel|ordersExtensionGuide|安装 Edge 扩展|\.xlsx|\.csv|\.json/);
});
test('both platform buttons start background sync; stop is explicit',()=>{
 assert.match(section,/id="ordersSyncJD"/);
 assert.match(section,/id="ordersSyncTB"/);
 assert.match(section,/id="ordersSyncStop"/);
 assert.match(js,/wardrobe:sync-order/);
 assert.match(js,/wardrobe:stop-order-sync/);
 (hasAdapter?assert.match:assert.doesNotMatch)(controls,/normalizedOrderBatch/);
 (hasAdapter?assert.match:assert.doesNotMatch)(controls,/orders\/import/);
 (hasAdapter?assert.match:assert.doesNotMatch)(controls,/wardrobe:sync-order/);
 assert.match(app,/\$\('#title'\)\.textContent='订单库'/);
});
test('desktop and narrow extension layouts keep sources, stats and filters usable',()=>{
 assert.match(css,/\.ordersSyncSources\{display:grid/);
 assert.match(css,/\.workspaceMain \.orderStats\{display:grid/);
 assert.match(css,/\.workspaceMain \.orderFilters\{display:grid/);
 assert.match(css,/@media\(max-width:760px\)/);
 assert.match(css,/\.extension-host \.ordersSyncSources\{grid-template-columns:minmax\(0,1fr\)/);
 assert.match(css,/\.extension-host \.workspaceMain \.orderStats\{grid-template-columns:repeat\(2/);
});

test('inventory actions are on Wardrobe page, never inside Orders',()=>{
 const inventory=html.match(/<section id="wardrobeCatalogTools"[\s\S]*?<section id="filters"/)?.[0];
 assert.ok(inventory);
 assert.match(inventory,/id="ordersOrganize"/);
 assert.match(inventory,/id="ordersCacheImages"/);
 assert.match(inventory,/id="ordersCatalogStatus"/);
 assert.doesNotMatch(section,/id="ordersOrganize"|id="ordersCacheImages"|id="ordersCatalogStatus"/);
 assert.match(app,/\$\('#wardrobeCatalogTools'\)\.hidden=tab!=='wardrobe'/);
 assert.match(js,/orders\/organize/);
 assert.match(js,/orders\/cache-images/);
 assert.match(js,/x\.local_image/);
 assert.match(js,/classList\.add\('orderThumbnail'\)/);
 (hasAdapter?assert.match:assert.doesNotMatch)(controls,/orderUI\?\.organize\(\{auto:true\}\)/);
 assert.doesNotMatch(controls,/orderUI\?\.cacheImages\(\{auto:true\}\)/);
 assert.doesNotMatch(js,/ordersOrganize'\)\.onclick=async.*cacheImages/);
});
