'use strict';
// Inspired by BlogCTL's locale architecture: common host-independent DOM
// translation, a small bundled catalog, and one SQLite preference shared by
// the full Web Console and the browser extension side panel. User content is
// never sent to a translation service.
(function(root){
 const en={
 '语言':'Language','自动':'Auto','中文':'中文','界面语言':'Interface language',
 '统一管理京东和淘宝的购物记录，筛选穿搭商品。':'Manage JD and Taobao orders and identify wardrobe purchases.',
 '订单概览':'Order overview',
 '从已登录的京东和淘宝订单页同步商品记录，本机去重并识别穿搭候选。':'Sync JD and Taobao purchases from signed-in order pages. Deduplicate locally and identify wardrobe candidates.',
 '同步订单':'Sync orders','从购物网站同步':'Sync from shopping sites',
 '使用 Edge 当前登录状态，在后台访问对应订单列表并同步至本机 SQLite；不读取 Cookie，不需要导出文件。':'Use your existing Edge login to read shopping orders in a background tab and save to local SQLite. No Cookie access or exported files.',
 '订单中心 · 支持增量去重':'Order Center · Incremental sync',
 '已买到的宝贝 · 支持增量去重':'Purchase history · Incremental sync',
 '同步京东':'Sync JD','同步淘宝':'Sync Taobao',
 '首次同步会在后台打开相应平台的订单列表；如果网站要求登录，可手动打开页面完成登录后重新同步。当前默认列表可同步，其他年份或分页需核对覆盖范围。':'Sync opens an inactive order-list tab. If sign-in is required, log in to the site and retry. Verify other years and any pages that could not be reached.',
 '未开始同步':'Not synced yet','停止同步':'Stop sync',
 '整理穿搭入衣橱':'Organize wearables',
 '补齐商品图':'Cache product photos',
 '继续补图':'Continue caching',
 '衣橱整理':'Wardrobe organization',
 '从订单识别穿搭候选；整理单品与下载图片是两项独立操作。':'Identify wardrobe candidates from orders; organizing items and caching photos are separate actions.',
 '「整理穿搭」只建立待确认单品；「补齐商品图」才会下载图片，下载失败不会覆盖现有照片。':'Organize creates pending items only; Cache Product Photos downloads images separately and never overwrites existing photos.',
 '自动识别穿搭商品并建立待确认单品':'Classify purchases as pending wardrobe items',
 '下载并归档订单商品图片到本机':'Download verified product photos locally',
 '图片待归档':'Image to archive','暂无商品图':'No product photo',

 '未收到 Wardrobe 浏览器扩展响应。请确认扩展已启用，并刷新当前网页。':'Wardrobe extension did not respond. Check that it is enabled and reload this page.',

 '购买记录自动分类，确认实际持有后再加入衣橱。':'Automatically classify purchases; confirm ownership before adding items.',
 '导入订单文件':'Import order file',
 '支持京东、淘宝导出的 XLSX / CSV / JSON 文件，自动去重。':'Supports JD/Taobao XLSX, CSV and JSON exports with local deduplication.',
 '来源平台':'Source platform','本地订单文件':'Local order file',
 '选择文件':'Choose file','未选择文件':'No file selected','开始导入':'Import file','关闭导入':'Close import',
 '文件仅发送至本机 Wardrobe 服务。退款不等于退货，需人工确认实际持有。':'Files stay on your local Wardrobe server. A refund is not a return; confirm ownership manually.',
 '如何在 Edge 安装 Wardrobe 扩展':'How to install Wardrobe in Edge',
 '浏览器订单采集':'Browser order capture','开启采集':'Start capture','停止采集':'Stop capture',
 '请选择订单文件':'Choose an order file','单文件不能超过 12MB':'File size must not exceed 12 MB',

 '工作台':'Workspace','本地已连接':'Local connected','个人衣橱 · 穿搭管理工作台':'Personal wardrobe · Outfit workspace','刷新工作台':'Refresh workspace','新建搭配':'New Outfit',
 '我的衣橱':'My Wardrobe','本地版':'Local','订单库':'Orders','衣橱':'Wardrobe','发型':'Hairstyles','身体数据':'Body Profile','搭配':'Outfits','Skills':'Skills','导出备份':'Export Backup',
 '从购买记录，整理真正拥有的穿搭。':'From purchases to the clothes you actually own.',
 '正在读取本地订单…':'Loading local orders…','从已有的衣服，找到今天的搭配。':'Build today’s look from what you own.',
 '从购买记录，整理真正拥有的穿搭。':'Turn purchases into a real wardrobe.',
 '京东、淘宝订单统一列表 · 本地去重 · 自动分类 · 人工确认后入库':'JD and Taobao orders · Local deduplication · Automatic categories · Confirm before adding to wardrobe',
 '京东 · 淘宝统一订单':'JD · Taobao Orders',
 '所有订单与穿搭候选在本机 SQLite 保存，退款不等于退货。仅导入订单信息，不上传 Cookie、收货地址或浏览器令牌。':'Orders and outfit candidates stay in local SQLite. A refund does not imply a return. Cookies, shipping addresses and browser tokens are never imported.',
 '平台':'Platform','淘宝':'Taobao','京东':'JD','选择本地导出文件':'Select a local export','导入订单':'Import Orders','如何安装 Edge 扩展':'How to install the Edge extension',
 '复制浏览器扩展配对码':'Copy extension pairing key','开启订单页采集':'Start order capture','停止采集':'Stop capture',
 '已导入商品行':'Imported items','穿搭相关':'Wearable items','待入库穿搭':'Candidates to add','已关联衣橱':'Linked to wardrobe',
 '全部平台':'All platforms','仅穿搭相关':'Wearables only','全部订单商品':'All order items','待入库穿搭商品':'Unlinked wearable items','全部穿搭分类':'All categories',
 '全部分类':'All categories','全部评分':'All ratings','五星':'5 stars','四星及以上':'4+ stars','三星及以上':'3+ stars','仅本人新评分':'My ratings only',
 '全部持有状态':'All ownership states','全部合身状态':'All fit states','＋ 新建搭配':'+ New Outfit',
 '订单号':'Order ID','商品金额':'Item amount','实付':'Paid','退款':'Refund','退货':'Returned','交易状态待核对':'Transaction status not verified',
 '待确认':'Unconfirmed','本人仍持有':'Still owned','已退货':'Returned','未拥有':'Not owned','加入衣橱':'Add to wardrobe','已关联衣橱':'In wardrobe',
 '打开商品':'View product','查看商品图':'View image','无商品链接':'No product URL','非穿搭／待识别':'Not wearable / Unclassified',
 'T恤与POLO':'T-shirts & Polo','衬衫':'Shirts','针织衫':'Knitwear','卫衣':'Sweatshirts','外套':'Outerwear','裤装':'Pants','鞋靴':'Footwear','袜子':'Socks','家居与保暖内搭':'Home & base layers','配饰':'Accessories',
 '暂无本地图片':'No local image','图片未归档':'Image unavailable','选择单品':'Select Item','清空此部位':'Clear slot','没有匹配单品，可切换全部分类。':'No matching items. Try all categories.',
 '编辑搭配':'Edit Outfit','删除搭配':'Delete Outfit','生成搭配效果卡':'Generate lookbook card','确认本人正脸':'Confirm front-facing photo','确认本人全身':'Confirm full-body photo','试穿前检查':'Try-on preflight',
 '商品详情':'Item Details','关闭':'Close','保存搭配':'Save Outfit','删除':'Delete','取消':'Cancel','评分':'Rating','查看商品':'View Item',
 '暂无本地图片':'No local photo','正在读取本地衣橱…':'Loading wardrobe…','暂无':'None',
 '发型也是整体搭配的一部分。':'Your hairstyle completes the look.',
 '把喜欢的单品，穿成自己的风格。':'Turn individual pieces into your own style.',
 '编辑 AI 的衣橱工作流。':'Edit wardrobe AI workflows.',
 'Skill 说明来自仓库模板；你的修改保存在本机 SQLite，并供 MCP 实时读取。':'The workflow template comes from the repository. Your edits are saved in local SQLite and available through MCP.',
 '已安装的工作流':'Installed Workflows','内置模板与本地编辑版本':'Built-in templates and local edits','选择 Skill':'Select a workflow','恢复默认':'Reset','保存修改':'Save changes','编辑':'Edit','查看文本':'View text',
 'Skill Markdown 内容':'Workflow Markdown','当前使用内置模板，可直接编辑后保存为本地版本。':'Using the bundled template; edit to create a local version.',
 '商品导入':'Product Import','穿搭管理':'Outfit Management','身体数据与发型':'Body Profile & Hair','个人审美':'Style Preferences',
 '浏览器扩展需要单独安装':'Install the browser extension separately',
 'Wardrobe 完整应用扩展随安装包一起发布，但复制到磁盘不等于已经安装到 Edge。':'Extension files ship with Wardrobe, but copying files does not install the extension in Edge.',
 '选择本机 Wardrobe 安装位置的':'Select the','文件夹，安装后再点击「复制浏览器扩展配对码」并在 Wardrobe 侧边栏中粘贴。':'folder inside your Wardrobe installation, then copy and enter the local pairing key.',
 '无需重启 Edge、无需读取 Cookie；扩展属于实验性采集功能，导出文件导入仍可正常使用。':'No Edge restart or Cookie access required. Browser capture is experimental; local file import remains available.',
 '连接本地 Wardrobe':'Connect to local Wardrobe',
 '此侧边栏与本地 Web UI 共用数据；首次使用请在本地 Wardrobe 的「订单库」复制配对码。配对码只保存在 Edge 扩展私有存储中。':'This side panel uses the same database as the Web UI. Copy the pairing key from the local Orders page. The key stays in private extension storage.',
 '本地配对码':'Local pairing key','连接本地应用':'Connect local app','打开 Web UI':'Open Web UI','打开网页':'Open Web UI','在新标签页打开 Wardrobe':'Open Wardrobe in a new tab',
 '订单采集':'Order capture','商家':'Shop','商品名称':'Product name','购物平台':'Shopping platform','规格':'Specifications',
 };
 const stored=new WeakMap();const attribs=new WeakMap();const attrs=['placeholder','title','aria-label'];
 const skipSelector='script,style,textarea,pre,code,[data-i18n-ignore],.orderInfo,.cardTitle,.lookCard h3,.detailsTable td,.detailsTable th,.skillPreview,.skillEditorLabel';
 const preferenceValues=new Set(['auto','zh-CN','en']);
 let preference='auto',effective='zh-CN',observer=null,translating=false;
 function systemLocale(){return String(navigator.languages?.[0]||navigator.language||'zh-CN').toLowerCase().startsWith('en')?'en':'zh-CN'}
 function trans(s){
  if(effective!=='en'||typeof s!=='string')return s;
  const m=/^(\s*)([\s\S]*?)(\s*)$/.exec(s);if(!m)return s;
  const value=m[2];if(Object.hasOwn(en,value))return m[1]+en[value]+m[3];
  const pieces=[[/^(\d+) 件商品$/,'$1 items'],[/^(\d+) 件有本地图片$/,'$1 with local images'],[/^(\d+) 套搭配$/,'$1 outfits'],[/^(\d+) 件$/,'$1 pcs'],[/^订单 (.+)$/,'Order $1'],[/^退款：(.*)$/,'Refund: $1'],[/^退货：(.*)$/,'Return: $1'],[/^商品金额 (.*)$/,'Amount $1'],[/^实付 (.*)$/,'Paid $1'],[/^正在本地导入并去重…$/,'Importing and deduplicating locally…']];
  for(const [p,replacement] of pieces)if(p.test(value))return m[1]+value.replace(p,replacement)+m[3];
  return s;
 }
 function skip(el){return !!el?.closest?.(skipSelector)}
 function translateNode(node){if(!node||node.nodeType!==3||skip(node.parentElement))return;
  const current=node.nodeValue||'';const prev=stored.get(node);const source=prev&&current===prev.translated?prev.source:current;
  const translated=trans(source);stored.set(node,{source,translated});if(current!==translated)node.nodeValue=translated;
 }
 function translateAttrs(el){if(skip(el))return;let values=attribs.get(el);if(!values){values=new Map();attribs.set(el,values)}
  for(const attr of attrs){if(!el.hasAttribute(attr))continue;const current=el.getAttribute(attr);const old=values.get(attr);const source=old&&old.translated===current?old.source:current;const translated=trans(source);values.set(attr,{source,translated});if(current!==translated)el.setAttribute(attr,translated)}
 }
 function apply(rootNode){if(!rootNode)return;if(rootNode.nodeType===3){translateNode(rootNode);return}if(rootNode.nodeType!==1||skip(rootNode))return;translateAttrs(rootNode);const it=document.createTreeWalker(rootNode,NodeFilter.SHOW_TEXT);while(it.nextNode())translateNode(it.currentNode);for(const el of rootNode.querySelectorAll('*'))translateAttrs(el)}
 function change(pref){preference=pref;effective=pref==='auto'?systemLocale():pref;document.documentElement.lang=effective;translating=true;apply(document.body);translating=false;const sel=document.getElementById('wardrobeUiLocale');if(sel)sel.value=preference}
 async function reload(){try{const r=await root.WardrobeTransport.request('/api/ui-locale');if(!r.ok)return;const d=await r.json();if(preferenceValues.has(d.uiLocale))change(d.uiLocale)}catch{/* not connected yet */}}
 async function setLocale(pref){if(!preferenceValues.has(pref))throw Error('Invalid locale');const prior=preference;change(pref);try{const r=await root.WardrobeTransport.request('/api/ui-locale',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({uiLocale:pref})});if(!r.ok)throw Error('Cannot save locale');}catch(e){change(prior);throw e}}
 function init(){if(observer)return;const sel=document.getElementById('wardrobeUiLocale');if(!sel)return;sel.addEventListener('change',()=>setLocale(sel.value).catch(()=>{const status=document.getElementById('extensionConnectionStatus');if(status)status.textContent='无法保存界面语言 / Unable to save language'}));change('auto');observer=new MutationObserver(records=>{if(translating)return;translating=true;for(const m of records){if(m.type==='characterData')translateNode(m.target);else if(m.type==='attributes')translateAttrs(m.target);else for(const n of m.addedNodes)apply(n)}translating=false});observer.observe(document.body,{subtree:true,childList:true,characterData:true,attributes:true,attributeFilter:attrs});reload();}
 root.WardrobeI18n={get preference(){return preference},get locale(){return effective},translate:trans,setLocale,reload};
 document.addEventListener('DOMContentLoaded',init);
 root.addEventListener('wardrobe:paired',reload);
 root.addEventListener('focus',reload);
 document.addEventListener('visibilitychange',()=>{if(!document.hidden)reload()});
})(globalThis);
