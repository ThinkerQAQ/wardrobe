'use strict';
const hostOrigin='http://127.0.0.1:17936';
const offlineFrame=document.getElementById('offlineFrame');
const onlineFrame=document.getElementById('onlineFrame');
let connected=false;
function showOnline(){
  connected=true;
  onlineFrame.hidden=false;
  offlineFrame.hidden=true;
}
function retry(){
  if(connected)return;
  // A failed network load never blocks the extension-packaged UI.
  onlineFrame.src=hostOrigin+'/?sidepanel=1&v='+Date.now();
}
window.addEventListener('message',event=>{
  if(event.source===offlineFrame.contentWindow && event.origin===location.origin &&
     event.data?.app==='wardrobe-panel' && event.data.type==='retry'){
    retry();
    return;
  }
  if(event.source!==onlineFrame.contentWindow || event.origin!==hostOrigin ||
     event.data?.app!=='wardrobe-ui')return;
  if(event.data.type==='ready')showOnline();
});
// Recheck for a newly started local server without replacing the offline UI.
setInterval(()=>{if(!connected)retry()},12000);
