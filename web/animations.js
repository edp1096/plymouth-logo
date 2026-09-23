// Both image and animation layers share one ordered draft; installed state is independent.
const maxAnimationLayers=8,maxImageLayers=8;
let animationItems=[],draftAnimations=[],activeAnimation=-1,animationIssue='',sceneReady=false,sceneLoading=false;
function animationMeta(item){return animationItems.find(entry=>entry.id===item);}
function newAnimation(item='default',position={x:50,y:70}){
 const meta=animationMeta(item);
 return {kind:'animation',item,size:item==='default'?32:160,fps:meta?.fps||(item==='default'?15:30),position:{...position},center:false};
}
function copyAnimation(layer){return {...layer,kind:layer.kind||'animation',position:{...layer.position}};}
function stateAnimations(state){
 if(Array.isArray(state.animations)) return state.animations.map(copyAnimation);
 const item=state.spinner_item||'default';
 return [{...newAnimation(item,state.spinner||{x:50,y:70}),size:state.spinner_size||(item==='default'?32:160),fps:state.spinner_fps||animationMeta(item)?.fps||(item==='default'?15:30),center:item==='default'}];
}
function animationDimensions(meta,size){return meta.width>=meta.height?[size,Math.max(1,Math.floor(meta.height*size/meta.width))]:[Math.max(1,Math.floor(meta.width*size/meta.height)),size];}
function imageCount(){return draftAnimations.filter(l=>l.kind==='image').length;}
function animationCount(){return draftAnimations.length-imageCount();}
function layerName(layer){return layer.kind==='image'?(layer.name||tr('이미지')):(layer.item==='default'?tr('기본 회전'):(animationMeta(layer.item)?.name||layer.item));}
function animationBudget(){
 let bytes=0,sources=0,imageBytes=0,animationBytes=0;
 if(imageCount()>8||animationCount()>8)return {bytes,issue:'이미지와 애니메이션은 각각 최대 8개입니다.'};
 for(const layer of draftAnimations){
  const staticImage=layer.kind==='image';
  const meta=staticImage?{width:layer.source_width,height:layer.source_height,frames:1}:animationMeta(layer.item);
  if(!meta||!meta.width||!meta.height||(staticImage&&!layer.image))return {bytes,issue:'레이어 리소스를 읽지 못했습니다. 항목을 바꾸거나 삭제하세요.'};
  if(!Number.isInteger(layer.size)||layer.size<(staticImage?1:32)||layer.size>(staticImage?1920:320)||(!staticImage&&(!Number.isFinite(layer.fps)||layer.fps<1||layer.fps>60))||![layer.position.x,layer.position.y].every(n=>Number.isFinite(n)&&n>=0&&n<=100))return {bytes,issue:'레이어 크기·위치·재생 속도를 확인하세요.'};
  const [w,h]=animationDimensions(meta,layer.size);const memory=w*h*4*meta.frames;bytes+=memory;
  if(staticImage)imageBytes+=memory;else animationBytes+=memory;
  if(staticImage)sources+=Math.floor(layer.image.length*3/4)-(layer.image.endsWith('==')?2:layer.image.endsWith('=')?1:0);
 }
 return {bytes,imageBytes,animationBytes,issue:sources>32*1024*1024?'이미지 원본 합계는 32 MiB 이하여야 합니다.':imageBytes>16*1024*1024?'이미지 합계가 16 MiB를 넘습니다. 크기를 줄이거나 항목을 삭제하세요.':animationBytes>64*1024*1024?'애니메이션 합계가 64 MiB를 넘습니다. 크기를 줄이거나 항목을 삭제하세요.':''};
}
function animationControls(){
 const empty=activeAnimation<0,locked=busy||sceneLoading||!sceneReady;
 const staticImage=draftAnimations[activeAnimation]?.kind==='image';
 for(const id of ['spinnerItem','spinnerSize','spinnerX','spinnerY','animationFPS'])$(id).disabled=locked||empty;
 $('animationLayer').disabled=locked||empty;
 $('addAnimation').disabled=locked||animationCount()>=8;
 $('addImage').disabled=locked||imageCount()>=8;
 $('replaceImage').disabled=locked||!staticImage;
 $('file').disabled=locked;$('replaceFile').disabled=locked||!staticImage;
 $('removeAnimation').disabled=locked||empty;
 $('animationBack').disabled=locked||activeAnimation<=0;
 $('animationFront').disabled=locked||empty||activeAnimation>=draftAnimations.length-1;
 $('animationType').hidden=staticImage||empty;$('fpsControl').hidden=staticImage||empty;
 $('replaceImage').hidden=!staticImage;
}
function updateAnimationLabels(){
 $('animationLayer').replaceChildren();
 draftAnimations.forEach((layer,i)=>{const option=document.createElement('option');option.value=String(i);option.textContent=`${i+1} · ${layer.kind==='image'?tr('이미지'):tr('애니메이션')} · ${layerName(layer)}`;$('animationLayer').appendChild(option);});
 $('animationLayer').value=String(activeAnimation);$('animationCount').textContent=`${draftAnimations.length} / 16`;
}
function selectAnimation(index){
 activeAnimation=index>=0&&index<draftAnimations.length?index:-1;updateAnimationLabels();
 const layer=draftAnimations[activeAnimation];
 if(layer){
  if(layer.kind!=='image'){
   if(!animationMeta(layer.item)&&![...$('spinnerItem').children].some(o=>o.value===layer.item)){const option=document.createElement('option');option.value=layer.item;option.textContent=layer.item;$('spinnerItem').appendChild(option);}
   $('spinnerItem').value=layer.item;
  }
  $('spinnerSize').min=layer.kind==='image'?1:32;$('spinnerSize').max=layer.kind==='image'?1920:320;$('spinnerSize').value=layer.size;
  $('spinnerX').value=layer.position.x;$('spinnerY').value=layer.position.y;$('animationFPS').value=layer.fps||30;
  $('spinnerSizeValue').textContent=layer.size+' px';$('spinnerXValue').textContent=layer.position.x+'%';$('spinnerYValue').textContent=layer.position.y+'%';
 }
 controls();
}
function clearAnimationImages(which){const container=$(which+'Layers');for(const image of container.children)clearTimeout(image.previewTimer);container.replaceChildren();}
function renderAnimationLayers(which,layers,installed=false,scene=false){
 const container=$(which+'Layers');
 while(container.children.length>layers.length){const last=container.children[container.children.length-1];clearTimeout(last.previewTimer);container.removeChild(last);}
 layers.forEach((layer,i)=>{
  let image=container.children[i];if(!image){image=document.createElement('img');image.className='animation-layer';image.alt='';container.appendChild(image);}
  const staticImage=layer.kind==='image';
  const meta=installed?layer:staticImage?{width:layer.source_width,height:layer.source_height}:animationMeta(layer.item);
  if(!meta){image.style.display='none';return;}
  const [w,h]=installed?[layer.width,layer.height]:animationDimensions(meta,layer.size);
  image.style.width=(w/1920*100)+'%';image.style.height=(h/1080*100)+'%';placeLogo(image,layer.position.x,layer.position.y);
  if(layer.center)image.style.transform='translate(-50%, -50%)';image.style.zIndex=String(i+2);
  const source=installed?`/${scene?'scene-layer':'installed-animation-layer'}.png?index=${i}&v=${revision}`:staticImage?layer.preview:`/animation-layer.png?item=${encodeURIComponent(layer.item)}&size=${layer.size}&fps=${layer.fps}`;
  if(image.previewSource!==source){
   image.previewSource=source;clearTimeout(image.previewTimer);image.style.display='none';
   image.onload=()=>{if(image.getAttribute('src')===image.previewSource)image.style.display='block';};
   image.previewTimer=setTimeout(()=>{image.src=source;},staticImage?0:120);
  }
  image.onerror=()=>{image.style.display='none';setText(installed?'currentPreviewNote':'animationStatus','레이어 미리보기를 읽지 못했습니다.');};
 });
}
function updateAnimationMemory(result=animationBudget()){
 $('animationMemory').textContent=`${tr('이미지')} ${((result.imageBytes||0)/1024/1024).toFixed(1)} / 16 MiB · ${tr('애니메이션')} ${((result.animationBytes||0)/1024/1024).toFixed(1)} / 64 MiB`;
}
function refreshDraftAnimations(){
 const result=animationBudget();animationIssue=result.issue;setText('animationStatus',animationIssue);
 updateAnimationMemory(result);
 if(animationIssue)clearAnimationImages('draft');else renderAnimationLayers('draft',draftAnimations);
 controls();
}
function changeAnimations(){dirty=true;++presetGeneration;$('preset').value='';setText('presetDescription','직접 편집');refreshDraftAnimations();}
function loadDraftAnimations(layers){draftAnimations=layers.map(copyAnimation);sceneReady=true;selectAnimation(draftAnimations.length?0:-1);refreshDraftAnimations();}
function editActiveAnimation(){
 const layer=draftAnimations[activeAnimation];if(!layer)return;
 layer.size=Number($('spinnerSize').value);if(layer.kind!=='image')layer.fps=$('animationFPS').value===''?NaN:Number($('animationFPS').value);
 layer.position={x:Number($('spinnerX').value),y:Number($('spinnerY').value)};
 $('spinnerSizeValue').textContent=layer.size+' px';$('spinnerXValue').textContent=layer.position.x+'%';$('spinnerYValue').textContent=layer.position.y+'%';changeAnimations();
}
function encodeBlob(blob){return new Promise((resolve,reject)=>{const reader=new FileReader();reader.onload=()=>resolve(reader.result.split(',')[1]);reader.onerror=()=>reject(Error(tr('이미지를 읽지 못했습니다.')));reader.readAsDataURL(blob);});}
async function newImage(blob,name=''){
 if(blob.size>12*1024*1024)throw Error(tr('12 MB 이하 이미지를 선택하세요.'));
 const image=await encodeBlob(blob);
 const meta=await(await api('/api/inspect-image',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({image})})).json();
 return {kind:'image',name,image,preview:'data:image/png;base64,'+meta.preview,source_width:meta.width,source_height:meta.height,size:Math.max(32,Math.min(320,Math.max(meta.width,meta.height))),position:{x:50,y:50},center:false};
}
async function loadStateLayers(state,blob){
 if(state.layer_error||state.animation_error)throw Error(state.layer_error||state.animation_error);
 if(Array.isArray(state.layers)){
  const layers=[];
  for(let i=0;i<state.layers.length;i++){
   const l=state.layers[i];
   if(l.kind==='image'){
    const source=await(await api('/api/layer-source?index='+i)).blob();
    const loaded=await newImage(source,l.name);
    layers.push({...loaded,...l,image:loaded.image,preview:loaded.preview});
   }else layers.push(copyAnimation(l));
  }
  return layers;
 }
 const logo=await newImage(blob,tr('기존 로고'));
 // Legacy logo resizing did not upscale small source images.
 logo.size=Math.min(state.size||320,Math.max(logo.source_width,logo.source_height));
 logo.position={...(state.logo_position||{x:50,y:50})};
 return [logo,...stateAnimations(state)];
}
function scenePayload(){return draftAnimations.map(l=>l.kind==='image'?{kind:l.kind,name:l.name,image:l.image,size:l.size,position:l.position,center:!!l.center}:{kind:'animation',item:l.item,size:l.size,fps:l.fps,position:l.position,center:!!l.center});}
function initAnimationEditor(){
 $('animationLayer').onchange=()=>selectAnimation(Number($('animationLayer').value));
 $('spinnerItem').onchange=()=>{const previous=draftAnimations[activeAnimation];if(!previous||previous.kind==='image')return;draftAnimations[activeAnimation]={...newAnimation($('spinnerItem').value,previous.position),center:previous.center};selectAnimation(activeAnimation);changeAnimations();};
 for(const id of ['spinnerSize','spinnerX','spinnerY','animationFPS'])$(id).oninput=editActiveAnimation;
 $('addAnimation').onclick=()=>{if(busy||sceneLoading||!sceneReady||animationCount()>=8)return;draftAnimations.push(newAnimation());selectAnimation(draftAnimations.length-1);changeAnimations();};
 $('addImage').onclick=()=>$('file').click();$('replaceImage').onclick=()=>$('replaceFile').click();
 async function upload(input,replace){
  const files=[...input.files];input.value='';if(!files.length||busy||sceneLoading||!sceneReady)return;
  if(!replace&&imageCount()+files.length>8){setText('animationStatus','이미지와 애니메이션은 각각 최대 8개입니다.');return;}
  const selected=activeAnimation,generation=++presetGeneration;
  sceneLoading=true;controls();
  try{
   const loaded=[];for(const file of files)loaded.push(await newImage(file,file.name));
   if(generation!==presetGeneration)return;
   if(replace){const old=draftAnimations[selected];if(old?.kind!=='image')return;draftAnimations[selected]={...loaded[0],size:old.size,position:old.position,center:old.center};}
   else draftAnimations.push(...loaded);
   selectAnimation(replace?selected:draftAnimations.length-1);changeAnimations();
  }catch(e){setLiteral('animationStatus',e.message);}
  finally{sceneLoading=false;controls();}
 }
 $('file').onchange=()=>upload($('file'),false);$('replaceFile').onchange=()=>upload($('replaceFile'),true);
 $('removeAnimation').onclick=()=>{if(busy||sceneLoading||activeAnimation<0)return;draftAnimations.splice(activeAnimation,1);selectAnimation(Math.min(activeAnimation,draftAnimations.length-1));changeAnimations();};
 function reorder(delta){const next=activeAnimation+delta;if(busy||sceneLoading||activeAnimation<0||next<0||next>=draftAnimations.length)return;[draftAnimations[activeAnimation],draftAnimations[next]]=[draftAnimations[next],draftAnimations[activeAnimation]];selectAnimation(next);changeAnimations();}
 $('animationBack').onclick=()=>reorder(-1);$('animationFront').onclick=()=>reorder(1);
}
