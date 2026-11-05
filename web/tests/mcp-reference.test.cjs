'use strict';
const test=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const html=fs.readFileSync('web/index.html','utf8');
const css=fs.readFileSync('web/style.css','utf8');
const js=fs.readFileSync('web/mcp-connection.js','utf8');
const locale=fs.readFileSync('web/locale.js','utf8');

test('MCP layout mirrors the reviewed Reverfolio workflow',()=>{
  const panel=html.slice(html.indexOf('<section id="mcpPanel"'),html.indexOf('<section id="skillsPanel"'));
  assert.match(panel,/id="mcpConnectionSettings"/);
  assert.match(panel,/class="mcp-workflow-header"/);
  assert.match(panel,/class="mcp-status-chip"/);
  assert.match(panel,/mcp-step"[^>]*>01<\/span>/);
  assert.match(panel,/mcp-step"[^>]*>02<\/span>/);
  assert.ok(panel.indexOf('id="mcpTunnelID"')<panel.indexOf('id="mcpRuntimeKey"'));
  assert.ok(panel.indexOf('id="mcpRuntimeKey"')<panel.indexOf('id="mcpAutoStart"'));
  assert.match(panel,/id="mcpKeyReveal"/);
  assert.match(panel,/id="mcpTunnelToggle"/);
  assert.match(panel,/class="mcp-setup-advanced"/);
  assert.doesNotMatch(panel,/<details[^>]*\sopen(?:\s|=|>)/);
  assert.doesNotMatch(panel,/id="mcpStop"|id="mcpDisconnect"|class="mcpFields"/);
  assert.match(css,/#mcpConnectionSettings \.mcp-flow-section/);
  assert.match(css,/#mcpConnectionSettings \.mcp-status-chip\[data-kind="ready"\]/);
});

test('MCP controller preserves native status and secret handling',()=>{
  assert.match(js,/const readMCP=async/);
  assert.match(js,/const sendMCP=async/);
  assert.match(js,/mcpRuntimeKey/);
  assert.match(js,/runtime_key_saved/);
  assert.match(js,/pending\.revision/);
  assert.match(js,/tunnel_connecting/);
  assert.match(js,/auto-connect-enable/);
  assert.match(js,/auto-status/);
  assert.match(js,/root\.WardrobeMCP=/);
  assert.doesNotMatch(js,/root\.ReverfolioMCP/);
  assert.match(locale,/"本地 MCP 客户端（STDIO）"/);
  assert.match(locale,/"本地服务开机自启"/);
});

test('Wardrobe extension has the same native MCP panel and stylesheet',()=>{
  for(const name of ['index.html','mcp-connection.js','style.css','locale.js']){
    assert.equal(fs.readFileSync('web/'+name,'utf8'),fs.readFileSync('browser-extension/app/'+name,'utf8'));
  }
});

// On-demand client installation is independent of credentials and bundled installers.
test('Tunnel Client exposes an independent verified on-demand download',()=>{
  assert.match(html,/id="mcpInstallClient"/);
  assert.match(html,/id="mcpClientHint"/);
  assert.match(js,/if\(action==='install-client'\)/);
  assert.match(js,/sendMCP\(\{action:'install-client'\}\)/);
  assert.match(js,/mcpInstallClient'\)\.hidden=!!s\.client_installed/);
  assert.match(locale,/"安装官方 Tunnel Client"/);
});
