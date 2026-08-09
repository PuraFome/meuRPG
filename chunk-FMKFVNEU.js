import{a as Ce,b as Me,c as we}from"./chunk-BLBGU42S.js";import{a as ee}from"./chunk-B7UIUJFO.js";import{a as Oe}from"./chunk-JZYZ6NTX.js";import{a as E}from"./chunk-2ELLHHL2.js";import{a as ve}from"./chunk-GCTOM2WE.js";import{a as xe}from"./chunk-QSRZ3ENZ.js";import{a as Pe}from"./chunk-U2IQIRUJ.js";import"./chunk-7RUXH4GA.js";import"./chunk-RBFJTBQV.js";import{A as be,H as he,I as ye,x as fe}from"./chunk-M47ASA6Q.js";import{Ha as D,Ia as u,Jb as W,Ka as g,L as ie,M as re,Mb as Y,Na as L,Nb as j,O as y,Oa as A,Pa as b,Q as f,Qa as a,Qb as pe,R as _,Ra as o,Sa as v,Sb as J,Sc as ue,Ta as z,Ua as F,Va as U,X as I,Xa as h,Y as ae,Z as c,_ as oe,_a as p,_c as ge,ab as d,ac as S,ba as le,eb as V,fb as R,gb as B,gd as M,ha as G,hd as w,jd as X,ka as se,kd as _e,la as l,lb as Z,mb as O,nb as ce,nd as P,ob as s,od as T,pb as C,qa as de,qb as k,rb as N,ua as x,va as me}from"./chunk-4UBTO7VS.js";import"./chunk-QXIBXHVB.js";function Ue(r,t){r&1&&U(0,"div",2)}var Ge=new re("MAT_PROGRESS_BAR_DEFAULT_OPTIONS");var Ie=(()=>{class r{_elementRef=y(le);_ngZone=y(ae);_changeDetectorRef=y(pe);_renderer=y(de);_cleanupTransitionEnd;constructor(){let e=ge(),n=y(Ge,{optional:!0});this._isNoopAnimation=e==="di-disabled",e==="reduced-motion"&&this._elementRef.nativeElement.classList.add("mat-progress-bar-reduced-motion"),n&&(n.color&&(this.color=this._defaultColor=n.color),this.mode=n.mode||this.mode)}_isNoopAnimation;get color(){return this._color||this._defaultColor}set color(e){this._color=e}_color;_defaultColor="primary";get value(){return this._value}set value(e){this._value=ke(e||0),this._changeDetectorRef.markForCheck()}_value=0;get bufferValue(){return this._bufferValue||0}set bufferValue(e){this._bufferValue=ke(e||0),this._changeDetectorRef.markForCheck()}_bufferValue=0;animationEnd=new I;get mode(){return this._mode}set mode(e){this._mode=e,this._changeDetectorRef.markForCheck()}_mode="determinate";ngAfterViewInit(){this._ngZone.runOutsideAngular(()=>{this._cleanupTransitionEnd=this._renderer.listen(this._elementRef.nativeElement,"transitionend",this._transitionendHandler)})}ngOnDestroy(){this._cleanupTransitionEnd?.()}_getPrimaryBarTransform(){return`scaleX(${this._isIndeterminate()?1:this.value/100})`}_getBufferBarFlexBasis(){return`${this.mode==="buffer"?this.bufferValue:100}%`}_isIndeterminate(){return this.mode==="indeterminate"||this.mode==="query"}_transitionendHandler=e=>{this.animationEnd.observers.length===0||!e.target||!e.target.classList.contains("mdc-linear-progress__primary-bar")||(this.mode==="determinate"||this.mode==="buffer")&&this._ngZone.run(()=>this.animationEnd.next({value:this.value}))};static \u0275fac=function(n){return new(n||r)};static \u0275cmp=x({type:r,selectors:[["mat-progress-bar"]],hostAttrs:["role","progressbar","aria-valuemin","0","aria-valuemax","100","tabindex","-1",1,"mat-mdc-progress-bar","mdc-linear-progress"],hostVars:10,hostBindings:function(n,i){n&2&&(D("aria-valuenow",i._isIndeterminate()?null:i.value)("mode",i.mode),ce("mat-"+i.color),O("_mat-animation-noopable",i._isNoopAnimation)("mdc-linear-progress--animation-ready",!i._isNoopAnimation)("mdc-linear-progress--indeterminate",i._isIndeterminate()))},inputs:{color:"color",value:[2,"value","value",J],bufferValue:[2,"bufferValue","bufferValue",J],mode:"mode"},outputs:{animationEnd:"animationEnd"},exportAs:["matProgressBar"],decls:7,vars:5,consts:[["aria-hidden","true",1,"mdc-linear-progress__buffer"],[1,"mdc-linear-progress__buffer-bar"],[1,"mdc-linear-progress__buffer-dots"],["aria-hidden","true",1,"mdc-linear-progress__bar","mdc-linear-progress__primary-bar"],[1,"mdc-linear-progress__bar-inner"],["aria-hidden","true",1,"mdc-linear-progress__bar","mdc-linear-progress__secondary-bar"]],template:function(n,i){n&1&&(z(0,"div",0),U(1,"div",1),u(2,Ue,1,0,"div",2),F(),z(3,"div",3),U(4,"span",4),F(),z(5,"div",5),U(6,"span",4),F()),n&2&&(l(),Z("flex-basis",i._getBufferBarFlexBasis()),l(),g(i.mode==="buffer"?2:-1),l(),Z("transform",i._getPrimaryBarTransform()))},styles:[`.mat-mdc-progress-bar {
  --mat-progress-bar-animation-multiplier: 1;
  display: block;
  text-align: start;
}
.mat-mdc-progress-bar[mode=query] {
  transform: scaleX(-1);
}
.mat-mdc-progress-bar._mat-animation-noopable .mdc-linear-progress__buffer-dots,
.mat-mdc-progress-bar._mat-animation-noopable .mdc-linear-progress__primary-bar,
.mat-mdc-progress-bar._mat-animation-noopable .mdc-linear-progress__secondary-bar,
.mat-mdc-progress-bar._mat-animation-noopable .mdc-linear-progress__bar-inner.mdc-linear-progress__bar-inner {
  animation: none;
}
.mat-mdc-progress-bar._mat-animation-noopable .mdc-linear-progress__primary-bar,
.mat-mdc-progress-bar._mat-animation-noopable .mdc-linear-progress__buffer-bar {
  transition: transform 1ms;
}

.mat-progress-bar-reduced-motion {
  --mat-progress-bar-animation-multiplier: 2;
}

.mdc-linear-progress {
  position: relative;
  width: 100%;
  transform: translateZ(0);
  outline: 1px solid transparent;
  overflow-x: hidden;
  transition: opacity 250ms 0ms cubic-bezier(0.4, 0, 0.6, 1);
  height: max(var(--mat-progress-bar-track-height, 4px), var(--mat-progress-bar-active-indicator-height, 4px));
}
@media (forced-colors: active) {
  .mdc-linear-progress {
    outline-color: CanvasText;
  }
}

.mdc-linear-progress__bar {
  position: absolute;
  top: 0;
  bottom: 0;
  margin: auto 0;
  width: 100%;
  animation: none;
  transform-origin: top left;
  transition: transform 250ms 0ms cubic-bezier(0.4, 0, 0.6, 1);
  height: var(--mat-progress-bar-active-indicator-height, 4px);
}
.mdc-linear-progress--indeterminate .mdc-linear-progress__bar {
  transition: none;
}
[dir=rtl] .mdc-linear-progress__bar {
  right: 0;
  transform-origin: center right;
}

.mdc-linear-progress__bar-inner {
  display: inline-block;
  position: absolute;
  width: 100%;
  animation: none;
  border-top-style: solid;
  border-color: var(--mat-progress-bar-active-indicator-color, var(--mat-sys-primary));
  border-top-width: var(--mat-progress-bar-active-indicator-height, 4px);
}

.mdc-linear-progress__buffer {
  display: flex;
  position: absolute;
  top: 0;
  bottom: 0;
  margin: auto 0;
  width: 100%;
  overflow: hidden;
  height: var(--mat-progress-bar-track-height, 4px);
  border-radius: var(--mat-progress-bar-track-shape, var(--mat-sys-corner-none));
}

.mdc-linear-progress__buffer-dots {
  background-image: radial-gradient(circle, var(--mat-progress-bar-track-color, var(--mat-sys-surface-variant)) calc(var(--mat-progress-bar-track-height, 4px) / 2), transparent 0);
  background-repeat: repeat-x;
  background-size: calc(calc(var(--mat-progress-bar-track-height, 4px) / 2) * 5);
  background-position: left;
  flex: auto;
  transform: rotate(180deg);
  animation: mdc-linear-progress-buffering calc(250ms * var(--mat-progress-bar-animation-multiplier)) infinite linear;
}
@media (forced-colors: active) {
  .mdc-linear-progress__buffer-dots {
    background-color: ButtonBorder;
  }
}
[dir=rtl] .mdc-linear-progress__buffer-dots {
  animation: mdc-linear-progress-buffering-reverse calc(250ms * var(--mat-progress-bar-animation-multiplier)) infinite linear;
  transform: rotate(0);
}

.mdc-linear-progress__buffer-bar {
  flex: 0 1 100%;
  transition: flex-basis 250ms 0ms cubic-bezier(0.4, 0, 0.6, 1);
  background-color: var(--mat-progress-bar-track-color, var(--mat-sys-surface-variant));
}

.mdc-linear-progress__primary-bar {
  transform: scaleX(0);
}
.mdc-linear-progress--indeterminate .mdc-linear-progress__primary-bar {
  left: -145.166611%;
}
.mdc-linear-progress--indeterminate.mdc-linear-progress--animation-ready .mdc-linear-progress__primary-bar {
  animation: mdc-linear-progress-primary-indeterminate-translate calc(2s * var(--mat-progress-bar-animation-multiplier)) infinite linear;
}
.mdc-linear-progress--indeterminate.mdc-linear-progress--animation-ready .mdc-linear-progress__primary-bar > .mdc-linear-progress__bar-inner {
  animation: mdc-linear-progress-primary-indeterminate-scale calc(2s * var(--mat-progress-bar-animation-multiplier)) infinite linear;
}
[dir=rtl] .mdc-linear-progress.mdc-linear-progress--animation-ready .mdc-linear-progress__primary-bar {
  animation-name: mdc-linear-progress-primary-indeterminate-translate-reverse;
}
[dir=rtl] .mdc-linear-progress.mdc-linear-progress--indeterminate .mdc-linear-progress__primary-bar {
  right: -145.166611%;
  left: auto;
}

.mdc-linear-progress__secondary-bar {
  display: none;
}
.mdc-linear-progress--indeterminate .mdc-linear-progress__secondary-bar {
  left: -54.888891%;
  display: block;
}
.mdc-linear-progress--indeterminate.mdc-linear-progress--animation-ready .mdc-linear-progress__secondary-bar {
  animation: mdc-linear-progress-secondary-indeterminate-translate calc(2s * var(--mat-progress-bar-animation-multiplier)) infinite linear;
}
.mdc-linear-progress--indeterminate.mdc-linear-progress--animation-ready .mdc-linear-progress__secondary-bar > .mdc-linear-progress__bar-inner {
  animation: mdc-linear-progress-secondary-indeterminate-scale calc(2s * var(--mat-progress-bar-animation-multiplier)) infinite linear;
}
[dir=rtl] .mdc-linear-progress.mdc-linear-progress--animation-ready .mdc-linear-progress__secondary-bar {
  animation-name: mdc-linear-progress-secondary-indeterminate-translate-reverse;
}
[dir=rtl] .mdc-linear-progress.mdc-linear-progress--indeterminate .mdc-linear-progress__secondary-bar {
  right: -54.888891%;
  left: auto;
}

@keyframes mdc-linear-progress-buffering {
  from {
    transform: rotate(180deg) translateX(calc(var(--mat-progress-bar-track-height, 4px) * -2.5));
  }
}
@keyframes mdc-linear-progress-primary-indeterminate-translate {
  0% {
    transform: translateX(0);
  }
  20% {
    animation-timing-function: cubic-bezier(0.5, 0, 0.701732, 0.495819);
    transform: translateX(0);
  }
  59.15% {
    animation-timing-function: cubic-bezier(0.302435, 0.381352, 0.55, 0.956352);
    transform: translateX(83.67142%);
  }
  100% {
    transform: translateX(200.611057%);
  }
}
@keyframes mdc-linear-progress-primary-indeterminate-scale {
  0% {
    transform: scaleX(0.08);
  }
  36.65% {
    animation-timing-function: cubic-bezier(0.334731, 0.12482, 0.785844, 1);
    transform: scaleX(0.08);
  }
  69.15% {
    animation-timing-function: cubic-bezier(0.06, 0.11, 0.6, 1);
    transform: scaleX(0.661479);
  }
  100% {
    transform: scaleX(0.08);
  }
}
@keyframes mdc-linear-progress-secondary-indeterminate-translate {
  0% {
    animation-timing-function: cubic-bezier(0.15, 0, 0.515058, 0.409685);
    transform: translateX(0);
  }
  25% {
    animation-timing-function: cubic-bezier(0.31033, 0.284058, 0.8, 0.733712);
    transform: translateX(37.651913%);
  }
  48.35% {
    animation-timing-function: cubic-bezier(0.4, 0.627035, 0.6, 0.902026);
    transform: translateX(84.386165%);
  }
  100% {
    transform: translateX(160.277782%);
  }
}
@keyframes mdc-linear-progress-secondary-indeterminate-scale {
  0% {
    animation-timing-function: cubic-bezier(0.205028, 0.057051, 0.57661, 0.453971);
    transform: scaleX(0.08);
  }
  19.15% {
    animation-timing-function: cubic-bezier(0.152313, 0.196432, 0.648374, 1.004315);
    transform: scaleX(0.457104);
  }
  44.15% {
    animation-timing-function: cubic-bezier(0.257759, -0.003163, 0.211762, 1.38179);
    transform: scaleX(0.72796);
  }
  100% {
    transform: scaleX(0.08);
  }
}
@keyframes mdc-linear-progress-primary-indeterminate-translate-reverse {
  0% {
    transform: translateX(0);
  }
  20% {
    animation-timing-function: cubic-bezier(0.5, 0, 0.701732, 0.495819);
    transform: translateX(0);
  }
  59.15% {
    animation-timing-function: cubic-bezier(0.302435, 0.381352, 0.55, 0.956352);
    transform: translateX(-83.67142%);
  }
  100% {
    transform: translateX(-200.611057%);
  }
}
@keyframes mdc-linear-progress-secondary-indeterminate-translate-reverse {
  0% {
    animation-timing-function: cubic-bezier(0.15, 0, 0.515058, 0.409685);
    transform: translateX(0);
  }
  25% {
    animation-timing-function: cubic-bezier(0.31033, 0.284058, 0.8, 0.733712);
    transform: translateX(-37.651913%);
  }
  48.35% {
    animation-timing-function: cubic-bezier(0.4, 0.627035, 0.6, 0.902026);
    transform: translateX(-84.386165%);
  }
  100% {
    transform: translateX(-160.277782%);
  }
}
@keyframes mdc-linear-progress-buffering-reverse {
  from {
    transform: translateX(-10px);
  }
}
`],encapsulation:2,changeDetection:0})}return r})();function ke(r,t=0,e=100){return Math.max(t,Math.min(e,r))}var Se=(()=>{class r{static \u0275fac=function(n){return new(n||r)};static \u0275mod=me({type:r});static \u0275inj=ie({imports:[ue]})}return r})();var Le=["fileUpload"];function Ae(r,t){if(r&1){let e=h();a(0,"div",3)(1,"span",5)(2,"mat-icon",6),s(3),o(),s(4),a(5,"span",7),s(6),o()(),a(7,"div",8)(8,"button",9),p("click",function(){f(e);let i=d();return _(i.cancelUpload())}),a(9,"mat-icon"),s(10,"close"),o(),s(11," Cancelar "),o(),a(12,"button",10),p("click",function(){f(e);let i=d();return _(i.confirmUpload())}),a(13,"mat-icon"),s(14,"cloud_upload"),o(),s(15," Fazer Upload "),o()()()}if(r&2){let e=d();l(3),k(" ",e.selectedFile().type.startsWith("image/")?"image":"audiotrack"," "),l(),k(" ",e.selectedFile().name," "),l(2),k("(",e.formatSize(e.selectedFile().size),")")}}function ze(r,t){r&1&&(a(0,"p",15),s(1,"Upload conclu\xEDdo!"),o())}function Fe(r,t){if(r&1&&(a(0,"div",4)(1,"div",11)(2,"span",12),s(3,"Enviando\u2026"),o(),a(4,"span",13),s(5),o()(),v(6,"mat-progress-bar",14),u(7,ze,2,0,"p",15),o()),r&2){let e=d();l(5),k("",e.uploadProgress(),"%"),l(),b("value",e.uploadProgress()),l(),g(e.uploadProgress()===100?7:-1)}}var H=class r{store=y(T);fileRepo=new E;uploadComplete=new I;fileUpload;acceptedTypes=["image/png","image/jpeg","image/gif","image/webp","image/svg+xml","audio/mpeg","audio/ogg","audio/wav"];maxSize=10*1024*1024;selectedFile=c(null);isUploading=c(!1);uploadProgress=c(0);previewUrl=c(null);simulatedTimer=null;onFileSelected(t){let e=this.previewUrl();e&&URL.revokeObjectURL(e),this.selectedFile.set(t),t.type.startsWith("image/")?this.previewUrl.set(URL.createObjectURL(t)):this.previewUrl.set(null)}async confirmUpload(){let t=this.selectedFile();if(!t)return;this.isUploading.set(!0),this.uploadProgress.set(0),this.simulatedTimer=setInterval(()=>{this.uploadProgress.update(m=>{if(m>=95)return 95;let K=m<50?15:m<80?8:3;return Math.min(m+K+Math.random()*5,95)})},250);let e=crypto.randomUUID(),n=t.type.startsWith("image/")?"image":"audio";await this.fileRepo.save({id:e,name:t.name,data:t,mimeType:t.type,size:t.size,uploadedAt:new Date});let i={id:e,name:t.name,type:n,url:e,thumbnailUrl:void 0,folderId:null,size:t.size,createdAt:new Date};this.store.set("gallery",i),this.simulatedTimer&&(clearInterval(this.simulatedTimer),this.simulatedTimer=null),this.uploadProgress.set(100),await new Promise(m=>setTimeout(m,600)),this.resetUpload(),this.uploadComplete.emit()}cancelUpload(){this.resetUpload()}resetUpload(){let t=this.previewUrl();t&&URL.revokeObjectURL(t),this.selectedFile.set(null),this.previewUrl.set(null),this.isUploading.set(!1),this.uploadProgress.set(0),this.fileUpload?.clearFile(),this.simulatedTimer&&(clearInterval(this.simulatedTimer),this.simulatedTimer=null)}formatSize(t){return t<1024?`${t} B`:t<1024*1024?`${(t/1024).toFixed(1)} KB`:`${(t/(1024*1024)).toFixed(1)} MB`}static \u0275fac=function(e){return new(e||r)};static \u0275cmp=x({type:r,selectors:[["app-gallery-upload"]],viewQuery:function(e,n){if(e&1&&V(Le,5),e&2){let i;R(i=B())&&(n.fileUpload=i.first)}},outputs:{uploadComplete:"uploadComplete"},decls:5,vars:4,consts:[["fileUpload",""],[1,"upload-section"],[3,"fileChange","acceptedTypes","maxSize"],[1,"upload-actions"],[1,"progress-section"],[1,"file-name"],[1,"file-icon"],[1,"file-size"],[1,"action-buttons"],["mat-stroked-button","","color","warn",3,"click"],["mat-raised-button","","color","primary",3,"click"],[1,"progress-header"],[1,"progress-label"],[1,"progress-value"],["mode","determinate",1,"upload-progress",3,"value"],[1,"progress-done"]],template:function(e,n){e&1&&(a(0,"div",1)(1,"app-file-upload",2,0),p("fileChange",function(m){return n.onFileSelected(m)}),o(),u(3,Ae,16,3,"div",3),u(4,Fe,8,3,"div",4),o()),e&2&&(l(),b("acceptedTypes",n.acceptedTypes)("maxSize",n.maxSize),l(2),g(n.selectedFile()&&!n.isUploading()?3:-1),l(),g(n.isUploading()?4:-1))},dependencies:[Oe,Se,Ie,P,_e,w,M],styles:["[_nghost-%COMP%]{display:block}.upload-section[_ngcontent-%COMP%]{padding:24px;border-radius:12px;background:#ffffff08;border:1px solid rgba(255,255,255,.06)}.upload-actions[_ngcontent-%COMP%]{display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:12px;margin-top:16px;padding:12px 16px;border-radius:8px;background:#ffffff0a}.file-name[_ngcontent-%COMP%]{display:flex;align-items:center;gap:8px;font-size:.9rem;opacity:.85}.file-icon[_ngcontent-%COMP%]{font-size:1.2rem;width:1.2rem;height:1.2rem;opacity:.6}.file-size[_ngcontent-%COMP%]{opacity:.45;font-size:.8rem}.action-buttons[_ngcontent-%COMP%]{display:flex;gap:8px}.progress-section[_ngcontent-%COMP%]{margin-top:16px;padding:16px;border-radius:8px;background:#ffffff0a}.progress-header[_ngcontent-%COMP%]{display:flex;justify-content:space-between;align-items:center;margin-bottom:8px}.progress-label[_ngcontent-%COMP%]{font-size:.85rem;opacity:.7}.progress-value[_ngcontent-%COMP%]{font-size:.85rem;font-weight:500;opacity:.8}.upload-progress[_ngcontent-%COMP%]{border-radius:4px}.progress-done[_ngcontent-%COMP%]{color:#66bb6a;font-size:.85rem;margin:8px 0 0}"]})};var Ve=(r,t)=>t.id;function Re(r,t){r&1&&v(0,"app-loading-spinner",8),r&2&&b("isLoading",!0)}function Be(r,t){r&1&&v(0,"app-empty-state",9)}function Ne(r,t){if(r&1&&v(0,"img",18),r&2){let e=d(2).$implicit;b("src",t,G)("alt",e.name)}}function je(r,t){r&1&&(a(0,"div",19)(1,"mat-icon",20),s(2,"image"),o()())}function Xe(r,t){if(r&1&&u(0,Ne,1,2,"img",18)(1,je,3,0,"div",19),r&2){let e,n=d().$implicit,i=d(2);g((e=i.getThumbnail(n.id))?0:1,e)}}function $e(r,t){r&1&&(a(0,"div",14)(1,"mat-icon",21),s(2,"audiotrack"),o(),a(3,"div",22),v(4,"span")(5,"span")(6,"span")(7,"span")(8,"span"),o()())}function He(r,t){if(r&1){let e=h();a(0,"button",12),p("click",function(){let i=f(e).$implicit,m=d(2);return _(m.openLightbox(i))}),a(1,"div",13),u(2,Xe,2,1)(3,$e,9,0,"div",14),o(),a(4,"div",15)(5,"span",16),s(6),o(),a(7,"span",17),s(8),o()()()}if(r&2){let e=t.$implicit,n=d(2);O("is-audio",e.type==="audio"),l(2),g(e.type==="image"?2:3),l(3),b("title",e.name),l(),C(e.name),l(2),N("",n.formatSize(e.size)," \u2022 ",n.formatDate(e.createdAt))}}function Qe(r,t){if(r&1&&(a(0,"div",10),L(1,He,9,7,"button",11,Ve),o()),r&2){let e=d();l(),A(e.sortedItems())}}var Q=class r{store=y(T);fileRepo=new E;lightboxOpen=new I;loading=c(!0);sortField=c("date");sortedItems=c([]);items=[];thumbnailUrls=new Map;subscription=null;ngOnInit(){this.subscription=this.store.getAll("gallery").subscribe(async t=>{this.items=t,await this.loadThumbnails(t),this.applySort(),this.loading.set(!1)})}ngOnDestroy(){this.subscription?.unsubscribe();for(let t of this.thumbnailUrls.values())URL.revokeObjectURL(t);this.thumbnailUrls.clear()}getThumbnail(t){return this.thumbnailUrls.get(t)}onSortChange(){this.applySort()}openLightbox(t){this.lightboxOpen.emit(t)}async loadThumbnails(t){for(let i of this.thumbnailUrls.values())URL.revokeObjectURL(i);this.thumbnailUrls.clear();let e=t.filter(i=>i.type==="image"),n=await Promise.allSettled(e.map(async i=>{let m=await this.fileRepo.get(i.id);if(m){let K=URL.createObjectURL(m.data);return{id:i.id,url:K}}return null}));for(let i of n)i.status==="fulfilled"&&i.value&&this.thumbnailUrls.set(i.value.id,i.value.url)}applySort(){let t=[...this.items];switch(this.sortField()){case"name":t.sort((n,i)=>n.name.localeCompare(i.name));break;case"date":t.sort((n,i)=>new Date(i.createdAt).getTime()-new Date(n.createdAt).getTime());break;case"type":t.sort((n,i)=>n.type!==i.type?n.type.localeCompare(i.type):n.name.localeCompare(i.name));break}this.sortedItems.set(t)}formatSize(t){return t<1024?`${t} B`:t<1024*1024?`${(t/1024).toFixed(1)} KB`:`${(t/(1024*1024)).toFixed(1)} MB`}formatDate(t){return new Date(t).toLocaleDateString("pt-BR",{day:"2-digit",month:"short",year:"numeric"})}static \u0275fac=function(e){return new(e||r)};static \u0275cmp=x({type:r,selectors:[["app-gallery-grid"]],outputs:{lightboxOpen:"lightboxOpen"},decls:17,vars:2,consts:[[1,"grid-section"],[1,"grid-toolbar"],[1,"grid-title"],["appearance","fill","subscriptSizing","dynamic",1,"sort-select"],[3,"valueChange","value"],["value","date"],["value","name"],["value","type"],["message","Carregando galeria...",3,"isLoading"],["icon","collections_bookmark","message","Nenhum arquivo na galeria. Arraste ou clique acima para adicionar."],[1,"thumbnail-grid"],[1,"thumbnail-card",3,"is-audio"],[1,"thumbnail-card",3,"click"],[1,"card-preview"],[1,"audio-icon-wrapper"],[1,"card-info"],[1,"card-name",3,"title"],[1,"card-meta"],["loading","lazy",1,"thumb-img",3,"src","alt"],[1,"thumb-placeholder"],[1,"placeholder-icon"],[1,"audio-icon"],[1,"audio-wave"]],template:function(e,n){e&1&&(a(0,"div",0)(1,"div",1)(2,"h2",2),s(3,"Galeria"),o(),a(4,"mat-form-field",3)(5,"mat-label"),s(6,"Ordenar por"),o(),a(7,"mat-select",4),p("valueChange",function(m){return n.sortField.set(m),n.onSortChange()}),a(8,"mat-option",5),s(9,"Data"),o(),a(10,"mat-option",6),s(11,"Nome"),o(),a(12,"mat-option",7),s(13,"Tipo"),o()()()(),u(14,Re,1,1,"app-loading-spinner",8)(15,Be,1,0,"app-empty-state",9)(16,Qe,3,0,"div",10),o()),e&2&&(l(7),b("value",n.sortField()),l(7),g(n.loading()?14:n.sortedItems().length===0?15:16))},dependencies:[S,w,M,P,we,he,be,Me,Ce,ye,fe,xe,ve],styles:["[_nghost-%COMP%]{display:block}.grid-section[_ngcontent-%COMP%]{padding:0 24px 24px}@media(max-width:480px){.grid-section[_ngcontent-%COMP%]{padding:0 16px 16px}}.grid-toolbar[_ngcontent-%COMP%]{display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:12px;margin-bottom:20px}.grid-title[_ngcontent-%COMP%]{font-size:1.1rem;font-weight:500;margin:0;opacity:.8}.sort-select[_ngcontent-%COMP%]{width:160px}.thumbnail-grid[_ngcontent-%COMP%]{display:grid;grid-template-columns:repeat(auto-fill,minmax(200px,1fr));gap:16px}.thumbnail-card[_ngcontent-%COMP%]{all:unset;display:flex;flex-direction:column;border-radius:10px;overflow:hidden;background:#ffffff08;border:1px solid rgba(255,255,255,.06);cursor:pointer;transition:transform .2s ease,box-shadow .2s ease,border-color .2s ease}.thumbnail-card[_ngcontent-%COMP%]:hover{transform:translateY(-3px);box-shadow:0 8px 24px #00000040;border-color:#ffffff26}.thumbnail-card[_ngcontent-%COMP%]:focus-visible{outline:2px solid rgb(var(--mat-app-primary));outline-offset:2px}.card-preview[_ngcontent-%COMP%]{aspect-ratio:1;overflow:hidden;display:flex;align-items:center;justify-content:center;background:#ffffff05}.thumb-img[_ngcontent-%COMP%]{width:100%;height:100%;object-fit:cover;display:block;transition:transform .3s ease}.thumbnail-card[_ngcontent-%COMP%]:hover   .thumb-img[_ngcontent-%COMP%]{transform:scale(1.08)}.thumb-placeholder[_ngcontent-%COMP%]{display:flex;align-items:center;justify-content:center;width:100%;height:100%;opacity:.3}.placeholder-icon[_ngcontent-%COMP%]{font-size:2.5rem;width:2.5rem;height:2.5rem}.audio-icon-wrapper[_ngcontent-%COMP%]{display:flex;flex-direction:column;align-items:center;justify-content:center;gap:8px;width:100%;height:100%}.audio-icon[_ngcontent-%COMP%]{font-size:2.5rem;width:2.5rem;height:2.5rem;opacity:.5;transition:opacity .2s}.thumbnail-card[_ngcontent-%COMP%]:hover   .audio-icon[_ngcontent-%COMP%]{opacity:.8}.audio-wave[_ngcontent-%COMP%]{display:flex;align-items:center;gap:3px;height:24px}.audio-wave[_ngcontent-%COMP%]   span[_ngcontent-%COMP%]{display:block;width:3px;border-radius:2px;background:#fff6;animation:_ngcontent-%COMP%_wave 1.2s ease-in-out infinite}.audio-wave[_ngcontent-%COMP%]   span[_ngcontent-%COMP%]:nth-child(1){height:8px;animation-delay:0s}.audio-wave[_ngcontent-%COMP%]   span[_ngcontent-%COMP%]:nth-child(2){height:16px;animation-delay:.15s}.audio-wave[_ngcontent-%COMP%]   span[_ngcontent-%COMP%]:nth-child(3){height:22px;animation-delay:.3s}.audio-wave[_ngcontent-%COMP%]   span[_ngcontent-%COMP%]:nth-child(4){height:14px;animation-delay:.45s}.audio-wave[_ngcontent-%COMP%]   span[_ngcontent-%COMP%]:nth-child(5){height:10px;animation-delay:.6s}@keyframes _ngcontent-%COMP%_wave{0%,to{transform:scaleY(.5);opacity:.3}50%{transform:scaleY(1);opacity:.7}}.card-info[_ngcontent-%COMP%]{display:flex;flex-direction:column;padding:10px 12px;gap:2px;border-top:1px solid rgba(255,255,255,.04)}.card-name[_ngcontent-%COMP%]{font-size:.85rem;line-height:1.3;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;opacity:.85}.card-meta[_ngcontent-%COMP%]{font-size:.75rem;opacity:.45}"]})};var qe=(r,t)=>t.id;function Ke(r,t){if(r&1){let e=h();a(0,"button",14),p("click",function(){f(e);let i=d(2);return _(i.prev())}),a(1,"mat-icon"),s(2,"chevron_left"),o()()}}function Ze(r,t){if(r&1){let e=h();a(0,"img",15),p("load",function(){f(e);let i=d(2);return _(i.imageLoaded.set(!0))})("error",function(){f(e);let i=d(2);return _(i.imageLoaded.set(!0))}),o()}if(r&2){let e,n=d(2);O("loaded",n.imageLoaded()),b("src",n.currentImageUrl(),G)("alt",(e=n.currentItem())==null?null:e.name)}}function We(r,t){if(r&1){let e=h();a(0,"button",21),p("click",function(){let i=f(e).$index,m=d(3);return _(m.toggleAudioTrack(i))}),a(1,"mat-icon",22),s(2),o(),a(3,"span",23),s(4),o()()}if(r&2){let e=t.$implicit,n=t.$index,i=d(3);O("active",n===i.currentAudioIndex()),l(2),k(" ",n===i.currentAudioIndex()&&i.audioPlaying()?"pause":"play_arrow"," "),l(2),C(e.name)}}function Ye(r,t){if(r&1){let e=h();a(0,"div",20)(1,"app-audio-player",24),p("ended",function(){f(e);let i=d(3);return _(i.onAudioEnded())}),o()()}if(r&2){let e=d(3);l(),b("src",e.currentAudioSrc())("title",e.currentAudioTitle())}}function Je(r,t){if(r&1&&(a(0,"div",9)(1,"h3",16)(2,"mat-icon",17),s(3,"audiotrack"),o(),s(4," Faixas de \xC1udio "),o(),a(5,"div",18),L(6,We,5,4,"button",19,qe),o(),u(8,Ye,2,2,"div",20),o()),r&2){let e=d(2);l(6),A(e.audioItems()),l(2),g(e.currentAudioSrc()?8:-1)}}function et(r,t){if(r&1&&(a(0,"div",10)(1,"mat-icon",25),s(2,"insert_drive_file"),o(),a(3,"p",26),s(4),o()()),r&2){let e,n=d(2);l(4),C((e=n.currentItem())==null?null:e.name)}}function tt(r,t){if(r&1){let e=h();a(0,"button",27),p("click",function(){f(e);let i=d(2);return _(i.next())}),a(1,"mat-icon"),s(2,"chevron_right"),o()()}}function nt(r,t){if(r&1){let e=h();a(0,"div",1),p("click",function(i){f(e);let m=d();return _(m.onBackdropClick(i))}),a(1,"div",2)(2,"span",3),s(3),o(),a(4,"button",4),p("click",function(){f(e);let i=d();return _(i.close.emit())}),a(5,"mat-icon"),s(6,"close"),o()()(),a(7,"div",5),u(8,Ke,3,0,"button",6),a(9,"div",7),u(10,Ze,1,4,"img",8)(11,Je,9,1,"div",9)(12,et,5,1,"div",10),o(),u(13,tt,3,0,"button",11),o(),a(14,"div",12)(15,"span",13),s(16),o()()()}if(r&2){let e,n,i=d();l(3),C((e=i.currentItem())==null?null:e.name),l(5),g(i.items().length>1?8:-1),l(2),g(((n=i.currentItem())==null?null:n.type)==="image"?10:((n=i.currentItem())==null?null:n.type)==="audio"?11:12),l(3),g(i.items().length>1?13:-1),l(3),N("",i.currentIndex()+1," / ",i.items().length)}}var q=class r{visible=j(!1);items=j([]);currentIndex=j(0);close=Y();indexChange=Y();audioPlayer;fileRepo=new E;blobUrls=new Map;currentItem=W(()=>{let t=this.items(),e=this.currentIndex();return t[e]??null});audioItems=W(()=>this.items().filter(t=>t.type==="audio"));currentImageUrl=c("");imageLoaded=c(!1);currentAudioIndex=c(-1);currentAudioSrc=c("");currentAudioTitle=c("");audioPlaying=c(!1);constructor(){oe(()=>{this.visible()&&this.currentItem()&&this.loadCurrentItem()})}ngOnDestroy(){this.revokeAllBlobUrls()}prev(){let t=this.items().length;t<=1||this.navigateTo((this.currentIndex()-1+t)%t)}next(){let t=this.items().length;t<=1||this.navigateTo((this.currentIndex()+1)%t)}navigateTo(t){t!==this.currentIndex()&&(this.imageLoaded.set(!1),this.resetAudio(),this.indexChange.emit(t))}onKeydown(t){if(this.visible()&&!(t.target instanceof HTMLInputElement||t.target instanceof HTMLTextAreaElement))switch(t.key){case"Escape":t.preventDefault(),this.close.emit();break;case"ArrowLeft":t.preventDefault(),this.prev();break;case"ArrowRight":t.preventDefault(),this.next();break}}onBackdropClick(t){t.target.classList.contains("lightbox-overlay")&&this.close.emit()}toggleAudioTrack(t){let e=this.audioItems();if(t<0||t>=e.length)return;if(t===this.currentAudioIndex()){this.audioPlayer?.togglePlay(),this.audioPlaying.update(i=>!i);return}let n=e[t];this.currentAudioIndex.set(t),this.currentAudioTitle.set(n.name),this.getBlobUrl(n.id).then(i=>{i&&(this.currentAudioSrc.set(i),this.audioPlaying.set(!0),setTimeout(()=>{this.audioPlayer?.togglePlay()},100))})}onAudioEnded(){this.audioPlaying.set(!1);let t=this.audioItems(),e=this.currentAudioIndex();e>=0&&e<t.length-1?(this.currentAudioIndex.set(-1),this.toggleAudioTrack(e+1)):(this.currentAudioIndex.set(-1),this.currentAudioSrc.set(""))}resetAudio(){this.audioPlayer?.playing()&&this.audioPlayer.togglePlay(),this.currentAudioIndex.set(-1),this.currentAudioSrc.set(""),this.currentAudioTitle.set(""),this.audioPlaying.set(!1)}async loadCurrentItem(){let t=this.currentItem();if(!(!t||!this.visible())&&t.type==="image"){let e=await this.getBlobUrl(t.id);e&&(this.currentImageUrl.set(e),this.blobUrls.has(t.id)&&this.imageLoaded.set(!0))}}async getBlobUrl(t){if(this.blobUrls.has(t))return this.blobUrls.get(t);try{let e=await this.fileRepo.get(t);if(e){let n=URL.createObjectURL(e.data);return this.blobUrls.set(t,n),n}}catch{}return null}revokeAllBlobUrls(){for(let t of this.blobUrls.values())URL.revokeObjectURL(t);this.blobUrls.clear()}static \u0275fac=function(e){return new(e||r)};static \u0275cmp=x({type:r,selectors:[["app-gallery-lightbox"]],viewQuery:function(e,n){if(e&1&&V(ee,5),e&2){let i;R(i=B())&&(n.audioPlayer=i.first)}},hostBindings:function(e,n){e&1&&p("keydown",function(m){return n.onKeydown(m)},se)},inputs:{visible:[1,"visible"],items:[1,"items"],currentIndex:[1,"currentIndex"]},outputs:{close:"close",indexChange:"indexChange"},decls:1,vars:1,consts:[["tabindex","0",1,"lightbox-overlay"],["tabindex","0",1,"lightbox-overlay",3,"click"],[1,"lightbox-topbar"],[1,"item-name"],["mat-icon-button","","aria-label","Fechar",1,"close-btn",3,"click"],[1,"lightbox-body"],["aria-label","Anterior",1,"nav-btn","nav-prev"],[1,"lightbox-content"],[1,"lightbox-image",3,"src","alt","loaded"],[1,"audio-panel"],[1,"other-placeholder"],["aria-label","Pr\xF3ximo",1,"nav-btn","nav-next"],[1,"lightbox-bottombar"],[1,"counter"],["aria-label","Anterior",1,"nav-btn","nav-prev",3,"click"],[1,"lightbox-image",3,"load","error","src","alt"],[1,"audio-panel-title"],[1,"audio-panel-icon"],[1,"audio-track-list"],[1,"audio-track-item",3,"active"],[1,"current-player"],[1,"audio-track-item",3,"click"],[1,"track-play-icon"],[1,"track-name"],[3,"ended","src","title"],[1,"other-icon"],[1,"other-name"],["aria-label","Pr\xF3ximo",1,"nav-btn","nav-next",3,"click"]],template:function(e,n){e&1&&u(0,nt,17,6,"div",0),e&2&&g(n.visible()?0:-1)},dependencies:[S,w,M,P,X,ee],styles:['@charset "UTF-8";[_nghost-%COMP%]{display:contents}.lightbox-overlay[_ngcontent-%COMP%]{position:fixed;inset:0;z-index:1000;display:flex;flex-direction:column;background:#000000eb;-webkit-backdrop-filter:blur(4px);backdrop-filter:blur(4px);animation:_ngcontent-%COMP%_fadeIn .2s ease}@keyframes _ngcontent-%COMP%_fadeIn{0%{opacity:0}to{opacity:1}}.lightbox-topbar[_ngcontent-%COMP%]{display:flex;align-items:center;justify-content:space-between;padding:12px 16px;min-height:56px;flex-shrink:0}.item-name[_ngcontent-%COMP%]{font-size:.9rem;opacity:.7;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;margin-right:12px}.close-btn[_ngcontent-%COMP%]{flex-shrink:0;color:#ffffffb3;transition:color .15s,transform .15s}.close-btn[_ngcontent-%COMP%]:hover{color:#fff;transform:scale(1.1)}.lightbox-body[_ngcontent-%COMP%]{flex:1;display:flex;align-items:center;justify-content:center;gap:16px;padding:8px 16px;min-height:0}.lightbox-content[_ngcontent-%COMP%]{flex:1;display:flex;align-items:center;justify-content:center;max-width:90vw;max-height:90vh}.lightbox-image[_ngcontent-%COMP%]{max-width:90vw;max-height:85vh;object-fit:contain;border-radius:4px;opacity:0;transition:opacity .35s ease;box-shadow:0 4px 40px #00000080}.lightbox-image.loaded[_ngcontent-%COMP%]{opacity:1}.nav-btn[_ngcontent-%COMP%]{flex-shrink:0;display:flex;align-items:center;justify-content:center;width:48px;height:48px;border-radius:50%;border:none;background:#ffffff0f;color:#fff9;cursor:pointer;transition:background .2s,color .2s,transform .2s}.nav-btn[_ngcontent-%COMP%]:hover{background:#ffffff24;color:#fff;transform:scale(1.08)}.nav-btn[_ngcontent-%COMP%]:active{transform:scale(.95)}.nav-btn[_ngcontent-%COMP%]   mat-icon[_ngcontent-%COMP%]{font-size:32px;width:32px;height:32px}.audio-panel[_ngcontent-%COMP%]{max-width:500px;width:100%;padding:8px 0}.audio-panel-title[_ngcontent-%COMP%]{display:flex;align-items:center;gap:8px;margin:0 0 16px;font-size:1rem;font-weight:500;opacity:.7}.audio-panel-icon[_ngcontent-%COMP%]{font-size:1.2rem;width:1.2rem;height:1.2rem;opacity:.6}.audio-track-list[_ngcontent-%COMP%]{display:flex;flex-direction:column;gap:4px;margin-bottom:16px;max-height:400px;overflow-y:auto}.audio-track-item[_ngcontent-%COMP%]{all:unset;display:flex;align-items:center;gap:12px;padding:10px 14px;border-radius:8px;cursor:pointer;transition:background .15s;font-size:.88rem;color:#ffffffb3}.audio-track-item[_ngcontent-%COMP%]:hover{background:#ffffff0f;color:#fff}.audio-track-item.active[_ngcontent-%COMP%]{background:#ce93d81f;color:#ce93d8}.track-play-icon[_ngcontent-%COMP%]{font-size:1.3rem;width:1.3rem;height:1.3rem;flex-shrink:0}.track-name[_ngcontent-%COMP%]{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.current-player[_ngcontent-%COMP%]{padding:8px;border-radius:8px;background:#ffffff08}.other-placeholder[_ngcontent-%COMP%]{display:flex;flex-direction:column;align-items:center;gap:12px;opacity:.5}.other-icon[_ngcontent-%COMP%]{font-size:3rem;width:3rem;height:3rem}.other-name[_ngcontent-%COMP%]{margin:0;font-size:1rem;text-align:center}.lightbox-bottombar[_ngcontent-%COMP%]{display:flex;align-items:center;justify-content:center;padding:12px 16px;min-height:48px;flex-shrink:0}.counter[_ngcontent-%COMP%]{font-size:.85rem;font-variant-numeric:tabular-nums;opacity:.5}@media(max-width:480px){.lightbox-topbar[_ngcontent-%COMP%]{padding:8px 10px;gap:8px}.item-name[_ngcontent-%COMP%]{font-size:.8rem;margin-right:8px}.lightbox-topbar[_ngcontent-%COMP%]   .close-btn[_ngcontent-%COMP%]{width:36px;height:36px;line-height:36px}.lightbox-topbar[_ngcontent-%COMP%]   .close-btn[_ngcontent-%COMP%]   mat-icon[_ngcontent-%COMP%]{font-size:20px;width:20px;height:20px;line-height:20px}}']})};function it(r,t){if(r&1){let e=h();a(0,"app-gallery-upload",10),p("uploadComplete",function(){f(e);let i=d();return _(i.onUploadComplete())}),o()}}var Ee=class r{store=y(T);subscription;breadcrumbs=[{label:"Galeria"}];showUpload=c(!0);lightboxVisible=c(!1);lightboxItems=c([]);lightboxIndex=c(0);constructor(){this.subscription=this.store.getAll("gallery").subscribe(t=>{this.lightboxItems.set(t)})}ngOnDestroy(){this.subscription?.unsubscribe()}openLightbox(t){let n=this.lightboxItems().findIndex(i=>i.id===t.id);n>=0&&(this.lightboxIndex.set(n),this.lightboxVisible.set(!0))}closeLightbox(){this.lightboxVisible.set(!1)}onUploadComplete(){this.showUpload.set(!1)}static \u0275fac=function(e){return new(e||r)};static \u0275cmp=x({type:r,selectors:[["app-gallery"]],decls:15,vars:7,consts:[[1,"gallery-page"],["title","Galeria","icon","collections_bookmark",3,"breadcrumbs"],[1,"gallery-content"],[1,"upload-area"],[1,"upload-header"],[1,"upload-heading"],[1,"heading-icon"],["mat-icon-button","","aria-label","Alternar upload",1,"collapse-btn",3,"click"],[3,"lightboxOpen"],[3,"close","indexChange","visible","items","currentIndex"],[3,"uploadComplete"]],template:function(e,n){e&1&&(a(0,"div",0),v(1,"app-page-header",1),a(2,"div",2)(3,"section",3)(4,"div",4)(5,"h3",5)(6,"mat-icon",6),s(7,"add_photo_alternate"),o(),s(8," Adicionar M\xEDdia "),o(),a(9,"button",7),p("click",function(){return n.showUpload.set(!n.showUpload())}),a(10,"mat-icon"),s(11),o()()(),u(12,it,1,0,"app-gallery-upload"),o(),a(13,"app-gallery-grid",8),p("lightboxOpen",function(m){return n.openLightbox(m)}),o()()(),a(14,"app-gallery-lightbox",9),p("close",function(){return n.closeLightbox()})("indexChange",function(m){return n.lightboxIndex.set(m)}),o()),e&2&&(l(),b("breadcrumbs",n.breadcrumbs),l(8),D("aria-expanded",n.showUpload()),l(2),C(n.showUpload()?"expand_less":"expand_more"),l(),g(n.showUpload()?12:-1),l(2),b("visible",n.lightboxVisible())("items",n.lightboxItems())("currentIndex",n.lightboxIndex()))},dependencies:[S,w,M,P,X,Pe,H,Q,q],styles:["[_nghost-%COMP%]{display:block}.gallery-page[_ngcontent-%COMP%]{max-width:1200px;margin:0 auto}.gallery-content[_ngcontent-%COMP%]{display:flex;flex-direction:column;gap:24px;margin-top:8px}.upload-area[_ngcontent-%COMP%]{border-radius:12px;background:#ffffff05;border:1px solid rgba(255,255,255,.04);overflow:hidden}.upload-header[_ngcontent-%COMP%]{display:flex;align-items:center;justify-content:space-between;padding:12px 24px}.upload-heading[_ngcontent-%COMP%]{display:flex;align-items:center;gap:8px;margin:0;font-size:.95rem;font-weight:500;opacity:.7}.heading-icon[_ngcontent-%COMP%]{font-size:1.2rem;width:1.2rem;height:1.2rem;opacity:.6}.collapse-btn[_ngcontent-%COMP%]{opacity:.4;transition:opacity .15s}.collapse-btn[_ngcontent-%COMP%]:hover{opacity:.8}"]})};export{Ee as GalleryComponent};
