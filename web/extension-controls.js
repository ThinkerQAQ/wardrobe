'use strict';
// The browser panel and local Web UI share the same local Core.
(function(){
 if(WardrobeTransport.host==='embedded'){
  const button=document.getElementById('extensionOpenWorkspace');
  button.hidden=false;
  button.onclick=()=>window.open(WardrobeTransport.base+'/','_blank','noopener');
 }
})();
