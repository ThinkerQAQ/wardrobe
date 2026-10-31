'use strict';
chrome.storage.local.setAccessLevel({accessLevel:'TRUSTED_CONTEXTS'}).catch(()=>{});
const openWorkspace=()=>chrome.tabs.create({url:chrome.runtime.getURL('panel.html')});
if(chrome.sidePanel?.setPanelBehavior){
  chrome.sidePanel.setPanelBehavior({openPanelOnActionClick:true})
    .catch(()=>chrome.action.onClicked.addListener(openWorkspace));
}else chrome.action.onClicked.addListener(openWorkspace);
