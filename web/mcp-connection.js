'use strict';
// The embedded Web UI and browser side panel share these exact assets.
// Status and actions are from the native Go MCP manager, not browser-local state.
(function () {
  const $ = id => document.getElementById(id);
  const stateNames = {
    unconfigured:'未配置 / Unconfigured',client_missing:'缺少 Tunnel Client / Client missing',
    stopped_or_unhealthy:'服务已停止 / Stopped',starting_or_unhealthy:'正在连接或健康检查失败 / Connecting or unhealthy',
    standby:'服务已启动，未连接 / Standby',ready:'Tunnel Ready（仍需在 ChatGPT 授权）',
  };
  let current = null, loading = false, polling = 0;
  const offline = window.WardrobeTransport?.host === 'extension';
  async function request(method, body) {
    const res = await WardrobeTransport.request('/api/mcp-connection', {
      method, ...(body ? {headers: {'Content-Type':'application/json'}, body:JSON.stringify(body)}:{})
    });
    if (!res.ok) {
      const msg = (await res.text()).slice(0,250);
      throw Error(msg || 'MCP 请求失败 / Request failed');
    }
    return res.json();
  }
  function message(s){$('mcpFeedback').textContent = s}
  function setBusy(b) {
    loading=b;
    $('mcpPanel').querySelectorAll('button, input').forEach(x=>x.disabled=b||offline);
  }
  function draw(s) {
    current=s;
    const ready=s.ready?'Ready':(stateNames[s.state]||s.state);
    $('mcpConnectionState').textContent=ready+(s.restart_required?' · 配置待重启 / Restart required':'');
    if (document.activeElement!==$('mcpTunnelID')) $('mcpTunnelID').value=s.tunnel_id||'';
    if (document.activeElement!==$('mcpKey')) $('mcpKey').value='';
    $('mcpKey').placeholder=s.runtime_key_saved?'已保存（留空沿用） / Saved (leave blank)':'Runtime API Key';
    $('mcpAutoConnect').checked=!!s.tunnel_auto_connect;
    $('mcpLoginStartup').checked=!!s.auto_start;
    $('mcpSaveState').textContent=s.setup_state==='running'?'正在校验 / Validating':
       s.setup_state==='failed'?'保存失败，旧配置仍保留 / Save failed; old settings retained':
       s.setup_state==='succeeded'?'已保存 / Saved':'';
    $('mcpLocalSTDIO').textContent=JSON.stringify(s.local_config||{},null,2);
    $('mcpLocalSTDIO').hidden=false;
    $('mcpMissingClient').hidden=!!s.client_installed;
    const readyToRun=s.configured&&s.runtime_key_saved&&s.client_installed;
    for(const [id,can] of Object.entries({
      mcpStart: readyToRun&&!s.running,mcpStop: s.running,mcpRestart:readyToRun&&s.running,
      mcpConnect:readyToRun&&s.running&&!s.ready&&!s.restart_required,
      mcpDisconnect:s.running&&s.tunnel_connecting,
    }))$(id).disabled=loading||offline||!can;
  }
  async function load(){
    if(offline){message('本地服务未连接，MCP 可选配置暂不可用。');return}
    try{const s=await request('GET');draw(s)}catch(e){message('无法读取 MCP 状态：'+e.message)}
  }
  async function action(type,extra) {
    if(loading||offline)return;
    setBusy(true);message('正在执行 / Working…');
    try {
      const result=await request('POST',{action:type,...extra});
      message(type==='save'?'配置正在校验；当前连接不会改变。 / Validating; current connection unchanged.':
          '操作已提交 / Action accepted');
      if(result.auto_start!==undefined)$('mcpLoginStartup').checked=!!result.auto_start;
    } catch(e){message('操作失败 / Failed: '+e.message)}
    finally {setBusy(false);await load()}
  }
  $('mcpSave').onclick=()=> {
    const id=$('mcpTunnelID').value.trim();
    const key=$('mcpKey').value.trim();
    if(!id){message('请输入 Tunnel ID');return}
    if(!key&&!current?.runtime_key_saved){message('请输入 Runtime API Key');return}
    action('save',{tunnel_id:id,runtime_api_key:key,reuse_key:!key,
      replace:!!current?.configured,previous_tunnel_id:current?.tunnel_id||''})
      .finally(()=>$('mcpKey').value='');
  };
  for(const [id,cmd] of [['mcpStart','start'],['mcpStop','stop'],['mcpRestart','restart'],
    ['mcpConnect','connect'],['mcpDisconnect','disconnect'],['mcpInstallClient','install-client']])
    $(id).onclick=()=>action(cmd);
  $('mcpLoginStartup').onchange=e=>action(e.target.checked?'auto-enable':'auto-disable');
  $('mcpAutoConnect').onchange=e=>action(e.target.checked?'auto-connect-enable':'auto-connect-disable');
  $('mcpLocalSTDIO').onclick=async()=>{
    const text=$('mcpLocalSTDIO').textContent;
    try{await navigator.clipboard.writeText(text);message('STDIO 配置已复制')}catch{message('请手动复制 STDIO 配置')}
  };
  window.WardrobeMCP={load};
  // Refresh process and Tunnel health while on this tab; never store credentials in JS state.
  polling=window.setInterval(()=>{if(!$('mcpPanel').hidden&&!loading)load()},5000);
})();
