const test=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const root=path.resolve(__dirname,'../..');
test('public browser extension has no retailer capture permissions',()=>{
 const mf=JSON.parse(fs.readFileSync(path.join(root,'browser-extension/manifest.json'),'utf8'));
 assert.deepEqual(mf.host_permissions,['http://127.0.0.1:17936/*']);
 assert.deepEqual(mf.content_scripts,[]);
 assert.ok(!mf.permissions.includes('cookies'));
 assert.ok(!mf.permissions.includes('webRequest'));
 assert.ok(!mf.permissions.includes('scripting'));
 for(const name of ['collector-main.js','collector-content.js','order-parser.js','bridge-local.js']){
  assert.equal(fs.existsSync(path.join(root,'browser-extension',name)),false);
 }
});
