'use strict';
// Order rows are rendered using textContent. Browser extension captures
// normalized product fields from shopping pages; no file upload in this UI.
(()=>{
 const $o=s=>document.querySelector(s);
 let rows=[],loading=false;
 const labels={unknown:'待确认',owned:'本人仍持有',returned:'已退货',not_owned:'未拥有'};
 function status(s){$o('#ordersStatus').textContent=s||''}
 function count(label,value){const n=document.createElement('div');n.className='orderStat';n.append(el('strong','',String(value)),el('span','meta',label));return n}
 function text(tag,cls,value){return el(tag,cls,value||'')}
 function linkTo(url,label){const a=el('a','',label);try{const u=new URL(url);if(!['https:','http:'].includes(u.protocol))throw Error('invalid');a.href=u.href;a.target='_blank';a.rel='noopener noreferrer'}catch{return text('span','meta','无有效链接')};return a}
 const cats=['','T恤与POLO','衬衫','针织衫','卫衣','外套','裤装','鞋靴','袜子','家居与保暖内搭','配饰'];
 const options=(select,values,current)=>{select.replaceChildren();values.forEach(([v,label])=>{const o=el('option','',label);o.value=v;select.append(o)});select.value=current};
 const date=x=>x.ordered_at?.slice(0,19).replace('T',' ')||'日期未知';
 function render(){
  const q=$o('#ordersSearch').value.trim().toLowerCase();const platform=$o('#ordersPlatform').value,cat=$o('#ordersCategory').value,scope=$o('#ordersScope').value;
  const list=rows.filter(x=>(!platform||x.platform===platform)&&(!cat||x.category===cat)&&(!q||[x.name,x.spec,x.order_id,x.shop].join(' ').toLowerCase().includes(q))&&(scope==='all'||(x.wearable&&(scope!=='unlinked'||!x.product_id))));
  const stats=$o('#ordersStats');stats.replaceChildren(count('已导入商品行',rows.length),count('穿搭相关',rows.filter(x=>x.wearable).length),count('待入库穿搭',rows.filter(x=>x.wearable&&!x.product_id).length),count('已关联衣橱',rows.filter(x=>x.product_id).length));
  const candidateCount=rows.filter(x=>x.wearable&&!['returned','not_owned'].includes(x.owned)).length;
  const localImageCount=rows.filter(x=>x.wearable&&x.local_image).length;
  // The inventory toolbar owns the organization/image archive status.
  // Rendering an order list must never clear an active inventory operation.
  const wrap=$o('#ordersList');wrap.replaceChildren();if(!list.length){wrap.append(el('div','empty','当前筛选没有订单商品。点击「同步订单」从京东或淘宝读取，或调整筛选条件。'));return}
  const fragment=document.createDocumentFragment();
  for(const x of list.slice(0,2000)){
   const card=el('article','orderRow');const body=el('div','orderInfo');
   const thumb=x.local_image?image(x.local_image,x.name):el('div','imageBox orderMissingImage',x.image_url?'图片待归档':'暂无商品图');
   thumb.classList.add('orderThumbnail');
   const meta=el('div','orderRowMeta');meta.append(text('span','tag',x.platform),text('span','meta',date(x)),text('span','meta','订单 '+x.order_id),text('span','meta',x.shop));
   const title=el('h4','',x.name);const detail=el('p','meta',[x.spec,x.quantity+' 件',x.amount?'商品金额 '+x.amount:'',x.paid?'实付 '+x.paid:''].filter(Boolean).join(' · '));
   const statusLine=el('p','meta',[x.order_status,x.refund_status?'退款：'+x.refund_status:'',x.return_status?'退货：'+x.return_status:''].filter(Boolean).join(' · ')||'交易状态待核对');
   body.append(meta,title,detail,statusLine);
   const actions=el('div','orderRowActions');
   const category=el('select');category.setAttribute('aria-label','穿搭分类');options(category,cats.map(v=>[v,v||'非穿搭／待识别']),x.category||'');
   category.onchange=async()=>{category.disabled=true;try{await write('orders/decision','POST',{source_key:x.source_key,category:category.value});await load()}catch(e){status('分类更新失败：'+e.message);category.value=x.category}finally{category.disabled=false}};
   const owned=el('select');owned.setAttribute('aria-label','实际持有状态');options(owned,Object.entries(labels),x.owned||'unknown');
   owned.onchange=async()=>{owned.disabled=true;try{await write('orders/decision','POST',{source_key:x.source_key,owned:owned.value});await load()}catch(e){status('持有状态更新失败：'+e.message);owned.value=x.owned}finally{owned.disabled=false}};
   const promote=el('button','',x.product_id?'已关联衣橱':'加入衣橱');promote.disabled=!!x.product_id||!x.wearable||['returned','not_owned'].includes(x.owned);
   promote.onclick=async()=>{if(!window.confirm('将此商品加入衣橱？未确认实际持有的商品会标记为“待确认”，不会直接视为已拥有。'))return;promote.disabled=true;try{await write('orders/promote','POST',{source_key:x.source_key});await load();products=await api('products');indexProducts();toast('商品已加入衣橱')}catch(e){status('入库失败：'+e.message);promote.disabled=false}};
   actions.append(category,owned,promote,x.url?linkTo(x.url,'打开商品'):text('span','meta','无商品链接'));if(x.image_url)actions.append(linkTo(x.image_url,'查看商品图'));
   card.append(thumb,body,actions);fragment.append(card);
  }
  wrap.append(fragment);
  status('当前显示 '+Math.min(list.length,2000)+' / '+list.length+' 条；仅本地存储，重复同步会按订单商品键去重。');
 }
 async function load(){if(loading)return;loading=true;try{rows=await api('orders');render()}catch(e){status('加载订单失败：'+e.message)}finally{loading=false}}
 let organizing=false,caching=false,organizeQueued=false;
 async function organize({auto=false}={}){
  if(organizing){organizeQueued=true;return}
  organizing=true;const btn=$o('#ordersOrganize');btn.disabled=true;
  let created=0,remaining=0,errors=0,previousRemaining=Infinity;
  try{
   for(let round=0;round<30;round++){
    const result=await write('orders/organize','POST',{limit:100});
    created+=result.created;errors+=result.errors;
    remaining=result.remaining;
    $o('#ordersCatalogStatus').textContent='整理穿搭中：新增 '+created+' 件单品，未整理 '+remaining+' 条（所有新单品默认待确认持有）';
    if(remaining===0||result.errors>=100||remaining>=previousRemaining)break;
    previousRemaining=remaining;
   }
   await load();
   products=await api('products');indexProducts();window.refreshWardrobeView?.();
   $o('#ordersCatalogStatus').textContent='整理完成：新增 '+created+' 件穿搭单品；未处理 '+remaining+' 条、错误 '+errors+' 条。未经确认的购买不会标记为本人持有。';
   if(!auto&&created>0)toast('已整理 '+created+' 件穿搭候选，待确认实际持有');
  }catch(e){$o('#ordersCatalogStatus').textContent='整理失败：'+e.message}
  finally{
   organizing=false;btn.disabled=false;
   if(organizeQueued){organizeQueued=false;setTimeout(()=>organize({auto:true}),0)}
  }
 }
 let nextImageCursor='';
 async function cacheImages({auto=false}={}){
  if(caching)return;
  caching=true;const btn=$o('#ordersCacheImages');btn.disabled=true;
  let cursor=nextImageCursor,cached=0,failed=0,missing=0,attempted=0;
  let httpFailed=0,networkFailed=0,contentFailed=0,saveFailed=0;
  let hasMore=false;
  const label=btn.textContent;
  try{
   // CDN images may be slow, unreachable, or removed. Keep each request to
   // one item so a bad CDN never blocks several unrelated items in one HTTP call.
   // Continue with another click; the cursor survives between clicks, but no
   // private order record or source URL is written to extension storage.
   for(let round=0;round<10;round++){
    let result;
    for(let attempt=0;attempt<2;attempt++){
     try{result=await write('orders/cache-images','POST',{after:cursor,limit:1});break}
     catch(e){if(attempt===1)throw e;await new Promise(r=>setTimeout(r,350))}
    }
    if(!result.attempted){hasMore=false;break}
    attempted+=result.attempted;cached+=result.cached;failed+=result.failed;missing+=result.missing_source;
    httpFailed+=result.http_failures||0;networkFailed+=result.network_failures||0;
    contentFailed+=result.content_failures||0;saveFailed+=result.save_failures||0;
    cursor=result.next;nextImageCursor=cursor;hasMore=result.has_more;
    $o('#ordersCatalogStatus').textContent='补图进行中：已归档 '+cached+' 张，未成功 '+failed+' 张（图片源失效 '+httpFailed+'，网络或超时 '+networkFailed+'，格式 '+contentFailed+'，写入 '+saveFailed+'）。';
    if(!hasMore)break;
   }
   if(attempted){await load();products=await api('products');indexProducts();window.refreshWardrobeView?.()}
   if(!hasMore){nextImageCursor='';btn.textContent=label==='继续补图'?'补齐商品图':label}
   else{btn.textContent='继续补图'}
   $o('#ordersCatalogStatus').textContent='本次归档 '+cached+' 张、下载失败 '+failed+' 张（源地址失效 '+httpFailed+'，网络或超时 '+networkFailed+'，格式 '+contentFailed+'，保存 '+saveFailed+'），无效来源 '+missing+' 张。'+(hasMore?'点击「继续补图」处理后续商品。':'已检查完当前可用来源；失败图片不会被替换。');
   if(!auto&&cached)toast('已保存 '+cached+' 张本地商品图');
  }catch(e){
   // A transient error should not lose the cursor or hide already completed
   // items. Retrying the button resumes from the last acknowledged image.
   btn.textContent='继续补图';
   $o('#ordersCatalogStatus').textContent='补图中断：已保存 '+cached+' 张、失败 '+failed+' 张；可点击「继续补图」重试。原因：'+e.message;
  }finally{caching=false;btn.disabled=false}
 }
 $o('#ordersOrganize').onclick=()=>organize();
 $o('#ordersCacheImages').onclick=()=>cacheImages();

 const syncPanel=$o('#ordersSyncPanel');
 const syncToggle=$o('#ordersSyncToggle');
 const syncStatus=$o('#ordersSyncStatus');
 const awaitingExtension=new Map();
 const setSyncOpen=open=>{syncPanel.hidden=!open;syncToggle.setAttribute('aria-expanded',String(open))};
 syncToggle.onclick=()=>setSyncOpen(syncPanel.hidden);
  api('capabilities').then(c=>{syncToggle.hidden=!c.retailerSync;$o('#ordersCacheImages').hidden=!c.remoteOrderImages;if(!c.retailerSync)setSyncOpen(false)}).catch(()=>{syncToggle.hidden=true;$o('#ordersCacheImages').hidden=true;});
 $o('#ordersSyncClose').onclick=()=>setSyncOpen(false);
 function sync(platform){
  const button=$o(platform==='京东'?'#ordersSyncJD':'#ordersSyncTB');button.disabled=true;
  syncStatus.textContent='正在连接 '+platform+' 订单中心…';
  window.dispatchEvent(new CustomEvent('wardrobe:sync-order',{detail:{platform}}));
  // A pre-existing tab may not yet have the newly installed extension bridge.
  clearTimeout(awaitingExtension.get(platform));
  awaitingExtension.set(platform,setTimeout(()=>{
   if(!awaitingExtension.has(platform))return;awaitingExtension.delete(platform);
   button.disabled=false;syncStatus.textContent='未收到 Wardrobe 浏览器扩展响应。请确认扩展已启用，并刷新当前网页。';
  },7000));
 }
 $o('#ordersSyncJD').onclick=()=>sync('京东');
 $o('#ordersSyncTB').onclick=()=>sync('淘宝');
 $o('#ordersSyncStop').onclick=()=>{syncStatus.textContent='正在停止同步…';window.dispatchEvent(new CustomEvent('wardrobe:stop-order-sync'))};
 window.addEventListener('wardrobe:sync-status',event=>{
  const {platform,message}=event.detail||{};if(!message)return;
  if(platform&&awaitingExtension.has(platform)){clearTimeout(awaitingExtension.get(platform));awaitingExtension.delete(platform)}
  syncStatus.textContent=(platform?platform+'：':'')+message;
  if(platform)$o(platform==='京东'?'#ordersSyncJD':'#ordersSyncTB').disabled=false;
 });
 for(const id of ['ordersPlatform','ordersScope','ordersCategory'])$o('#'+id).addEventListener('change',render);
 let timer; $o('#ordersSearch').addEventListener('input',()=>{clearTimeout(timer);timer=setTimeout(render,100)});
 options($o('#ordersCategory'),cats.map(v=>[v,v||'全部穿搭分类']),'');
 window.orderUI={load,render,organize,cacheImages};
})();
