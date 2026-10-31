'use strict';
const $=s=>document.querySelector(s);
// Legacy inner/accessory1/accessory2 keys remain readable in old looks.
const garmentSlots={outer:'外套',mid:'中层 · 衬衫/毛衣',inner:'贴身内搭 · T恤/长袖',pants:'裤装',socks:'袜子',shoes:'鞋子'};
const accessorySlots={glasses:'眼镜',hat:'帽子',gloves:'手套',necklace:'项链',watch:'手表',belt:'腰带',scarf:'围巾',bag:'包',tie:'领带',accessory1:'其他配饰①',accessory2:'其他配饰②'};
const slots={hair:'发型',...garmentSlots,...accessorySlots,other:'其他单品'};
const recommendations={outer:['外套'],mid:['衬衫','针织衫','卫衣'],inner:['T恤与POLO','家居与保暖内搭','衬衫'],pants:['裤装'],socks:['袜子'],shoes:['鞋靴'],accessory1:['配饰'],accessory2:['配饰']};
const accessoryKeywords={glasses:['眼镜','镜框','墨镜','太阳镜'],hat:['帽','贝雷'],gloves:['手套','护掌'],necklace:['项链','吊坠','毛衣链'],watch:['手表','腕表'],belt:['腰带','皮带'],scarf:['围巾','丝巾','领巾'],bag:['背包','斜挎包','手提包','单肩包','挎包','胸包','腰包'],tie:['领带','领结']};
function isRecommended(p,key){if(accessoryKeywords[key])return accessoryKeywords[key].some(word=>p.name.includes(word));return (recommendations[key]||[]).includes(p.category)}
let products=[],looks=[],hairStyles=[],bodyProfile=null,byID={},hairByID={},tab='orders',draft=null,picking='',toastTimer;
const skillTitles={'wardrobe-import':'商品导入','wardrobe-outfits':'穿搭管理','wardrobe-profile':'身体数据与发型','wardrobe-aesthetics':'个人审美'};
let currentSkill=null,skillItems=[],skillBusy=false,skillRequest=0;
function el(tag,cls,text){const n=document.createElement(tag);if(cls)n.className=cls;if(text!==undefined)n.textContent=text;return n}
function toast(message){$('#toast').textContent=message;$('#toast').style.display='block';clearTimeout(toastTimer);toastTimer=setTimeout(()=>$('#toast').style.display='none',3500)}
async function api(path,options){const r=await WardrobeTransport.request('/api/'+path,options);if(!r.ok){let text=await r.text();try{text=JSON.parse(text).error||text}catch{}throw Error(r.status===409?'数据已被其他页面更新，请关闭后重新打开再保存。':text)}return r.json()}
function write(path,method,body){return api(path,{method,headers:{'Content-Type':'application/json'},body:JSON.stringify(body)})}
function image(src,name){const box=el('div','imageBox');if(!src){box.append(el('span','missing','暂无本地图片'));return box}const img=el('img');const source=src;Promise.resolve(WardrobeTransport.media(source)).then(value=>{img.src=value}).catch(()=>{box.replaceChildren(el('span','missing','图片未归档'))});img.alt=name||'商品图片';img.loading='lazy';img.decoding='async';img.onerror=()=>{box.replaceChildren(el('span','missing','图片未归档'))};box.append(img);return box}
function zoom(src){if(!src)return;WardrobeTransport.media(src).then(value=>{$('#largeImage').src=value;$('#lightbox').showModal()}).catch(()=>toast('图片暂不可用'))}
function link(url,text){const a=el('a','',text);try{const u=new URL(url);if(!['https:','http:'].includes(u.protocol))return el('span','meta',text+'（无链接）');a.href=u.href;a.target='_blank';a.rel='noopener noreferrer';a.onclick=e=>e.stopPropagation()}catch{return el('span','meta',text+'（无链接）')}return a}
function productCard(p,choose){const c=el('article','card');c.tabIndex=0;c.setAttribute('role','button');c.setAttribute('aria-label',p.id+' '+p.name);const go=()=>choose?choose(p):detail(p);c.onclick=go;c.onkeydown=e=>{if(e.target!==c)return;if(e.key==='Enter'||e.key===' '){e.preventDefault();go()}};c.append(image(p.image,p.name));const b=el('div','cardBody');b.append(el('span','tag',p.category),el('span','meta','  '+p.id),el('div','cardTitle',p.name),el('div','meta',[p.color,p.size].filter(Boolean).join(' · ')),el('div','stars','★'.repeat(p.rating||0)+' '+(p.ratingSource==='user'?'本人评分':'原表评分')),link(p.url,'查看商品 ↗'));c.append(b);return c}
function selectOptions(selector,values,initial){const s=$(selector);s.replaceChildren();for(const [value,label] of values){const o=el('option','',label);o.value=value;s.append(o)}if(initial!==undefined)s.value=initial;return s}
function render(){const content=$('#content');content.replaceChildren();$('#filters').hidden=tab!=='wardrobe';$('#wardrobeCatalogTools').hidden=tab!=='wardrobe';$('#skillsPanel').hidden=tab!=='skills';$('#ordersPanel').hidden=tab!=='orders';content.hidden=tab==='skills'||tab==='orders';$('#newLook').hidden=tab==='skills'||tab==='orders';content.className='grid'+(tab==='looks'?' lookGrid':tab==='body'?' notesGrid':'');for(const [id,t]of [['ordersTab','orders'],['wardrobeTab','wardrobe'],['hairTab','hair'],['bodyTab','body'],['looksTab','looks'],['skillsTab','skills']])$('#'+id).classList.toggle('active',tab===t);
if(tab==='orders'){$('#title').textContent='订单库';$('#summary').textContent='管理已导入的本地订单记录，筛选穿搭商品。';return}
if(tab==='skills'){$('#title').textContent='编辑 AI 的衣橱工作流。';$('#summary').textContent='Skill 说明来自仓库模板；你的修改保存在本机 SQLite，并供 MCP 实时读取。';return}

if(tab==='wardrobe'){const q=$('#search').value.trim().toLowerCase(),cat=$('#category').value,min=Number($('#ratingFilter').value),src=$('#sourceFilter').value,owned=$('#ownedFilter').value,fit=$('#fitFilter').value;const filtered=products.filter(p=>(!q||p.search.includes(q))&&(!cat||p.category===cat)&&(!min||p.rating>=min)&&(!src||p.ratingSource===src)&&(!owned||p.owned===owned)&&(!fit||p.fit===fit));$('#title').textContent='从已有的衣服，找到今天的搭配。';$('#summary').textContent=filtered.length+' / '+products.length+' 件商品 · '+products.filter(p=>p.image).length+' 件有本地图片 · 搜索和筛选无需联网';const fragment=document.createDocumentFragment();filtered.forEach(p=>fragment.append(productCard(p)));content.append(fragment);if(!filtered.length)content.append(el('div','empty','没有找到符合条件的单品。'))}
else if(tab==='looks'){$('#title').textContent='把喜欢的单品，穿成自己的风格。';$('#summary').textContent=looks.length+' 套搭配 · 原表未确认关联继续保留为待确认';looks.forEach(l=>content.append(lookCard(l)))}
else if(tab==='hair'){$('#title').textContent='发型也是整体搭配的一部分。';$('#summary').textContent=hairStyles.length+' 款本地发型参考 · 不等于本人明确偏好';hairStyles.forEach(h=>content.append(hairCard(h)))}
else if(tab==='body'){renderBody(content)}
}
// Inventory refresh hook used by the orders catalog workflow.
window.refreshWardrobeView=()=>{if(tab==='wardrobe')render()};
function lookCard(l){
  const c=el('article','lookCard');c.append(el('span','tag',l.id),el('h3','',l.name||l.scene||'未命名搭配'));
  if(l.photo){const img=el('img','photo');WardrobeTransport.media(l.photo).then(value=>img.src=value).catch(()=>{});img.loading='lazy';img.alt=l.scene||'实穿照片';img.onclick=()=>zoom(l.photoOriginal||l.photo);c.append(img)}
  const selected=Object.entries(slots).filter(([key])=>key==='hair'?hairByID[l.slots?.[key]]:byID[l.slots?.[key]]);
  const row=el('div','lookImages');for(const [key,label] of selected){const p=key==='hair'?hairByID[l.slots[key]]:byID[l.slots[key]],cell=el('div');cell.append(image(p.image,p.name),el('div','slotLabel',label+' · '+p.id));cell.style.cursor='pointer';cell.onclick=()=>key==='hair'?zoom(p.image):detail(p);row.append(cell)}
  if(!selected.length)row.append(el('p','meta','尚未关联确定的单品'));
  c.append(row);if(l.rating)c.append(el('div','stars','★'.repeat(l.rating)));if(l.comment)c.append(el('p','',l.comment));if(l.candidates)c.append(el('p','',l.candidates));c.append(el('div','meta',l.verification||'手动搭配'));
  const actions=el('div','lookActions'),b=el('button','','编辑搭配'),del=el('button','danger','删除');
  b.onclick=()=>editLook(l);del.onclick=()=>removeLook(l,del);actions.append(b,del);
  const lookbook=el('button','','生成搭配效果卡');lookbook.type='button';lookbook.onclick=()=>window.open((WardrobeTransport.host==='extension'?WardrobeTransport.base:'')+'/lookbook.html?lookId='+encodeURIComponent(l.id),'_blank','noopener');actions.append(lookbook);
  if(l.photo){
    const face=el('button','','确认本人正脸'),full=el('button','','确认本人全身');
    face.type='button';full.type='button';
    face.onclick=()=>confirmTryonPhoto(l,'face');full.onclick=()=>confirmTryonPhoto(l,'body');
    actions.append(face,full);
  }
  const preflight=el('button','','试穿前检查');preflight.type='button';
  preflight.onclick=()=>checkTryonLook(l);actions.append(preflight);
  c.append(actions);return c
}
async function confirmTryonPhoto(l,role){
  const description=role==='face'?'本人真实、清晰、无遮挡的正脸，且头部正对镜头':'本人真实的全身实拍，从头到脚完整可见';
  if(!window.confirm('你正在确认 '+l.id+' 的照片为'+description+'。\n请先查看原图。AI生成、商家模特、侧脸或被遮挡的照片不可确认。\n这只是本人声明，不是自动人脸核验。确定记录吗？'))return;
  try{
    await write('tryon/confirm-photo','POST',{lookId:l.id,role,confirmRealPhoto:true,confirmFrontal:role==='face',confirmFullBody:role==='body'});
    toast('已记录该照片的'+(role==='face'?'正脸':'全身')+'本人确认；更换图片将自动失效');
  }catch(e){toast('照片核验失败：'+e.message)}
}
async function checkTryonLook(l){
  const faceLookId=window.prompt('请输入已确认本人正脸的 Look ID（必须是本人真实正脸，不能使用发型示意图）：',l.photo?l.id:'');
  if(faceLookId===null)return;
  const bodyLookId=window.prompt('请输入已确认本人全身照的 Look ID：',l.photo?l.id:'');
  if(bodyLookId===null)return;
  try{
    const result=await write('tryon/preflight','POST',{lookId:l.id,faceLookId:faceLookId.trim(),bodyLookId:bodyLookId.trim()});
    const summary=result.ready?'照片/商品与配饰白名单预检查通过。\n目前尚未接入受控图像编辑器，因此不能直接生成本人试穿图。':'预检查未通过：\n'+result.blocks.map(s=>'• '+s).join('\n');
    window.alert(summary);
  }catch(e){toast('试穿预检查失败：'+e.message)}
}
function hairCard(h,choose){
  const c=el('article','card hairCard');c.tabIndex=0;c.setAttribute('role','button');
  c.setAttribute('aria-label',h.name);c.append(image(h.image,h.name));
  const b=el('div','cardBody');b.append(el('span','tag',h.category||'发型'),el('div','cardTitle',h.name),el('div','meta',h.id+' · 本地参考图'));
  c.append(b);c.onclick=()=>choose?choose(h):zoom(h.image);
  c.onkeydown=e=>{if(e.target===c&&(e.key==='Enter'||e.key===' ')){e.preventDefault();c.click()}};return c
}
function renderBody(content){
  $('#title').textContent='个人身体数据与服装尺寸参考。';
  if(!bodyProfile){$('#summary').textContent='尚未导入身体测量数据';return}
  $('#summary').textContent=(bodyProfile.dateNote||'历史记录')+' · 保存到本机 SQLite';
  const grouped={};for(const [key,m] of Object.entries(bodyProfile.measurements||{})){(grouped[m.group]??=[]).push([key,m])}
  for(const [group,values] of Object.entries(grouped)){
    const panel=el('section','bodyPanel');panel.append(el('h3','',group));const table=el('table','detailsTable bodyTable');
    for(const [key,m] of values){const tr=el('tr'),input=el('input');input.type='number';input.step='0.1';input.min='0.1';input.max='500';input.value=m.value;input.dataset.metric=key;input.setAttribute('aria-label',m.label+' (cm)');tr.append(el('th','',m.label),el('td','',m.method||''));const td=el('td');td.append(input,' cm');tr.append(td);table.append(tr)}
    panel.append(table);content.append(panel)
  }
  const panel=el('section','bodyPanel widePanel');panel.append(el('h3','', '合身成衣尺寸参考（与身体净围分开）'));
  const table=el('table','detailsTable');for(const v of Object.values(bodyProfile.garmentTargets||{})){const tr=el('tr');tr.append(el('th','',v.label),el('td','',v.min===v.max?v.min+' '+v.unit:v.min+'–'+v.max+' '+v.unit));table.append(tr)}
  panel.append(table);for(const n of bodyProfile.fitNotes||[])panel.append(el('p','meta',n));content.append(panel);
  const actions=el('section','bodyPanel widePanel'),save=el('button','primary','保存身体测量更新');
  actions.append(el('p','meta','仅变更身体净尺寸；保留测量历史。旧笔记的测量日期未知，请勿视为当前数据。'),save);content.append(actions);
  save.onclick=async()=>{
    const updates={};for(const input of content.querySelectorAll('input[data-metric]')){
      const old=bodyProfile.measurements[input.dataset.metric].value;
      const num=Number(input.value);if(!Number.isFinite(num)||num<=0||num>500){toast('请输入有效尺寸 (0–500 cm)');return}
      if(num!==old)updates[input.dataset.metric]=num
    }
    if(!Object.keys(updates).length){toast('没有变化，无需保存');return}
    save.disabled=true;
    try{bodyProfile=await write('body/BODY','PUT',{version:bodyProfile.version,measurements:updates});render();toast('身体数据已更新，并保留历史')}
    catch(e){toast('保存失败：'+e.message)}finally{save.disabled=false}
  }
}
function labelInput(parent,title,value,type='input'){const lab=el('label','',title),input=el(type);input.value=value||'';lab.append(input);parent.append(lab);return input}
function detail(p){const box=$('#detailBody');box.replaceChildren();const layout=el('div','detailLayout'),pic=image(p.image,p.name);pic.onclick=()=>zoom(p.image);layout.append(pic);const info=el('div');info.append(el('span','tag',p.id+' · '+p.category),el('h3','',p.name),el('p','',p.spec),link(p.url,'打开商品链接 ↗'));const fields=el('div','fieldGrid');
const rating=labelInput(fields,'我的评分（0 表示未评分）',p.rating||0);rating.type='number';rating.min='0';rating.max='5';
const owned=labelInput(fields,'持有状态',p.owned),fit=labelInput(fields,'合身反馈',p.fit),comment=labelInput(fields,'我的备注',p.comment,'textarea');comment.parentElement.classList.add('wide');const save=el('button','primary','保存评价');save.onclick=async()=>{save.disabled=true;try{const updated=await write('products/'+p.id,'PUT',{version:p.version,rating:Number(rating.value),owned:owned.value,fit:fit.value,comment:comment.value});Object.assign(p,updated);indexProducts();render();$('#detail').close();toast('评价已保存到本地')}catch(e){toast(e.message)}finally{save.disabled=false}};fields.append(save);info.append(fields,el('p','',p.ratingSource==='user'?'评分来源：本人在应用中确认。':'评分来自原表，可能包含批量默认值；在这里保存后才标记为本人确认。'));layout.append(info);box.append(layout);
const table=el('table','detailsTable');for(const [key,value]of Object.entries(p.fields||{})){if(!value)continue;const tr=el('tr');tr.append(el('th','',key));const td=el('td');String(value).split('\n').forEach((line,i)=>{if(i)td.append(document.createElement('br'));if(/^https?:\/\//.test(line))td.append(link(line,line));else td.append(document.createTextNode(line))});tr.append(td);table.append(tr)}box.append(table);$('#detail').showModal()}
function editLook(l){draft=structuredClone(l||{name:'',scene:'',rating:0,comment:'',slots:{}});draft.slots ||= {};$('#editorTitle').textContent=l?'编辑 '+l.id:'新建搭配';$('#saveState').textContent='选择单品后，点击保存搭配。';$('#deleteLook').hidden=!l;renderEditor();$('#editor').showModal()}
async function removeLook(l,button){
  if(!l?.id||!l.version)return toast('缺少搭配版本，请刷新后重试。');
  if(!window.confirm('确定删除「'+(l.name||l.id)+'」？\n仅删除搭配记录，不会删除商品或照片文件。'))return;
  button.disabled=true;
  try{
    await write('looks/'+encodeURIComponent(l.id),'DELETE',{version:l.version});
    looks=looks.filter(x=>x.id!==l.id);
    if($('#editor').open&&draft?.id===l.id)$('#editor').close();
    tab='looks';render();toast('搭配已删除，商品和照片未受影响');
  }catch(e){toast('删除失败：'+e.message)}finally{button.disabled=false}
}
function renderSlotGroup(parent,title,items){
  parent.append(el('h3','slotGroupTitle',title));const group=el('div','slots');
  for(const [key,label]of Object.entries(items)){const p=byID[draft.slots?.[key]],b=el('button','slot');b.type='button';b.append(image(p?.image,p?.name),el('b','',label),el('span','',p?p.id+' · '+p.color:'＋ 选择单品'));b.onclick=()=>openPicker(key);group.append(b)}
  parent.append(group)
}
function renderEditor(){
  const body=$('#editorBody');body.replaceChildren();const fields=el('div','fieldGrid');
  const name=labelInput(fields,'搭配名称',draft.name),scene=labelInput(fields,'使用场景',draft.scene);
  name.oninput=()=>draft.name=name.value;scene.oninput=()=>draft.scene=scene.value;
  body.append(fields,el('p','meta','由内到外搭配：贴身内搭 → 中层 → 外套。没有穿的层次留空即可。'));
  const head=el('div','hairSelection'),chosen=hairByID[draft.slots?.hair];
  head.append(el('h3','slotGroupTitle','发型参考'));
  const chooseHair=el('button','slot hairSlot');chooseHair.type='button';chooseHair.append(image(chosen?.image,chosen?.name),el('b','','发型'),el('span','',chosen?chosen.name+' · '+chosen.id:'＋ 选择发型参考'));
  chooseHair.onclick=()=>openPicker('hair');head.append(chooseHair);body.append(head);
  renderSlotGroup(body,'服装与鞋袜',garmentSlots);
  body.append(el('p','meta','配饰按类型分别选择，可以同时佩戴多种配饰；不需要的留空。'));
  renderSlotGroup(body,'配饰',accessorySlots);
  renderSlotGroup(body,'其他',{'other':'其他单品'});
  const tail=el('div','fieldGrid'),rating=labelInput(tail,'整套喜爱程度（0 表示未评分）',draft.rating||0);
  rating.type='number';rating.min='0';rating.max='5';rating.oninput=()=>draft.rating=Number(rating.value);
  const comment=labelInput(tail,'一句话评价',draft.comment,'textarea');comment.oninput=()=>draft.comment=comment.value;body.append(tail);
  if(draft.photo){const img=el('img','photo');WardrobeTransport.media(draft.photo).then(v=>img.src=v).catch(()=>{});img.alt='实穿参考';body.append(img)}
  if(draft.candidates)body.append(el('p','',draft.candidates))
}
function openPicker(key){
  picking=key;$('#pickerTitle').textContent='选择'+slots[key];$('#pickerSearch').value='';
  if(key==='hair'){selectOptions('#pickerCategory',[['','全部发型'],...[...new Set(hairStyles.map(h=>h.category))].map(c=>[c,c])]);renderPicker();$('#picker').showModal();return}
  const cats=[['','全部分类'],...[...new Set(products.map(p=>p.category))].sort().map(c=>[c,c])];
  if(recommendations[key]||accessoryKeywords[key])cats.splice(1,0,['__suggested__','适合此部位（推荐）']);
  selectOptions('#pickerCategory',cats, recommendations[key]||accessoryKeywords[key]?'__suggested__':'');
  renderPicker();$('#picker').showModal()
}
function renderPicker(){
  const q=$('#pickerSearch').value.trim().toLowerCase(),cat=$('#pickerCategory').value,grid=$('#pickerGrid');grid.replaceChildren();
  if(picking==='hair'){
    hairStyles.filter(h=>(!q||(h.name+h.category+h.id).toLowerCase().includes(q))&&(!cat||h.category===cat)).forEach(h=>grid.append(hairCard(h,()=>{draft.slots.hair=h.id;$('#picker').close();renderEditor();$('#saveState').textContent='发型已选择，保存后生效。'})));
    if(!grid.children.length)grid.append(el('p','empty','没有匹配的发型参考'));return
  }
  const list=products.filter(p=>(!q||p.search.includes(q))&&(!cat||(cat==='__suggested__'?isRecommended(p,picking):p.category===cat)));
  list.forEach(p=>grid.append(productCard(p,()=>{draft.slots[picking]=p.id;$('#picker').close();renderEditor();$('#saveState').textContent='单品已选择，保存后生效。'})));
  if(!list.length)grid.append(el('p','empty',cat==='__suggested__'?'没有对应的已录入单品，可切换「全部分类」后搜索。':'没有匹配单品，可切换全部分类。'))
}
function indexProducts(){byID={};products.forEach(p=>{p.search=JSON.stringify([p.id,p.name,p.color,p.size,p.spec,p.category,p.fields]).toLowerCase();byID[p.id]=p});products.sort((a,b)=>(b.date||'').localeCompare(a.date||'')||a.id.localeCompare(b.id))}
$('#saveLook').onclick=async()=>{const b=$('#saveLook');b.disabled=true;try{const saved=await write(draft.id?'looks/'+draft.id:'looks',draft.id?'PUT':'POST',draft);const i=looks.findIndex(l=>l.id===saved.id);if(i>=0)looks[i]=saved;else looks.unshift(saved);$('#editor').close();tab='looks';render();toast('搭配已保存')}catch(e){$('#saveState').textContent=e.message}finally{b.disabled=false}};
$('#deleteLook').onclick=()=>removeLook(draft,$('#deleteLook'));
$('#clearSlot').onclick=()=>{draft.slots[picking]='';$('#picker').close();renderEditor()};
document.querySelectorAll('[data-close]').forEach(b=>b.onclick=()=>$('#'+b.dataset.close).close());
function changeTab(to){if(tab==='skills' && to!=='skills' && skillDirty() && !window.confirm('当前 Skill 尚未保存，确定离开？'))return;tab=to;render()}
$('#refreshWorkspace').onclick=()=>location.reload();$('#newLook').onclick=()=>editLook();$('#wardrobeTab').onclick=()=>changeTab('wardrobe');$('#ordersTab').onclick=async()=>{changeTab('orders');if(tab==='orders')await window.orderUI?.load()};$('#looksTab').onclick=()=>changeTab('looks');$('#hairTab').onclick=()=>changeTab('hair');$('#bodyTab').onclick=()=>changeTab('body');$('#skillsTab').onclick=async()=>{changeTab('skills');if(tab==='skills')await loadSkills()};
function skillDirty(){return currentSkill && $('#skillContent').value!==currentSkill.content}
function skillStatus(message){$('#skillStatus').textContent=message}
function skillButtons(){const dirty=skillDirty();$('#skillSave').disabled=skillBusy||!currentSkill||!dirty;$('#skillReset').disabled=skillBusy||!currentSkill||!currentSkill.modified;}
function renderSkillList(){const wrap=$('#skillList');wrap.replaceChildren();
  for(const s of skillItems){const b=el('button',s.name===currentSkill?.name?'skillSelected':'');b.type='button';b.append(el('strong','',skillTitles[s.name]||s.name),el('span','meta',s.modified?'本地已修改 · v'+s.version:'内置模板'),el('small','',s.name));b.onclick=()=>selectSkill(s.name);wrap.append(b)}
}
async function loadSkills(){
 try{skillItems=await api('skills');renderSkillList();if(!currentSkill)await selectSkill(skillItems[0]?.name);else if(!skillItems.some(s=>s.name===currentSkill.name))await selectSkill(skillItems[0]?.name)}
 catch(e){skillStatus('加载 Skill 失败：'+e.message)}
}
async function selectSkill(name){
 if(!name || skillBusy)return;
 if(skillDirty()&&!window.confirm('当前 Skill 有未保存修改，确定放弃并切换？'))return;
 const request=++skillRequest;
 skillStatus('正在读取 Skill…');$('#skillSave').disabled=true;
 try{const data=await api('skills/'+encodeURIComponent(name));if(request!==skillRequest)return;
   currentSkill=data;$('#skillContent').value=data.content;$('#skillTitle').textContent=skillTitles[name]||name;
   $('#skillMeta').textContent=name+' · '+(data.modified?'本地修改版本 v'+data.version:'内置默认版本');
   if($('#skillPreview').hidden===false)$('#skillPreview').textContent=data.content;
   skillStatus(data.modified?'本地自定义内容已生效。保存后 MCP 会读取新的版本。':'当前使用内置模板，可直接编辑后保存为本地版本。');renderSkillList();skillButtons();
 }catch(e){skillStatus('加载失败：'+e.message)}
}
$('#skillContent').oninput=()=>{skillButtons();skillStatus(skillDirty()?'有未保存的修改。':'当前内容已保存。');if(!$('#skillPreview').hidden)$('#skillPreview').textContent=$('#skillContent').value};
$('#skillEditMode').onclick=()=>{$('#skillEditMode').classList.add('active');$('#skillViewMode').classList.remove('active');$('#skillContent').hidden=false;$('#skillPreview').hidden=true};
$('#skillViewMode').onclick=()=>{$('#skillViewMode').classList.add('active');$('#skillEditMode').classList.remove('active');$('#skillPreview').textContent=$('#skillContent').value;$('#skillPreview').hidden=false;$('#skillContent').hidden=true};
$('#skillSave').onclick=async()=>{
 if(!currentSkill||!skillDirty()||skillBusy)return;skillBusy=true;skillButtons();const original=currentSkill;
 try{const result=await write('skills/'+encodeURIComponent(original.name),'PUT',{version:original.version,content:$('#skillContent').value});
 currentSkill=result;skillStatus('已保存 v'+result.version+'，MCP 下一次读取将使用新内容。');
 await refreshSkillList();toast('Skill 已保存至本地 SQLite');
 }catch(e){skillStatus('保存失败：'+e.message)}finally{skillBusy=false;skillButtons()}
};
$('#skillReset').onclick=async()=>{
 if(!currentSkill?.modified||skillBusy)return;
 if(!window.confirm('恢复「'+(skillTitles[currentSkill.name]||currentSkill.name)+'」为仓库内置模板？本地修改会清除。'))return;
 skillBusy=true;skillButtons();try{
 const result=await write('skills/'+encodeURIComponent(currentSkill.name),'DELETE',{version:currentSkill.version});
 currentSkill=result;$('#skillContent').value=result.content;$('#skillMeta').textContent=result.name+' · 内置默认版本';
 if(!$('#skillPreview').hidden)$('#skillPreview').textContent=result.content;
 skillStatus('已恢复默认模板，MCP 将读取此版本。');await refreshSkillList();toast('已恢复 Skill 默认内容')
 }catch(e){skillStatus('恢复失败：'+e.message)}finally{skillBusy=false;skillButtons()}
};
async function refreshSkillList(){skillItems=await api('skills');renderSkillList();$('#skillMeta').textContent=currentSkill.name+' · '+(currentSkill.modified?'本地修改版本 v'+currentSkill.version:'内置默认版本')}
let searchTimer;$('#search').oninput=()=>{clearTimeout(searchTimer);searchTimer=setTimeout(render,120)};for(const id of ['category','ratingFilter','sourceFilter','ownedFilter','fitFilter'])$('#'+id).onchange=render;$('#pickerSearch').oninput=renderPicker;$('#pickerCategory').onchange=renderPicker;
async function loadWardrobe(){try{if(!await WardrobeTransport.check()){window.dispatchEvent(new Event('wardrobe:connection-required'));return}[products,looks,hairStyles,bodyProfile]=await Promise.all([api('products'),api('looks'),api('hairstyles'),api('body/BODY').catch(()=>null)]);hairByID=Object.fromEntries(hairStyles.map(h=>[h.id,h]));indexProducts();const cats=[['','全部分类'],...[...new Set(products.map(p=>p.category))].sort().map(c=>[c,c])];selectOptions('#category',cats);selectOptions('#ownedFilter',[['','全部持有状态'],...[...new Set(products.map(p=>p.owned).filter(Boolean))].map(v=>[v,v])]);selectOptions('#fitFilter',[['','全部合身状态'],...[...new Set(products.map(p=>p.fit).filter(Boolean))].map(v=>[v,v])]);render();if(tab==='orders')await window.orderUI?.load()}catch(e){$('#summary').textContent='加载失败：'+e.message}}
window.addEventListener('wardrobe:paired',loadWardrobe);loadWardrobe();
