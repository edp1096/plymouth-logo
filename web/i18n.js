// UI strings are embedded with the application; user content is not translated.
const english = {
  "연결 중": "Connecting",
  "연결 중…": "Connecting…",
  "프리셋": "Preset",
  "직접 편집": "Custom",
  "선택 후 오른쪽 미리보기에서 확인하세요": "Choose a preset to preview",
  "배경색": "Background",
  "배경색 선택": "Choose background color",
  "배경색 HEX 코드": "Background HEX code",
  "현재 적용": "Applied",
  "확인 중": "Loading",
  "현재 적용된 로고": "Applied logo",
  "현재 설정을 읽는 중…": "Reading current settings…",
  "현재 스피너": "Applied spinner",
  "변경 미리보기": "Preview",
  "적용 전": "Not applied",
  "변경할 로고 미리보기": "Logo preview",
  "변경할 스피너": "Spinner preview",
  "Tux 로고 · 회전 표시 유지": "Tux logo · animated spinner",
  "1920 × 1080 기준 미리보기": "Preview at 1920 × 1080",
  "이미지 선택": "Choose image",
  "PNG · JPG / 최대 12 MB": "PNG · JPG / up to 12 MB",
  "현재 로고로 시작": "Start with the current logo",
  "변경할 로고 크기": "Logo size",
  "로고 좌우": "Logo X",
  "로고 상하": "Logo Y",
  "스피너": "Spinner",
  "기본 회전": "Default ring",
  "스피너 크기": "Spinner size",
  "스피너 좌우": "Spinner X",
  "스피너 상하": "Spinner Y",
  "로고 적용": "Apply",
  "직전 복원": "Undo last apply",
  "원본 복원": "Restore original",
  "관리자 인증 후 적용 · 재부팅 시 반영": "Authentication required · shown after reboot",
  "일반 부팅 화면만 변경합니다. 커널 시험용 슬롯과 U-Boot 로고는 유지됩니다.": "Changes the normal boot screen. Kernel trial slots and the U-Boot logo are preserved.",
  "요청 실패": "Request failed",
  "#과 6자리 HEX 값을 입력하세요. 예: #101827": "Enter # followed by six HEX digits, e.g. #101827",
  "12 MB 이하 이미지를 선택하세요.": "Choose an image no larger than 12 MB.",
  "최초 변경 전 백업으로 원본 부팅 화면을 복원할까요?": "Restore the original boot screen from the backup before the first change?",
  "마지막 적용 직전의 부팅 화면으로 복원할까요?": "Restore the boot screen from before the last apply?",
  "작업 요청 중…": "Requesting operation…",
  "기본 시스템 테마 · 사용자 로고 없음": "System theme · no custom logo",
  "기본 테마": "System theme",
  "기본 Tux · 아직 적용되지 않은 미리보기": "Default Tux · not applied",
  "프리셋을 불러오지 못했습니다.": "Could not load presets.",
  "스피너 목록을 불러오지 못했습니다: ": "Could not load spinners: ",
  "기본 Tux + 스피너": "Default Tux and spinner",
  "작은 로고와 가까운 스피너": "Small logo with spinner nearby",
  "큰 로고와 하단 스피너": "Large logo with spinner below",
  "짙은 남색 배경": "Dark navy background",
  "차분한 회색 배경": "Muted gray background",
  "언어": "Language",
  "언어 설정을 저장하지 못했습니다.": "Could not save language preference."
};
let language = typeof navigator !== 'undefined' && navigator.language.toLowerCase().startsWith('ko') ? 'ko' : 'en';
const localizedText = new Map();
function tr(text) {
 if(language==='ko') return ({'Ready':'준비됨','Preparing…':'준비 중…','Waiting for administrator authentication, then processing…':'관리자 인증을 기다리는 중입니다. 인증 후 작업합니다.'})[text] || text;
 return english[text] || text;
}
function setText(id,text) { localizedText.set(id,text); const el=document.getElementById(id);el.removeAttribute?.('data-i18n');el.textContent=tr(text); }
function setLiteral(id,text) { localizedText.delete(id); const el=document.getElementById(id);el.removeAttribute?.('data-i18n');el.textContent=text; }
function applyLanguage(value) {
 language=value==='ko'?'ko':'en';
 document.documentElement.lang=language;
 document.querySelectorAll('[data-i18n]').forEach(el=>el.textContent=tr(el.dataset.i18n));
 for(const attr of ['alt','aria-label']) document.querySelectorAll('[data-i18n-'+attr+']').forEach(el=>el.setAttribute(attr,tr(el.getAttribute('data-i18n-'+attr))));
 localizedText.forEach((text,id)=>document.getElementById(id).textContent=tr(text));
 document.getElementById('language').value=language;
 if(typeof validBackground!=='undefined' && !validBackground) document.getElementById('backgroundHex').title=tr('#과 6자리 HEX 값을 입력하세요. 예: #101827');
}
async function initLanguage() {
 const select=document.getElementById('language');select.disabled=true;
 applyLanguage(language);
 try { const pref=await (await api('/api/preferences')).json(); if(pref.language) applyLanguage(pref.language); } catch (_) {}
 select.disabled=false;
 select.onchange=async()=>{
  applyLanguage(select.value);select.disabled=true;
  try { await api('/api/preferences',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({language})}); }
  catch (_) {setText('status','언어 설정을 저장하지 못했습니다.');}
  finally {select.disabled=false;}
 };
}
