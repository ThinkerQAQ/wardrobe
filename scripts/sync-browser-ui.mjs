#!/usr/bin/env node
// Single-source UI. Go embeds /web; the extension side panel packages byte-
// identical UI assets under /browser-extension/app. --check prevents drift.
import fs from 'node:fs';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..');
const src=path.join(root,'web');
const dst=path.join(root,'browser-extension/app');
const files=['index.html','favicon.svg','style.css','runtime.js','locale.js','app.js','orders.js','extension-controls.js'];
const check=process.argv.includes('--check');
if(!check)fs.mkdirSync(dst,{recursive:true});
for(const file of files){
 const from=fs.readFileSync(path.join(src,file));const to=path.join(dst,file);
 if(check){if(!fs.existsSync(to)||!from.equals(fs.readFileSync(to))){console.error('Extension shared UI differs from web/'+file);process.exitCode=1}}
 else fs.writeFileSync(to,from);
}
if(!process.exitCode)console.log(check?'Shared browser UI matches Web UI':'Synced browser extension shared UI');
