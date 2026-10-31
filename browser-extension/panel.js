'use strict';
const hostOrigin='http://127.0.0.1:17936';
const frame=document.getElementById('wardrobeFrame');
const error=document.getElementById('connectionError');
let connected=false,pending=null;
const showError=()=>{if(!connected)error.hidden=false};
function retry(){
  connected=false;error.hidden=true;
  frame.src=hostOrigin+'/?sidepanel=1&v='+Date.now();
  clearTimeout(pending);pending=setTimeout(showError,6000);
}
window.addEventListener('message',event=>{
  if(event.source!==frame.contentWindow || event.origin!==hostOrigin || event.data?.app!=='wardrobe-ui')return;
  if(event.data.type==='ready'){connected=true;clearTimeout(pending);error.hidden=true}
});
document.getElementById('retry').onclick=retry;
document.getElementById('openWeb').onclick=()=>chrome.tabs.create({url:hostOrigin+'/'});
pending=setTimeout(showError,6500);
