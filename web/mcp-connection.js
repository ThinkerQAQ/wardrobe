'use strict';
(function(root){
 const PATTERN=/^tunnel_(?:[a-z0-9]{32}|[a-z0-9]{4}_[a-z0-9]{32})$/;
 const EXAMPLE='tunnel_0123456789abcdef0123456789abcdef';
 const $=id=>document.getElementById(id);
 const t=text=>root.WardrobeI18n?.translate(text)||text;
 const offline=()=>root.WardrobeTransport?.host==='extension';
 const mcpActive=()=>!!$('mcpPanel')&&!$('mcpPanel').hidden;
 const errorMessage=e=>String(e?.message||e);
 async function request(method,body){
   if(offline())throw Error('请先启动本地 Wardrobe 服务。');
   const response=await root.WardrobeTransport.request('/api/mcp-connection',{
     method,...(body?{headers:{'Content-Type':'application/json'},body:JSON.stringify(body)}:{})
   });
   if(!response.ok){
     let reason=(await response.text()).slice(0,400);
     throw Error(reason||'MCP request failed');
   }
   return response.json();
 }
 const readMCP=async()=>({connection:await request('GET')});
 const sendMCP=async input=>request('POST',{
   action:input.action,
   ...(input.action==='save'?{
     tunnel_id:input.tunnelId,runtime_api_key:input.runtimeApiKey,
     reuse_key:input.reuseKey,replace:input.replace,
     previous_tunnel_id:input.previousTunnelId
   }:{})
 });
 const state={status:{},busy:false,pending:null,editedID:false,editedKey:false,autoStart:false,autoKnown:false,initialized:false,stopPoll:null};
 const validID=id=>PATTERN.test(id)&&id!==EXAMPLE;
 const running=s=>Boolean(s.running||s.ready);
 const controls=(s,{id,key,busy=false,pending=false}={})=>{
  const changed=Boolean((id||'')!==(s.tunnel_id||'')||key);
  const stored=Boolean(s.configured&&s.runtime_key_saved&&validID(s.tunnel_id));
  const active=running(s);
  return {
   canSave:!busy&&!pending,
   canStart:!active&&stored&&!changed&&!busy&&!pending,
   canStop:active&&!busy&&!pending,
   canRestart:active&&stored&&!changed&&!busy&&!pending,
   canAutoStart:stored&&!changed&&!busy&&!pending,
   changed,active,stored
  };
 };
 function message(text,error=false) {
  if(!$('mcpConnectionMessage'))return;
  $('mcpConnectionMessage').textContent=text||'';
  $('mcpConnectionMessage').className=error?'mcp-message error':'mcp-message';
 }
 function form() {
  const input=$('mcpTunnelID'), key=$('mcpRuntimeKey');
  return {id:input?.value?.trim()||'',key:key?.value?.trim()||'',busy:state.busy,pending:!!state.pending};
 }
 function show() {
  if(!$('mcpConnectionSettings'))return;
  const s=state.status,c=controls(s,form()),run=running(s);
  $('mcpSave').disabled=!c.canSave;
  if($('mcpInstallClient')) {
   $('mcpInstallClient').hidden=!!s.client_installed;
   $('mcpInstallClient').disabled=state.busy||!!state.pending;
  }
  if($('mcpClientHint'))$('mcpClientHint').textContent=s.client_installed?t('官方 Tunnel Client 已安装'):t('官方 Tunnel Client 尚未安装（按需下载）');
  $('mcpConnect').disabled=run?!c.canStop:!c.canStart;
  $('mcpConnect').textContent=run?t('停止服务'):t('启动服务');
  $('mcpRestart').disabled=!c.canRestart;
  const connectPref=$('mcpAutoConnect');
  if(connectPref){
   connectPref.disabled=!c.canAutoStart||!s.configured;
   connectPref.checked=s.tunnel_auto_connect!==false;
  }
  const toggle=$('mcpTunnelToggle');
  if(toggle) {
   const connecting=Boolean(s.tunnel_connecting||s.ready);
   toggle.textContent=connecting?t('断开 Tunnel'):t('连接 Tunnel');
   toggle.disabled=state.busy||!!state.pending||!run||(!connecting&&c.changed)||(!connecting&&s.restart_required);
  }
  if($('mcpAutoStart')) {
   $('mcpAutoStart').disabled=!c.canAutoStart||!state.autoKnown;
   $('mcpAutoStart').checked=state.autoStart;
  }
  let label=t('未配置'),kind='disabled';
  if(state.pending){label=t('正在保存');kind='checking';}
  else if(s.restart_required&&run){label=t('等待重启');kind='checking';}
  else if(s.ready){label='Ready';kind='ok';}
  else if(run&&!s.tunnel_connecting){label=t('本地服务运行中 · Tunnel 未连接');kind='disabled';}
  else if(run){label=t('正在启动 / 未就绪');kind='checking';}
  else if(s.configured){label=t('已停止');kind='disabled';}
  $('mcpConnectionStatusText').textContent=label;
  const statusChip=$('mcpConnectionStatus');
  const chipKind=kind==='ok'?'ready':kind==='checking'?'pending':'idle';
  if(statusChip.dataset) statusChip.dataset.kind=chipKind;
  else if(statusChip.setAttribute) statusChip.setAttribute('data-kind',chipKind);
  $('mcpPluginDescription').textContent=s.restart_required&&run
    ? t('已保存的修改等待重启生效。重启服务后再到 ChatGPT 中管理插件。')
    : s.ready
      ? t('本地 Tunnel 已就绪。请在 ChatGPT 中创建或管理插件，授权结果以 ChatGPT 为准。')
      : run&&!s.tunnel_connecting
        ? t('本地服务正在运行，但 Tunnel 未连接。点击「连接 Tunnel」，等待就绪后再到 ChatGPT 管理插件。')
        : t('先启动本地 Tunnel 并等待就绪，再到 ChatGPT 中创建或管理插件。');
  $('mcpTunnelHint').textContent=s.configured?t('已保存；可修改后再次保存'):t('尚未保存');
  $('mcpRuntimeHint').textContent=s.runtime_key_saved?t('已保存；留空则复用'):t('尚未保存');
  const reveal=$('mcpKeyReveal');
  if(reveal) {
   reveal.disabled=!$('mcpRuntimeKey').value;
   if(reveal.disabled) {
    $('mcpRuntimeKey').type='password';
    reveal.setAttribute?.('aria-pressed','false');
    reveal.setAttribute?.('aria-label',t('显示输入的密钥'));
   }
  }
 }
 function intendedInput(el,value) {
  if(!el)return;
  el.value=value;
  el.readOnly=true;
  el.autocomplete='off';
  el.setAttribute('data-lpignore','true');
  el.setAttribute('data-1p-ignore','true');
  el.setAttribute('data-bwignore','true');
  const unlock=()=>{el.readOnly=false;};
  for(const evt of ['keydown','pointerdown','paste'])el.addEventListener(evt,unlock);
  el.addEventListener('focus',()=>{
    const intended=el.id==='mcpTunnelID'
      ? (state.editedID?el.value:(state.status.tunnel_id||''))
      : (state.editedKey?el.value:'');
    if(el.value!==intended)el.value=intended;
  });
  el.addEventListener('input',()=>{
   if(document.activeElement!==el||el.matches?.(':-webkit-autofill')) {
    el.value=value;return;
   }
   if(el.id==='mcpTunnelID')state.editedID=true;
   else state.editedKey=true;
   message('');
   show();
  });
 }
 async function refresh(){
  if(state.busy)return;
  try{
   const result=await readMCP();
   const s=result.connection||{};
   state.status=s;
   if(!$('mcpConnectionSettings'))return;
   if(!state.editedID&&document.activeElement!==$('mcpTunnelID'))$('mcpTunnelID').value=s.tunnel_id||'';
   $('mcpRuntimeKey').placeholder=s.runtime_key_saved?'****************':'';
   $('mcpLocalConfig').textContent=JSON.stringify(s.local_config||{},null,2);
   const pending=state.pending;
   if(pending){
    if(s.setup_state==='failed'){state.pending=null;message(t('保存失败，之前的配置已保留。'),true);}
    else if(s.setup_state==='succeeded'&&s.revision&&s.revision!==pending.revision&&s.tunnel_id===pending.id){
     state.pending=null;
     state.editedID=false;state.editedKey=false;
     $('mcpTunnelID').value=s.tunnel_id;
     $('mcpRuntimeKey').value='';
     message(t('配置已保存。'));
    }else if(Date.now()>pending.deadline){state.pending=null;message(t('保存超时；请核对 Tunnel ID 和 Runtime API Key。'),true);}
   }
   show();
  }catch(e){message(errorMessage(e),true);}
 }
 async function action(action){
  if(state.busy||state.pending)return;
  if(action==='install-client'){
   state.busy=true;show();
   try {
    await sendMCP({action:'install-client'});
    message(t('官方 Tunnel Client 已下载并校验；不会自动启动服务。'));
   } catch(e){message(errorMessage(e),true);}
   finally{state.busy=false;await refresh();}
   return;
  }
  const s=state.status,c=controls(s,form());
  if(action==='start'&&!c.canStart||action==='stop'&&!c.canStop||
     action==='restart'&&!c.canRestart)return;
  const id=$('mcpTunnelID').value.trim();
  // Ignore silent password-manager injection: only user-initiated input may rotate the key.
  const key=state.editedKey ? $('mcpRuntimeKey').value.trim() : '';
  if(action==='save'&&!state.editedID&&id!==(s.tunnel_id||'')){
   $('mcpTunnelID').value=s.tunnel_id||'';
   message(t('检测到非手动填入的 Tunnel ID，已恢复原值。'),true);
   show();return;
  }
  if(action==='save'){
   if(!validID(id)){message(t('请输入有效的非测试 Tunnel ID。'),true);return;}
   if(!key&&!s.runtime_key_saved){message(t('请输入 Runtime API Key。'),true);return;}
   if(id===s.tunnel_id&&!key&&s.configured&&s.runtime_key_saved){
    state.editedID=false;state.editedKey=false;$('mcpRuntimeKey').value='';
    message(t('原有配置已确认，无需再次写入。'));
    show();return;
   }
  }
  state.busy=true;show();
  try{
   if(action==='save'&&!s.client_installed){
    await sendMCP({action:'install-client'});
   }
   await sendMCP({
    action,tunnelId:id,runtimeApiKey:action==='save'?key:'',
    reuseKey:action==='save'&&!key,
    replace:action==='save'&&!!s.configured,previousTunnelId:s.tunnel_id||''
   });
   if(action==='save'){
    state.pending={id,revision:s.revision||'',deadline:Date.now()+120000};
    if($('mcpRuntimeKey'))$('mcpRuntimeKey').value='';
    message(t('已提交配置校验；保存成功前不会启动服务。'));
   }else message(t('操作已提交，正在刷新服务状态。'));
  }catch(e){message(errorMessage(e),true);}
  finally{state.busy=false;await refresh();}
 }
 async function autoStatus(){
  try{
   const r=await sendMCP({action:'auto-status'});
   state.autoStart=!!r.auto_start;state.autoKnown=true;
    if($('mcpStartupError'))$('mcpStartupError').textContent='';
  }catch(e){
    state.autoKnown=false;
    if($('mcpStartupError'))$('mcpStartupError').textContent=t('无法读取系统自启动设置，请检查系统权限或登录会话。');
  }
  show();
 }
   async function autoConnectChanged(){
   if(state.busy||state.pending)return;
   const c=controls(state.status,form());
   if(!c.canAutoStart){show();return;}
   const wanted=!!$('mcpAutoConnect').checked;
   state.busy=true;show();
   try {
    await sendMCP({action:wanted?'auto-connect-enable':'auto-connect-disable'});
    message(state.status.running?t('已保存。重启服务后，新的自动连接设置生效。'):t('Tunnel 自动连接设置已保存。'));
   }catch(e){message(errorMessage(e),true);}
   finally{state.busy=false;await refresh();}
  }
  async function tunnelChanged(){
   const s=state.status;
   if(state.busy||state.pending||!running(s))return;
   const connecting=Boolean(s.tunnel_connecting||s.ready);
   if(!connecting&&(!controls(s,form()).stored||controls(s,form()).changed||s.restart_required))return;
   state.busy=true;show();
   try {
    await sendMCP({action:connecting?'disconnect':'connect'});
    message(connecting?t('Tunnel 已断开，本地服务仍运行。'):t('Tunnel 连接请求已提交，正在检查状态。'));
   }catch(e){message(errorMessage(e),true);}
   finally{state.busy=false;await refresh();}
  }
async function autoChanged(){
  if(state.busy||state.pending)return;
  const c=controls(state.status,form());
  if(!c.canAutoStart){show();return;}
  const desired=$('mcpAutoStart').checked;
  state.busy=true;show();
  try{
   const r=await sendMCP({action:desired?'auto-enable':'auto-disable'});
   state.autoStart=!!r.auto_start;
   message(t('开机自启设置已更新，不会影响当前服务。'));
  }catch(e){message(errorMessage(e),true);}
  finally{state.busy=false;show();}
 }
 function init(){
  const panel=$('mcpConnectionSettings');
  if(state.panel===panel)return;
  state.stopPoll?.();state.panel=panel;state.initialized=true;
  const advanced=panel.querySelector?.('details');if(advanced)advanced.open=!!state.advanced;
  intendedInput($('mcpTunnelID'),state.editedID?state.draftID||'':state.status.tunnel_id||'');
  intendedInput($('mcpRuntimeKey'),state.editedKey?state.draftKey||'':'');
  $('mcpSave').addEventListener('click',()=>action('save'));
  $('mcpInstallClient')?.addEventListener('click',()=>action('install-client'));
  $('mcpConnect').addEventListener('click',()=>action(running(state.status)?'stop':'start'));
  $('mcpRestart').addEventListener('click',()=>action('restart'));
  $('mcpAutoStart')?.addEventListener('change',autoChanged);
  $('mcpAutoConnect')?.addEventListener('change',autoConnectChanged);
  $('mcpTunnelToggle')?.addEventListener('click',tunnelChanged);
  $('mcpKeyReveal')?.addEventListener('click',()=>{
   const input=$('mcpRuntimeKey'),reveal=$('mcpKeyReveal');
   if(!input.value || reveal.disabled)return;
   const showing=input.type==='password';
   input.type=showing?'text':'password';
   reveal.setAttribute('aria-pressed',String(showing));
   reveal.setAttribute('aria-label',showing?t('隐藏输入的密钥'):t('显示输入的密钥'));
   reveal.title=showing?t('隐藏输入的密钥'):t('显示输入的密钥');
  });
  $('mcpCopyConfig').addEventListener('click',async()=>{
   try{await navigator.clipboard.writeText(JSON.stringify(state.status.local_config||{},null,2));message(t('本地配置已复制。'));}
   catch(e){message(errorMessage(e),true);}
  });
  const poll=root.setInterval(()=>{
   if(mcpActive())refresh();
  },4000);
  state.stopPoll=()=>root.clearInterval(poll);
  if(offline()){message('本地服务未连接，MCP 设置暂不可用。',true);panel.querySelectorAll('button,input').forEach(el=>el.disabled=true);return;}
  refresh();autoStatus();
 }
 root.WardrobeMCP={load:init,init,refresh,controls,validID,
  capture(){if(!state.panel?.isConnected)return;state.advanced=!!state.panel.querySelector?.('details')?.open;state.draftID=$('mcpTunnelID').value;state.draftKey=$('mcpRuntimeKey').value;},
  unmount(){state.stopPoll?.();state.panel=null;}
 };
})(globalThis);
