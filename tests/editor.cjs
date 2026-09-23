const assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const elements=new Map();
const el=id=>{if(!elements.has(id))elements.set(id,{style:{},dataset:{},value:'320',files:[],children:[],replaceChildren(){this.children=[];},appendChild(child){this.children.push(child);},removeChild(child){this.children.splice(this.children.indexOf(child),1);},getAttribute(name){return this[name];},click(){return this.onclick?.();}});return elements.get(id);};
let source=0;
const ctx=vm.createContext({document:{getElementById:el,createElement(){return {style:{},getAttribute(name){return this[name];}};},documentElement:{lang:'ko'},querySelectorAll(){return [];}},navigator:{language:'ko'},location:{hash:'#test'},sessionStorage:{getItem(){},setItem(){}},history:{replaceState(){}},URL:{createObjectURL(){return 'blob:'+source++;},revokeObjectURL(){}},fetch:()=>new Promise(()=>{}),setTimeout(){},clearTimeout(){},confirm:()=>true,FileReader:class{readAsDataURL(blob){this.result='data:image/png;base64,'+(blob?.encoded||'c291cmNl');this.onload();}}});
for(const file of ['i18n','animations','app'])vm.runInContext(fs.readFileSync('web/'+file+'.js','utf8'),ctx);
const run=code=>vm.runInContext(code,ctx);
(async()=>{
 await run(`
 animationItems=[{id:'default',name:'Default',width:32,height:32,frames:30,fps:15},...['speaki','pepe'].map(id=>({id,name:id,width:160,height:160,frames:60,fps:30}))];
 let serverState={prepared:true,revision:1,size:640,applied:true,busy:false,ok:true,logo_position:{x:23,y:41},animations:[{item:'speaki',size:160,fps:12,position:{x:80,y:70}}]};
 let systemTheme={available:true,name:'bgrt',top:'#000000',bottom:'#000000',note:'reference',layers:[{image:'data:image/png;base64,test',width:128,height:128,x:50,y:38.2,center:true}]};
 const mockAPI=async(path,options)=>({json:async()=>path==='/api/inspect-image'?{width:640,height:320,preview:'cHJldmlldw=='}:path.startsWith('/api/theme-preview')?systemTheme:serverState,blob:async()=>({encoded:'c291cmNl'})});
 api=mockAPI;poll();`);
 assert.equal(run('sceneReady'),true);
 assert.equal(run('draftAnimations.length'),2);
 assert.equal(run('draftAnimations[0].kind'),'image','legacy logo must become first image layer');
 assert.equal(run('draftAnimations[0].size'),640);
 assert.equal(run('draftAnimations[0].position.x'),23);
 assert.equal(run('draftAnimations[1].fps'),12);
 assert.equal(el('fpsControl').hidden,true);
 assert.equal(el('spinnerSize').max,1920);
 assert.equal(el('draftLayers').children[0].style.height,(320/1080*100)+'%');
 run("$('spinnerSize').value='960';$('spinnerX').value='10';$('spinnerY').value='90';editActiveAnimation()");
 const draft=run('JSON.stringify(draftAnimations)');
 await run('serverState={...serverState,revision:2,size:128,logo_position:{x:70,y:80}};poll()');
 assert.equal(run('JSON.stringify(draftAnimations)'),draft,'poll must preserve draft sources and geometry');
 assert.equal(el('currentLogo').style.left,'70%');
 run("applyLanguage('en')");assert.equal(run("tr('이미지 추가')"),'Add image');assert.equal(run('JSON.stringify(draftAnimations)'),draft);
 run("applyLanguage('ko');selectAnimation(1)");assert.equal(el('fpsControl').hidden,false);assert.equal(el('spinnerSize').max,320);
 run("$('animationFPS').value='7';$('spinnerX').value='90';editActiveAnimation();$('animationBack').click()");
 assert.equal(run('draftAnimations[0].kind'),'animation');assert.equal(run('draftAnimations[0].fps'),7);assert.equal(run('draftAnimations[1].size'),960);
 assert.equal(el('draftLayers').children[1].style.zIndex,'3','image may render in front of animation');
 run("$('addAnimation').click();$('spinnerItem').value='speaki';$('spinnerItem').onchange();$('animationFPS').value='21';editActiveAnimation()");
 assert.equal(run('draftAnimations[0].fps'),7);assert.equal(run('draftAnimations[2].fps'),21,'duplicate timing must remain independent');
 run("$('backgroundHex').value='#12';$('backgroundHex').oninput()");assert.equal(el('apply').disabled,true);
 await run('serverState={...serverState,revision:3};poll()');assert.equal(el('backgroundHex').value,'#12');
 run("$('backgroundColor').value='#102030';$('backgroundColor').oninput()");assert.equal(el('apply').disabled,false);
 const beforeDefault=run('JSON.stringify(draftAnimations)');
 await run("previewSource='default';showSelectedTheme()");assert.equal(el('currentSize').textContent,'bgrt');assert.equal(el('systemLayers').children.length,1);
 assert.equal(run('JSON.stringify(draftAnimations)'),beforeDefault);
 await run("previewSource='current';showSelectedTheme()");assert.equal(el('currentLogo').style.display,'block');
 run("let finishDefault;api=()=>new Promise(resolve=>{finishDefault=resolve;});previewSource='default';let pending=showSelectedTheme()");
 await run("previewSource='current';showSelectedTheme()");await run('finishDefault({json:async()=>systemTheme});pending');assert.equal(el('systemLayers').children.length,0);
 // New installed scenes have one unified preview and preserve image originals on reload.
 await run(`api=mockAPI;dirty=false;serverState={...serverState,revision:4,layers:[{kind:'image',name:'second.png',size:400,width:400,height:200,source_width:640,source_height:320,position:{x:12,y:87},frames:1,fps:1},{kind:'animation',item:'speaki',size:160,width:160,height:160,frames:60,fps:9,position:{x:60,y:40}}]};poll()`);
 assert.equal(el('currentLogo').style.display,'none');assert.equal(el('currentLayers').children.length,2);
 assert.match(el('currentLayers').children[0].previewSource,/scene-layer.png/);
 assert.equal(run('draftAnimations[0].image'),'c291cmNl');assert.equal(run('draftAnimations[0].name'),'second.png');assert.equal(run('draftAnimations[0].size'),400);
 // Upload appends, replace preserves geometry, and malformed input leaves all layers untouched.
 el('file').files=[{name:'third.png',encoded:'dGhpcmQ=',size:10}];await run("$('file').onchange()");assert.equal(run('draftAnimations.length'),3);assert.equal(run('draftAnimations[2].name'),'third.png');
 run("$('spinnerSize').value='600';editActiveAnimation()");el('replaceFile').files=[{name:'replacement.jpg',encoded:'bmV3',size:10}];await run("$('replaceFile').onchange()");assert.equal(run('draftAnimations[2].size'),600);assert.equal(run('draftAnimations[2].image'),'bmV3');
 const valid=run('JSON.stringify(draftAnimations)');el('file').files=[{name:'bad.png',size:13*1024*1024}];await run("$('file').onchange()");assert.equal(run('JSON.stringify(draftAnimations)'),valid);
 // A delayed upload cannot overwrite a newer edit.
 run("let finishInspect;api=async()=>({json:()=>new Promise(resolve=>{finishInspect=resolve;})})");el('file').files=[{name:'slow.png',encoded:'c2xvdw==',size:10}];const uploading=run("$('file').onchange()");
 await new Promise(resolve=>setImmediate(resolve));assert.equal(el('apply').disabled,true);
 run("$('backgroundColor').value='#abcdef';$('backgroundColor').oninput();finishInspect({width:640,height:320,preview:'cHJldmlldw=='})");await uploading;assert.equal(run('JSON.stringify(draftAnimations)'),valid);
 await run('api=mockAPI;serverState={...serverState,host_error:"this tool is for Orange Pi 5 Plus"};poll()');assert.equal(el('apply').disabled,true);assert.equal(el('spinnerSize').disabled,false);
 run('let calls=0;api=async()=>{calls++}');await run("start('apply')");assert.equal(run('calls'),0);
 await run('api=mockAPI;serverState={...serverState,host_error:""};poll()');assert.equal(el('apply').disabled,false);
 await run('api=async()=>{throw Error("offline")};poll()');assert.equal(el('apply').disabled,true);
 // Animation capacity is preserved; all static images have a separate bounded budget.
 run("loadDraftAnimations([...Array.from({length:6},()=>({...newAnimation(),size:320})),{kind:'image',image:'c291cmNl',preview:'data:image/png;base64,cA==',size:1920,source_width:1920,source_height:1080,position:{x:50,y:50}}])");assert.match(run('animationIssue'),/64 MiB/);assert.equal(el('draftLayers').children.length,0);
 run("loadDraftAnimations([]);hostReady=true;busy=false;let submitted;api=async(path,options)=>{submitted=JSON.parse(options.body)}");await run("start('apply')");assert.deepEqual(JSON.parse(run('JSON.stringify(submitted.layers)')),[],'empty scene is explicit, not legacy');
 run("busy=false;for(let i=0;i<9;i++)$('addAnimation').click()");assert.equal(run('draftAnimations.length'),8);assert.equal(el('addAnimation').disabled,true);assert.equal(el('addImage').disabled,false);
 run("loadDraftAnimations([{kind:'image',name:'saved.png',image:'c291cmNl',preview:'data:image/png;base64,cA==',size:160,source_width:640,source_height:320,position:{x:50,y:50}},newAnimation()])");await run("start('apply')");
 assert.equal(run('submitted.layers[0].image'),'c291cmNl');assert.equal(run('submitted.layers[0].preview'),undefined);assert.equal(run('submitted.layers[1].kind'),'animation');
 console.log('Editor checks passed: migration, mixed ordering, originals, upload/replace races, independent previews, memory limits, host guards, empty scenes, and apply payload.');
})().catch(err=>{console.error(err);process.exitCode=1;});
