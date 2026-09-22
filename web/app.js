const token = location.hash.slice(1) || sessionStorage.getItem('logo-token');
if (token) sessionStorage.setItem('logo-token', token);
history.replaceState(null, '', '/');
const $ = id => document.getElementById(id);
let data = null, busy = false, dirty = false, previewUrl, revision = null, currentUrl, background = "#000000", presetGeneration = 0, validBackground = true, spinnerChoice = "default";
async function api(path, options={}) {
  const response = await fetch(path, {...options, headers: {'X-App-Token':token || '', ...options.headers}});
  if (!response.ok) {const error = await response.json(); throw Error(error.error || tr('요청 실패'));}
  return response;
}
function placeLogo(image,x,y){image.style.left=x+'%';image.style.top=y+'%';image.style.transform=`translate(-${x}%, -${y}%)`;}
function moveLogo(){
 const x=Number($('logoX').value),y=Number($('logoY').value);
 placeLogo($('logo'),x,y);$('logoXValue').textContent=x+'%';$('logoYValue').textContent=y+'%';
}
for(const id of ['logoX','logoY']) $(id).oninput=()=>{dirty=true;++presetGeneration;$('preset').value='';moveLogo();};
function scale() {
  $('sizeValue').textContent = $('size').value + ' px';
  const image = $('logo');
  const pixels = image.naturalWidth * Math.min(1, Number($('size').value) / Math.max(image.naturalWidth, image.naturalHeight, 1));
  $('logo').style.width = `${pixels / 1920 * 100}%`;
}
function controls() {
  $('spinnerSize').disabled=busy;
  $('spinnerItem').disabled=busy;
  $('preset').disabled=busy;
  $('backgroundColor').disabled=busy; $('backgroundHex').disabled=busy;
  $('apply').disabled = busy || !data || !validBackground;
  $('restore').disabled = busy;
  $('original').disabled = busy;
  $('file').disabled = busy;
  $('size').disabled = busy;
  $('logoX').disabled=busy;$('logoY').disabled=busy;
  $('spinnerX').disabled=busy; $('spinnerY').disabled=busy;
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
function showImage(blob) {
  if (previewUrl) URL.revokeObjectURL(previewUrl);
  previewUrl = URL.createObjectURL(blob);
  $('logo').onload = () => { $('logo').style.display='block'; scale(); };
  $('logo').src = previewUrl;
}
function renderSpinner(which, item, x, y, size){
  const ring=$(which+'Spinner'),animation=$(which+'Animation');
  ring.style.width=(size/1920*100)+'%';ring.style.height='auto';ring.style.aspectRatio='1';ring.style.margin='0';ring.style.translate='-50% -50%';
  animation.style.width=(size/1920*100)+'%';
  ring.style.left=x+'%';ring.style.top=y+'%';
  animation.style.left=x+'%';animation.style.top=y+'%';
  ring.style.display=item==='default'?'block':'none';
  animation.style.display=item==='default'?'none':'block';
  if(item!=='default') { const src=which==='current'?'/installed-spinner.png?v='+revision:'/spinner/'+item+'.png';if(animation.getAttribute('src')!==src)animation.src=src; }
}
$('spinnerItem').onchange=()=>{dirty=true;++presetGeneration;spinnerChoice=$('spinnerItem').value;$('spinnerSize').value=spinnerChoice==='default'?32:160;moveSpinner();};
function moveSpinner(){
  const x=$('spinnerX').value,y=$('spinnerY').value;
  renderSpinner('draft',spinnerChoice,x,y,Number($('spinnerSize').value));
  $('spinnerSizeValue').textContent=$('spinnerSize').value+' px';
  $('spinnerXValue').textContent=x+'%';$('spinnerYValue').textContent=y+'%';
}
for(const id of ['spinnerX','spinnerY','spinnerSize']) $(id).oninput=()=>{dirty=true;++presetGeneration;$('preset').value='';moveSpinner();};
$('size').oninput=()=>{dirty=true;++presetGeneration;$('preset').value=''; scale();};
$('file').onchange=async () => {
  const file=$('file').files[0]; if(!file) return;
  ++presetGeneration;$('preset').value='';
  if(file.size>12*1024*1024) { setText('status','12 MB 이하 이미지를 선택하세요.'); return; }
  try {
    const reader=new FileReader();
    data=await new Promise((resolve,reject)=>{reader.onload=()=>resolve(reader.result.split(',')[1]);reader.onerror=reject;reader.readAsDataURL(file);});
    dirty=true; showImage(file); setLiteral('filename',file.name); controls();
  } catch(e) { setText('status',String(e)); }
};
async function start(action) {
  if(action==='apply' && !validBackground) return;
  if(action==='restore-original' && !confirm(tr('최초 변경 전 백업으로 원본 부팅 화면을 복원할까요?'))) return;
  if(action==='restore' && !confirm(tr('마지막 적용 직전의 부팅 화면으로 복원할까요?'))) return;
  busy=true; controls(); setText('status','작업 요청 중…');
  try {
    await api('/api/'+action,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(action==='apply'?{image:data,size:Number($('size').value),logo_position:{x:Number($('logoX').value),y:Number($('logoY').value)},background,spinner_item:spinnerChoice,spinner_size:Number($('spinnerSize').value),spinner:{x:Number($('spinnerX').value),y:Number($('spinnerY').value)}}:{})});
  } catch(e) { setText('status',e.message); busy=false; controls(); }
}
$('apply').onclick=()=>start('apply'); $('restore').onclick=()=>start('restore'); $('original').onclick=()=>start('restore-original');
async function poll() {
  try {
    const state=await (await api('/api/status')).json();
    busy=state.busy; controls(); setLiteral('kernel',state.kernel);
    setText('status',state.message || ''); $('dot').style.color=state.ok?'#ffad70':'#ff7171';
    if(state.prepared && (revision === null || revision !== state.revision)) {
      const nextRevision = state.revision;
      const blob=await (await api('/api/preview')).blob();
      if (currentUrl) URL.revokeObjectURL(currentUrl);
      currentUrl=URL.createObjectURL(blob);
      const current=$('currentLogo');
      current.onload=()=>{current.style.width=`${current.naturalWidth/1920*100}%`;};
      current.src=currentUrl;
      current.style.display=state.applied?'block':'none';
      $('currentEmpty').style.display=state.applied?'none':'block';
      setText('currentEmpty','기본 시스템 테마 · 사용자 로고 없음');
      setText('currentSize',state.applied?`${state.size} px`:'기본 테마');
      $('currentScreen').style.backgroundColor=state.background || '#000000';
      const logoPos=state.logo_position || {x:50,y:50};placeLogo(current,logoPos.x,logoPos.y);
      const pos=state.spinner || {x:50,y:70};
      revision = nextRevision;
      renderSpinner('current',state.spinner_item || 'default',pos.x,pos.y,state.spinner_size || 32);
      if (!dirty) {
        setBackground(state.background || '#000000');
        spinnerChoice=state.spinner_item || 'default';$('spinnerItem').value=spinnerChoice;$('spinnerSize').value=state.spinner_size || (spinnerChoice==='default'?32:160);
        $('size').value=state.size || 320;
        $('logoX').value=logoPos.x;$('logoY').value=logoPos.y;moveLogo();
        $('spinnerX').value=pos.x;$('spinnerY').value=pos.y;moveSpinner();
        setText('filename',state.applied ? '현재 적용된 로고' : '기본 Tux · 아직 적용되지 않은 미리보기');
        showImage(blob);
        const reader=new FileReader();reader.onload=()=>{if (!dirty) { data=reader.result.split(',')[1]; controls(); }};reader.readAsDataURL(blob);
      }
    }
  } catch(e) { setText('status',e.message); }
  setTimeout(poll,1500);
}
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
      dirty=true;
      try{
        const blob=await (await api('/default.png')).blob();
        const encoded=await new Promise((resolve,reject)=>{const reader=new FileReader();reader.onload=()=>resolve(reader.result.split(',')[1]);reader.onerror=reject;reader.readAsDataURL(blob);});
        if(generation!==presetGeneration)return;
        data=encoded;setBackground(preset.background);spinnerChoice='default';$('spinnerItem').value=spinnerChoice;$('spinnerSize').value=32;
        $('logoX').value=50;$('logoY').value=50;moveLogo();
        $('size').value=preset.size;$('spinnerX').value=preset.spinner.x;$('spinnerY').value=preset.spinner.y;
        $('draftScreen').style.backgroundColor=background;
        setLiteral('filename',preset.name+' · Tux');setText('presetDescription',preset.description);
        showImage(blob);moveSpinner();scale();controls();
      }catch(e){setText('status',e.message);}
    };
  }catch(e){setText('presetDescription','프리셋을 불러오지 못했습니다.');}
}
loadPresets();

async function loadSpinnerItems(){
  try{
    const items=await (await api('/api/spinners')).json();
    for(const item of items){const option=document.createElement('option');option.value=item.id;option.textContent=item.name;$('spinnerItem').appendChild(option);}
    controls();poll();
  }catch(e){setLiteral('status',tr('스피너 목록을 불러오지 못했습니다: ')+e.message);}
}

initLanguage();
