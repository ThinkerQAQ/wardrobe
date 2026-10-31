'use strict';
const test=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const manifest=JSON.parse(fs.readFileSync('browser-extension/manifest.json','utf8'));

test('manifest uses the same Wardrobe brand icon at toolbar and extension sizes',()=>{
 assert.equal(manifest.name,'Wardrobe');
 for(const size of [16,32,48,128]){
  const icon=manifest.icons[String(size)];
  assert.equal(icon,manifest.action.default_icon[String(size)]);
  const bytes=fs.readFileSync(path.join('browser-extension',icon));
  assert.equal(bytes.subarray(1,4).toString(),'PNG');
  assert.equal(bytes.readUInt32BE(16),size);
  assert.equal(bytes.readUInt32BE(20),size);
 }
});
test('Web favicon, header and extension use a shared mark',()=>{
 const source=fs.readFileSync('brand/wardrobe-mark.svg');
 assert.deepEqual(fs.readFileSync('web/favicon.svg'),source);
 assert.deepEqual(fs.readFileSync('browser-extension/app/favicon.svg'),source);
 assert.deepEqual(fs.readFileSync('browser-extension/icons/icon.svg'),source);
 const html=fs.readFileSync('web/index.html','utf8');
 assert.match(html,/<link rel="icon" href="\.\/favicon\.svg"/);
 assert.match(html,/class="brandMark" src="\.\/favicon\.svg"/);
});
test('packaged Windows ICO has valid small/medium/large assets',()=>{
 const data=fs.readFileSync('brand/wardrobe.ico');
 assert.equal(data.readUInt16LE(2),1);
 assert.ok(data.readUInt16LE(4)>=4);
});
