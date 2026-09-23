const token = location.hash.slice(1) || sessionStorage.getItem('logo-token');
if (token) sessionStorage.setItem('logo-token', token);
history.replaceState(null, '', '/');
const $ = id => document.getElementById(id);
let busy=false,dirty=false,revision=null,currentUrl,background="#000000",presetGeneration=0,validBackground=true;
let hostReady = false, currentPreviewState = null, previewSource = 'current', themePreviewGeneration = 0;
async function api(path, options={}) {
  const response = await fetch(path, {...options, headers: {'X-App-Token':token || '', ...options.headers}});
  if (!response.ok) {const error = await response.json(); throw Error(error.error || tr('요청 실패'));}
  return response;
}
function placeLogo(image,x,y){image.style.left=x+'%';image.style.top=y+'%';image.style.transform=`translate(-${x}%, -${y}%)`;}
function controls() {
  animationControls();
  $('preset').disabled=busy||sceneLoading;
  $('backgroundColor').disabled=busy; $('backgroundHex').disabled=busy;
  $('apply').disabled = busy || sceneLoading || !sceneReady || !hostReady || !validBackground || !!animationIssue;
  $('restore').disabled = busy || !hostReady;
  $('original').disabled = busy || !hostReady;

}
function setBackground(value) {
  background=value.toLowerCase();validBackground=true;
  $('backgroundColor').value=background;$('backgroundHex').value=background;
  $('backgroundHex').style.borderColor='';$('backgroundHex').title='';
  $('draftScreen').style.backgroundColor=background;
}
function editBackground(value) {
  dirty=true;++presetGeneration;$('preset').value='';
  setText('presetDescription','직접 편집');
  validBackground=/^#[0-9a-fA-F]{6}$/.test(value);
  if(validBackground) setBackground(value);
  else { $('backgroundHex').style.borderColor='#ff7171';$('backgroundHex').title=tr('#과 6자리 HEX 값을 입력하세요. 예: #101827'); }
  controls();
}
$('backgroundColor').oninput=()=>editBackground($('backgroundColor').value);
$('backgroundHex').oninput=()=>editBackground($('backgroundHex').value.trim());
function renderSpinner(which, item, x, y, size){
  const ring=$(which+'Spinner'),animation=$(which+'Animation');
  ring.style.width=(size/1920*100)+'%';ring.style.height='auto';ring.style.aspectRatio='1';ring.style.margin='0';ring.style.translate='-50% -50%';
  animation.style.width=(size/1920*100)+'%';
  ring.style.left=x+'%';ring.style.top=y+'%';
  // Script sprites align inside the remaining space, including at screen edges.
  placeLogo(animation,x,y);
  ring.style.display=item==='default'?'block':'none';
  animation.style.display=item==='default'?'none':'block';
  if(item!=='default') { const src=which==='current'?'/installed-spinner.png?v='+revision:'/spinner/'+item+'.png';if(animation.getAttribute('src')!==src)animation.src=src; }
}
async function start(action) {
  if(busy || !hostReady) return;
  if(action==='apply' && (sceneLoading || !sceneReady || !validBackground || !!animationIssue)) return;
  if(action==='restore-original' && !confirm(tr('최초 변경 전 백업으로 원본 부팅 화면을 복원할까요?'))) return;
  if(action==='restore' && !confirm(tr('마지막 적용 직전의 부팅 화면으로 복원할까요?'))) return;
  busy=true; controls(); setText('status','작업 요청 중…');
  try {
    await api('/api/'+action,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(action==='apply'?{background,layers:scenePayload()}:{})});
  } catch(e) { setText('status',e.message); busy=false; controls(); }
}
$('apply').onclick=()=>start('apply'); $('restore').onclick=()=>start('restore'); $('original').onclick=()=>start('restore-original');
async function poll() {
  try {
    const state=await (await api('/api/status')).json();
    hostReady=!state.host_error;
    setText('hostStatus',state.host_error || '현재 환경에서 적용·복원할 수 있습니다.');
    busy=state.busy; controls(); setLiteral('kernel',state.kernel);
    setText('status',state.message || ''); $('dot').style.color=state.ok?'#ffad70':'#ff7171';
    if(state.prepared && (revision === null || revision !== state.revision)) {
      const nextRevision = state.revision;
      const blob=await (await api('/api/preview')).blob();
      if (currentUrl) URL.revokeObjectURL(currentUrl);
      currentUrl=URL.createObjectURL(blob);
      currentPreviewState=state;
      revision = nextRevision;
      await showSelectedTheme();
      if(!dirty&&!sceneLoading){
        const generation=presetGeneration;
        sceneLoading=true;controls();
        try {
          const layers=await loadStateLayers(state,blob);
          if(!dirty&&generation===presetGeneration){setBackground(state.background||'#000000');loadDraftAnimations(layers);}
        } catch(e){sceneReady=false;setLiteral('animationStatus',e.message);}
        finally{sceneLoading=false;controls();}
      }

    }
  } catch(e) { hostReady=false;controls();setText('hostStatus','적용 환경을 확인하지 못했습니다.');setText('status',e.message); }
  setTimeout(poll,1500);
}
// The left-hand preview is read-only: switching sources never alters the draft.
async function showSelectedTheme() {
  const generation=++themePreviewGeneration;
  for(const id of ['currentLogo','currentSpinner','currentAnimation']) $(id).style.display='none';
  $('systemLayers').replaceChildren();
  clearAnimationImages('current');
  $('currentScreen').style.backgroundImage='none';
  $('currentScreen').style.backgroundColor='#20242b';
  $('currentEmpty').style.display='block';
  setText('currentEmpty','현재 설정을 읽는 중…');
  setText('currentSize','확인 중');
  setText('currentPreviewNote','');
  const state=currentPreviewState;
  if(previewSource==='current' && state?.applied) {
    if(Array.isArray(state.layers)){
      $('currentEmpty').style.display='none';
      setLiteral('currentSize',state.layers.length+' / 16');
      setText('currentPreviewNote','현재 선택된 사용자 테마');
      $('currentScreen').style.backgroundColor=state.background||'#000000';
      renderAnimationLayers('current',state.layers,true,true);
      if(state.layer_error)setLiteral('currentPreviewNote',state.layer_error);
      return;
    }
    const current=$('currentLogo');
    current.onload=()=>{current.style.width=`${current.naturalWidth/1920*100}%`;};
    current.src=currentUrl;current.style.display='block';
    $('currentEmpty').style.display='none';
    setLiteral('currentSize',`${state.size} px`);
    setText('currentPreviewNote','현재 선택된 사용자 테마');
    $('currentScreen').style.backgroundColor=state.background || '#000000';
    const logoPos=state.logo_position || {x:50,y:50};placeLogo(current,logoPos.x,logoPos.y);
    const pos=state.spinner || {x:50,y:70};
    if(Array.isArray(state.animations)) renderAnimationLayers('current',state.animations,true);
    else renderSpinner('current',state.spinner_item || 'default',pos.x,pos.y,state.spinner_size || 32);
    if(state.animation_error) setLiteral('currentPreviewNote',state.animation_error);
    return;
  }
  try {
    const theme=await (await api('/api/theme-preview?source='+previewSource)).json();
    if(generation!==themePreviewGeneration) return;
    if(!theme.available) {
      setText('currentEmpty','현재 테마를 미리 볼 수 없습니다.');
      setText('currentSize','미리보기 없음');
      setText('currentPreviewNote',theme.reason || '');
      return;
    }
    setLiteral('currentSize',theme.name);
    setText('currentPreviewNote',theme.note);
    $('currentEmpty').style.display='none';
    $('currentScreen').style.backgroundColor=theme.top;
    $('currentScreen').style.backgroundImage=`linear-gradient(to bottom, ${theme.top}, ${theme.bottom})`;
    for(const layer of theme.layers) {
      const img=document.createElement('img');
      img.className='system-theme-layer';img.alt='';img.src=layer.image;
      img.style.width=(layer.width/1920*100)+'%';
      img.style.height=(layer.height/1080*100)+'%';
      img.style.left=layer.x+'%';img.style.top=layer.y+'%';
      img.style.transform=layer.center?'translate(-50%, -50%)':`translate(-${layer.x}%, -${layer.y}%)`;
      $('systemLayers').appendChild(img);
    }
  } catch(e) {
    if(generation!==themePreviewGeneration) return;
    setText('currentEmpty','현재 테마를 미리 볼 수 없습니다.');
    setText('currentSize','미리보기 없음');
    setLiteral('currentPreviewNote',e.message);
  }
}
$('previewSource').onchange=()=>{previewSource=$('previewSource').value;showSelectedTheme();};
initAnimationEditor();
loadSpinnerItems();

// Keep an authenticated connection open so closing the window stops the local app.
async function watchWindow() {
  try {
    const response = await api('/api/window');
    const reader = response.body.getReader();
    while (!(await reader.read()).done) {}
  } catch (_) {}
  setTimeout(watchWindow, 250);
}
watchWindow();

async function loadPresets(){
  try {
    const presets=await (await api('/presets.json')).json();
    for(const preset of presets){const option=document.createElement('option');option.value=preset.id;option.textContent=preset.name;$('preset').appendChild(option);}
    $('preset').onchange=async()=>{
      const generation=++presetGeneration;
      const preset=presets.find(p=>p.id===$('preset').value);if(!preset)return;
      dirty=true;sceneLoading=true;controls();
      try{
        const blob=await (await api('/default.png')).blob();
        const logo=await newImage(blob,'Tux');
        if(generation!==presetGeneration)return;
        logo.size=Math.min(preset.size,Math.max(logo.source_width,logo.source_height));
        setBackground(preset.background);
        loadDraftAnimations([logo,{...newAnimation('default',preset.spinner),center:true}]);
        setText('presetDescription',preset.description);controls();

      }catch(e){setText('status',e.message);}
      finally{sceneLoading=false;controls();}
    };
  }catch(e){setText('presetDescription','프리셋을 불러오지 못했습니다.');}
}
loadPresets();

async function loadSpinnerItems(){
  try{
    const items=await (await api('/api/spinners')).json();
    animationItems=items;
    $('spinnerItem').replaceChildren();
    for(const item of items){const option=document.createElement('option');option.value=item.id;option.textContent=item.id==='default'?tr('기본 회전'):item.name;$('spinnerItem').appendChild(option);}
    controls();poll();
  }catch(e){setLiteral('status',tr('스피너 목록을 불러오지 못했습니다: ')+e.message);}
}

initLanguage();
