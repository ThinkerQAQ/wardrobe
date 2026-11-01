'use strict';
const hostOrigin='http://127.0.0.1:17936';
const offlineFrame=document.getElementById('offlineFrame');
const onlineFrame=document.getElementById('onlineFrame');
const nativeHost='com.thinkerqaq.wardrobe';
let connected=false,starting=false;
function showOnline(){connected=true;onlineFrame.hidden=false;offlineFrame.hidden=true}
function retry(){if(!connected)onlineFrame.src=hostOrigin+'/?sidepanel=1&v='+Date.now()}
function report(ok,error){
 offlineFrame.contentWindow?.postMessage({app:'wardrobe-launch',type:'result',ok,error},location.origin);
}
function startLocal(){
 if(starting||connected)return;
 starting=true;
 chrome.runtime.sendNativeMessage(nativeHost,{command:'ensure_bridge'},response=>{
  starting=false;
  const error=chrome.runtime.lastError?.message;
  if(error||!response?.ok){
   report(false,error||response?.error||'Native Host returned no confirmation');
   return;
  }
  report(true);
  retry();
 });
}
window.addEventListener('message',event=>{
 if(event.source===offlineFrame.contentWindow && event.origin===location.origin &&
    event.data?.app==='wardrobe-panel'){
   if(event.data.type==='retry')retry();
   if(event.data.type==='start')startLocal();
   return;
 }
 if(event.source===onlineFrame.contentWindow && event.origin===hostOrigin &&
    event.data?.app==='wardrobe-ui' && event.data.type==='ready')showOnline();
});
// Wait for the packaged frame to load so startup errors are visible.
if(offlineFrame.contentDocument?.readyState==='complete')startLocal();
else offlineFrame.addEventListener('load',startLocal,{once:true});
setInterval(()=>{if(!connected)retry()},12000);
