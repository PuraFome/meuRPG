import{a as Gt}from"./chunk-YJGR3ZFA.js";import{a as ve,b as xe,c as ke}from"./chunk-VUUWDMQL.js";import"./chunk-NI3SGP4Y.js";import"./chunk-WMS5QWT6.js";import{a as Qt}from"./chunk-NS433YDR.js";import"./chunk-7LU7Z45C.js";import{a as Wt}from"./chunk-6P2T57PJ.js";import{a as Lt}from"./chunk-6VMDPNE6.js";import{a as Xt,b as jt}from"./chunk-U7O4B46E.js";import{a as Ht,b as $t}from"./chunk-FPNZMGYV.js";import"./chunk-UWMHAUF2.js";import{a as zt}from"./chunk-NYHWTZ2G.js";import{a as ge,b as be}from"./chunk-G4D42ZWV.js";import"./chunk-WAVAGPFD.js";import{a as Ft}from"./chunk-2OWPJV7R.js";import"./chunk-WENFVTIW.js";import"./chunk-67HIRYZA.js";import{b as qt}from"./chunk-OWRMLCCM.js";import"./chunk-LPQ5J3Q7.js";import"./chunk-BYEKSZ6U.js";import"./chunk-TAZHMNKP.js";import{a as Nt,b as Ut}from"./chunk-VVMOWHP2.js";import{C as ue,E as Bt,J as _e,K as fe,a as gt,b as bt,g as me,l as pe,z as he}from"./chunk-FG66T72J.js";import{a as Be}from"./chunk-QK2GSHD7.js";import"./chunk-O26QDNLQ.js";import{a as Et,b as Rt,c as Dt,d as Pt,e as Vt,f as At,g as Ot}from"./chunk-UYEHVHDK.js";import{e as Ae,f as Mt,g as Ct,i as Oe,j as yt,m as wt,q as St,s as Tt,v as It}from"./chunk-RXTSQZIA.js";import{a as st,c as lt}from"./chunk-J67BR7VW.js";import"./chunk-XRKHPZX2.js";import{$ as N,D as ut,E as ce,N as te,O as Pe,Q as _t,R as Ve,S as ft,T as vt,U as G,V as K,X as xt,Y as B,Z as kt,a as Q,d as dt,e as ct,o as mt,u as pt,z as ht}from"./chunk-BAFNKI6U.js";import{$ as c,Bb as p,Ca as qe,Cb as Ze,Db as Ye,Eb as Je,Fb as ee,Gb as S,Ha as d,Hb as T,Ja as Se,Jb as et,Jc as De,Ka as Qe,Kb as tt,Ma as Te,Mb as le,Nb as w,Ob as Re,Pb as l,Q as Xe,Qb as D,Rb as P,Sb as it,Ta as y,Ua as se,Ub as X,V as je,Va as z,Vb as j,W as He,Wb as H,X as re,Ya as Ie,Z as O,Za as Ge,Zb as de,_b as nt,d as R,ea as _,fa as f,gb as U,hb as g,ia as J,ic as at,j as ze,ja as $e,jb as b,kb as Ke,lb as Ee,ma as oe,mb as I,n as we,na as L,nb as E,ob as v,oc as rt,pb as o,qa as M,qb as s,qc as $,rb as k,rc as q,sc as V,tc as ot,va as W,wb as C,zb as u}from"./chunk-YBA3NCY7.js";import{a as F}from"./chunk-QXIBXHVB.js";function pi(n,a){if(n&1){let e=C();o(0,"div",1)(1,"button",2),u("click",function(){_(e);let i=p();return f(i.action())}),l(2),s()()}if(n&2){let e=p();d(2),P(" ",e.data.action," ")}}var hi=["label"];function ui(n,a){}var _i=Math.pow(2,31)-1,ie=class{_overlayRef;instance;containerInstance;_afterDismissed=new R;_afterOpened=new R;_onAction=new R;_durationTimeoutId;_dismissedByAction=!1;constructor(a,e){this._overlayRef=e,this.containerInstance=a,a._onExit.subscribe(()=>this._finishDismiss())}dismiss(){this._afterDismissed.closed||this.containerInstance.exit(),clearTimeout(this._durationTimeoutId)}dismissWithAction(){this._onAction.closed||(this._dismissedByAction=!0,this._onAction.next(),this._onAction.complete(),this.dismiss()),clearTimeout(this._durationTimeoutId)}closeWithAction(){this.dismissWithAction()}_dismissAfter(a){this._durationTimeoutId=setTimeout(()=>this.dismiss(),Math.min(a,_i))}_open(){this._afterOpened.closed||(this._afterOpened.next(),this._afterOpened.complete())}_finishDismiss(){this._overlayRef.dispose(),this._onAction.closed||this._onAction.complete(),this._afterDismissed.next({dismissedByAction:this._dismissedByAction}),this._afterDismissed.complete(),this._dismissedByAction=!1}afterDismissed(){return this._afterDismissed}afterOpened(){return this.containerInstance._onEnter}onAction(){return this._onAction}},Kt=new O("MatSnackBarData"),Z=class{politeness="polite";announcementMessage="";viewContainerRef;duration=0;panelClass;direction;data=null;horizontalPosition="center";verticalPosition="bottom"},fi=(()=>{class n{static \u0275fac=function(t){return new(t||n)};static \u0275dir=z({type:n,selectors:[["","matSnackBarLabel",""]],hostAttrs:[1,"mat-mdc-snack-bar-label","mdc-snackbar__label"]})}return n})(),gi=(()=>{class n{static \u0275fac=function(t){return new(t||n)};static \u0275dir=z({type:n,selectors:[["","matSnackBarActions",""]],hostAttrs:[1,"mat-mdc-snack-bar-actions","mdc-snackbar__actions"]})}return n})(),bi=(()=>{class n{static \u0275fac=function(t){return new(t||n)};static \u0275dir=z({type:n,selectors:[["","matSnackBarAction",""]],hostAttrs:[1,"mat-mdc-snack-bar-action","mdc-snackbar__action"]})}return n})(),Zt=(()=>{class n{snackBarRef=c(ie);data=c(Kt);constructor(){}action(){this.snackBarRef.dismissWithAction()}get hasAction(){return!!this.data.action}static \u0275fac=function(t){return new(t||n)};static \u0275cmp=y({type:n,selectors:[["simple-snack-bar"]],hostAttrs:[1,"mat-mdc-simple-snack-bar"],exportAs:["matSnackBar"],decls:3,vars:2,consts:[["matSnackBarLabel",""],["matSnackBarActions",""],["matButton","","matSnackBarAction","",3,"click"]],template:function(t,i){t&1&&(o(0,"div",0),l(1),s(),g(2,pi,3,1,"div",1)),t&2&&(d(),P(" ",i.data.message,`
`),d(),b(i.hasAction?2:-1))},dependencies:[B,fi,gi,bi],styles:[`.mat-mdc-simple-snack-bar {
  display: flex;
}
.mat-mdc-simple-snack-bar .mat-mdc-snack-bar-label {
  max-height: 50vh;
  overflow: auto;
}
`],encapsulation:2,changeDetection:0})}return n})(),Ne="_mat-snack-bar-enter",Ue="_mat-snack-bar-exit",vi=(()=>{class n extends Ct{_ngZone=c(L);_elementRef=c(W);_changeDetectorRef=c($);_platform=c(Q);_animationsDisabled=te();snackBarConfig=c(Z);_document=c($e);_trackedModals=new Set;_enterFallback;_exitFallback;_injector=c(J);_announceDelay=150;_announceTimeoutId;_destroyed=!1;_portalOutlet;_onAnnounce=new R;_onExit=new R;_onEnter=new R;_animationState="void";_live;_label;_role;_liveElementId=c(ht).getId("mat-snack-bar-container-live-");constructor(){super();let e=this.snackBarConfig;e.politeness==="assertive"&&!e.announcementMessage?this._live="assertive":e.politeness==="off"?this._live="off":this._live="polite",this._platform.FIREFOX&&(this._live==="polite"&&(this._role="status"),this._live==="assertive"&&(this._role="alert"))}attachComponentPortal(e){this._assertNotAttached();let t=this._portalOutlet.attachComponentPortal(e);return this._afterPortalAttached(),t}attachTemplatePortal(e){this._assertNotAttached();let t=this._portalOutlet.attachTemplatePortal(e);return this._afterPortalAttached(),t}attachDomPortal=e=>{this._assertNotAttached();let t=this._portalOutlet.attachDomPortal(e);return this._afterPortalAttached(),t};onAnimationEnd(e){e===Ue?this._completeExit():e===Ne&&(clearTimeout(this._enterFallback),this._ngZone.run(()=>{this._onEnter.next(),this._onEnter.complete()}))}enter(){this._destroyed||(this._animationState="visible",this._changeDetectorRef.markForCheck(),this._changeDetectorRef.detectChanges(),this._screenReaderAnnounce(),this._animationsDisabled?Se(()=>{this._ngZone.run(()=>queueMicrotask(()=>this.onAnimationEnd(Ne)))},{injector:this._injector}):(clearTimeout(this._enterFallback),this._enterFallback=setTimeout(()=>{this._elementRef.nativeElement.classList.add("mat-snack-bar-fallback-visible"),this.onAnimationEnd(Ne)},200)))}exit(){return this._destroyed?ze(void 0):(this._ngZone.run(()=>{this._animationState="hidden",this._changeDetectorRef.markForCheck(),this._elementRef.nativeElement.setAttribute("mat-exit",""),clearTimeout(this._announceTimeoutId),this._animationsDisabled?Se(()=>{this._ngZone.run(()=>queueMicrotask(()=>this.onAnimationEnd(Ue)))},{injector:this._injector}):(clearTimeout(this._exitFallback),this._exitFallback=setTimeout(()=>this.onAnimationEnd(Ue),200))}),this._onExit)}ngOnDestroy(){this._destroyed=!0,this._clearFromModals(),this._completeExit()}_completeExit(){clearTimeout(this._exitFallback),queueMicrotask(()=>{this._onExit.next(),this._onExit.complete()})}_afterPortalAttached(){let e=this._elementRef.nativeElement,t=this.snackBarConfig.panelClass;t&&(Array.isArray(t)?t.forEach(h=>e.classList.add(h)):e.classList.add(t)),this._exposeToModals();let i=this._label.nativeElement,r="mdc-snackbar__label";i.classList.toggle(r,!i.querySelector(`.${r}`))}_exposeToModals(){let e=this._liveElementId,t=this._document.querySelectorAll('body > .cdk-overlay-container [aria-modal="true"]');for(let i=0;i<t.length;i++){let r=t[i],h=r.getAttribute("aria-owns");this._trackedModals.add(r),h?h.indexOf(e)===-1&&r.setAttribute("aria-owns",h+" "+e):r.setAttribute("aria-owns",e)}}_clearFromModals(){this._trackedModals.forEach(e=>{let t=e.getAttribute("aria-owns");if(t){let i=t.replace(this._liveElementId,"").trim();i.length>0?e.setAttribute("aria-owns",i):e.removeAttribute("aria-owns")}}),this._trackedModals.clear()}_assertNotAttached(){this._portalOutlet.hasAttached()}_screenReaderAnnounce(){this._announceTimeoutId||this._ngZone.runOutsideAngular(()=>{this._announceTimeoutId=setTimeout(()=>{if(this._destroyed)return;let e=this._elementRef.nativeElement,t=e.querySelector("[aria-hidden]"),i=e.querySelector("[aria-live]");if(t&&i){let r=null;this._platform.isBrowser&&document.activeElement instanceof HTMLElement&&t.contains(document.activeElement)&&(r=document.activeElement),t.removeAttribute("aria-hidden"),i.appendChild(t),r?.focus(),this._onAnnounce.next(),this._onAnnounce.complete()}},this._announceDelay)})}static \u0275fac=function(t){return new(t||n)};static \u0275cmp=y({type:n,selectors:[["mat-snack-bar-container"]],viewQuery:function(t,i){if(t&1&&ee(Oe,7)(hi,7),t&2){let r;S(r=T())&&(i._portalOutlet=r.first),S(r=T())&&(i._label=r.first)}},hostAttrs:[1,"mdc-snackbar","mat-mdc-snack-bar-container"],hostVars:6,hostBindings:function(t,i){t&1&&u("animationend",function(h){return i.onAnimationEnd(h.animationName)})("animationcancel",function(h){return i.onAnimationEnd(h.animationName)}),t&2&&w("mat-snack-bar-container-enter",i._animationState==="visible")("mat-snack-bar-container-exit",i._animationState==="hidden")("mat-snack-bar-container-animations-enabled",!i._animationsDisabled)},features:[Ie],decls:6,vars:3,consts:[["label",""],[1,"mdc-snackbar__surface","mat-mdc-snackbar-surface"],[1,"mat-mdc-snack-bar-label"],["aria-hidden","true"],["cdkPortalOutlet",""]],template:function(t,i){t&1&&(o(0,"div",1)(1,"div",2,0)(3,"div",3),Ge(4,ui,0,0,"ng-template",4),s(),k(5,"div"),s()()),t&2&&(d(5),U("aria-live",i._live)("role",i._role)("id",i._liveElementId))},dependencies:[Oe],styles:[`@keyframes _mat-snack-bar-enter {
  from {
    transform: scale(0.8);
    opacity: 0;
  }
  to {
    transform: scale(1);
    opacity: 1;
  }
}
@keyframes _mat-snack-bar-exit {
  from {
    opacity: 1;
  }
  to {
    opacity: 0;
  }
}
.mat-mdc-snack-bar-container {
  display: flex;
  align-items: center;
  justify-content: center;
  box-sizing: border-box;
  -webkit-tap-highlight-color: rgba(0, 0, 0, 0);
  margin: 8px;
}
.mat-mdc-snack-bar-handset .mat-mdc-snack-bar-container {
  width: 100vw;
}

.mat-snack-bar-container-animations-enabled {
  opacity: 0;
}
.mat-snack-bar-container-animations-enabled.mat-snack-bar-fallback-visible {
  opacity: 1;
}
.mat-snack-bar-container-animations-enabled.mat-snack-bar-container-enter {
  animation: _mat-snack-bar-enter 150ms cubic-bezier(0, 0, 0.2, 1) forwards;
}
.mat-snack-bar-container-animations-enabled.mat-snack-bar-container-exit {
  animation: _mat-snack-bar-exit 75ms cubic-bezier(0.4, 0, 1, 1) forwards;
}

.mat-mdc-snackbar-surface {
  box-shadow: 0px 3px 5px -1px rgba(0, 0, 0, 0.2), 0px 6px 10px 0px rgba(0, 0, 0, 0.14), 0px 1px 18px 0px rgba(0, 0, 0, 0.12);
  display: flex;
  align-items: center;
  justify-content: flex-start;
  box-sizing: border-box;
  padding-left: 0;
  padding-right: 8px;
}
[dir=rtl] .mat-mdc-snackbar-surface {
  padding-right: 0;
  padding-left: 8px;
}
.mat-mdc-snack-bar-container .mat-mdc-snackbar-surface {
  min-width: 344px;
  max-width: 672px;
}
.mat-mdc-snack-bar-handset .mat-mdc-snackbar-surface {
  width: 100%;
  min-width: 0;
}
@media (forced-colors: active) {
  .mat-mdc-snackbar-surface {
    outline: solid 1px;
  }
}
.mat-mdc-snack-bar-container .mat-mdc-snackbar-surface {
  color: var(--mat-snack-bar-supporting-text-color, var(--mat-sys-inverse-on-surface));
  border-radius: var(--mat-snack-bar-container-shape, var(--mat-sys-corner-extra-small));
  background-color: var(--mat-snack-bar-container-color, var(--mat-sys-inverse-surface));
}

.mdc-snackbar__label {
  width: 100%;
  flex-grow: 1;
  box-sizing: border-box;
  margin: 0;
  padding: 14px 8px 14px 16px;
}
[dir=rtl] .mdc-snackbar__label {
  padding-left: 8px;
  padding-right: 16px;
}
.mat-mdc-snack-bar-container .mdc-snackbar__label {
  font-family: var(--mat-snack-bar-supporting-text-font, var(--mat-sys-body-medium-font));
  font-size: var(--mat-snack-bar-supporting-text-size, var(--mat-sys-body-medium-size));
  font-weight: var(--mat-snack-bar-supporting-text-weight, var(--mat-sys-body-medium-weight));
  line-height: var(--mat-snack-bar-supporting-text-line-height, var(--mat-sys-body-medium-line-height));
}

.mat-mdc-snack-bar-actions {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  box-sizing: border-box;
}

.mat-mdc-snack-bar-handset,
.mat-mdc-snack-bar-container,
.mat-mdc-snack-bar-label {
  flex: 1 1 auto;
}

.mat-mdc-snack-bar-container .mat-mdc-button.mat-mdc-snack-bar-action:not(:disabled).mat-unthemed {
  color: var(--mat-snack-bar-button-color, var(--mat-sys-inverse-primary));
}
.mat-mdc-snack-bar-container .mat-mdc-button.mat-mdc-snack-bar-action:not(:disabled) {
  --mat-button-text-state-layer-color: currentColor;
  --mat-button-text-ripple-color: currentColor;
}
.mat-mdc-snack-bar-container .mat-mdc-button.mat-mdc-snack-bar-action:not(:disabled) .mat-ripple-element {
  opacity: 0.1;
}
`],encapsulation:2})}return n})(),xi=new O("mat-snack-bar-default-options",{providedIn:"root",factory:()=>new Z}),Fe=(()=>{class n{_live=c(pt);_injector=c(J);_breakpointObserver=c(dt);_parentSnackBar=c(n,{optional:!0,skipSelf:!0});_defaultConfig=c(xi);_animationsDisabled=te();_snackBarRefAtThisLevel=null;simpleSnackBarComponent=Zt;snackBarContainerComponent=vi;handsetCssClass="mat-mdc-snack-bar-handset";get _openedSnackBarRef(){let e=this._parentSnackBar;return e?e._openedSnackBarRef:this._snackBarRefAtThisLevel}set _openedSnackBarRef(e){this._parentSnackBar?this._parentSnackBar._openedSnackBarRef=e:this._snackBarRefAtThisLevel=e}constructor(){}openFromComponent(e,t){return this._attach(e,t)}openFromTemplate(e,t){return this._attach(e,t)}open(e,t="",i){let r=F(F({},this._defaultConfig),i);return r.data={message:e,action:t},r.announcementMessage===e&&(r.announcementMessage=void 0),this.openFromComponent(this.simpleSnackBarComponent,r)}dismiss(){this._openedSnackBarRef&&this._openedSnackBarRef.dismiss()}ngOnDestroy(){this._snackBarRefAtThisLevel&&this._snackBarRefAtThisLevel.dismiss()}_attachSnackBarContainer(e,t){let i=t&&t.viewContainerRef&&t.viewContainerRef.injector,r=J.create({parent:i||this._injector,providers:[{provide:Z,useValue:t}]}),h=new Ae(this.snackBarContainerComponent,t.viewContainerRef,r),x=e.attach(h);return x.instance.snackBarConfig=t,x.instance}_attach(e,t){let i=F(F(F({},new Z),this._defaultConfig),t),r=this._createOverlay(i),h=this._attachSnackBarContainer(r,i),x=new ie(h,r);if(e instanceof Qe){let A=new Mt(e,null,{$implicit:i.data,snackBarRef:x});x.instance=h.attachTemplatePortal(A)}else{let A=this._createInjector(i,x),ye=new Ae(e,void 0,A),ae=h.attachComponentPortal(ye);x.instance=ae.instance}return this._breakpointObserver.observe(ct.HandsetPortrait).pipe(Xe(r.detachments())).subscribe(A=>{r.overlayElement.classList.toggle(this.handsetCssClass,A.matches)}),i.announcementMessage&&h._onAnnounce.subscribe(()=>{this._live.announce(i.announcementMessage,i.politeness)}),this._animateSnackBar(x,i),this._openedSnackBarRef=x,this._openedSnackBarRef}_animateSnackBar(e,t){e.afterDismissed().subscribe(()=>{this._openedSnackBarRef==e&&(this._openedSnackBarRef=null),t.announcementMessage&&this._live.clear()}),t.duration&&t.duration>0&&e.afterOpened().subscribe(()=>e._dismissAfter(t.duration)),this._openedSnackBarRef?(this._openedSnackBarRef.afterDismissed().subscribe(()=>{e.containerInstance.enter()}),this._openedSnackBarRef.dismiss()):e.containerInstance.enter()}_createOverlay(e){let t=new wt;t.direction=e.direction;let i=St(this._injector),r=e.direction==="rtl",h=e.horizontalPosition==="left"||e.horizontalPosition==="start"&&!r||e.horizontalPosition==="end"&&r,x=!h&&e.horizontalPosition!=="center";return h?i.left("0"):x?i.right("0"):i.centerHorizontally(),e.verticalPosition==="top"?i.top("0"):i.bottom("0"),t.positionStrategy=i,t.disableAnimations=this._animationsDisabled,Tt(this._injector,t)}_createInjector(e,t){let i=e&&e.viewContainerRef&&e.viewContainerRef.injector;return J.create({parent:i||this._injector,providers:[{provide:ie,useValue:t},{provide:Kt,useValue:e.data}]})}static \u0275fac=function(t){return new(t||n)};static \u0275prov=He({token:n,factory:n.\u0275fac,providedIn:"root"})}return n})();var Yt=(()=>{class n{static \u0275fac=function(t){return new(t||n)};static \u0275mod=se({type:n});static \u0275inj=re({providers:[Fe],imports:[It,yt,N,Zt,ce]})}return n})();var ki=["knob"],Mi=["valueIndicatorContainer"];function Ci(n,a){if(n&1&&(o(0,"div",2,1)(2,"div",5)(3,"span",6),l(4),s()()()),n&2){let e=p();d(4),D(e.valueIndicatorText)}}var yi=["trackActive"],wi=["*"];function Si(n,a){if(n&1&&k(0,"div"),n&2){let e=a.$implicit,t=a.$index,i=p(3);Re(e===0?"mdc-slider__tick-mark--active":"mdc-slider__tick-mark--inactive"),le("transform",i._calcTickMarkTransform(t))}}function Ti(n,a){if(n&1&&I(0,Si,1,4,"div",8,Ke),n&2){let e=p(2);E(e._tickMarks)}}function Ii(n,a){if(n&1&&(o(0,"div",6,1),g(2,Ti,2,0),s()),n&2){let e=p();d(2),b(e._cachedWidth?2:-1)}}function Ei(n,a){if(n&1&&k(0,"mat-slider-visual-thumb",7),n&2){let e=p();v("discrete",e.discrete)("thumbPosition",1)("valueIndicatorText",e.startValueIndicatorText)}}var m=(function(n){return n[n.START=1]="START",n[n.END=2]="END",n})(m||{}),Y=(function(n){return n[n.ACTIVE=0]="ACTIVE",n[n.INACTIVE=1]="INACTIVE",n})(Y||{}),Le=new O("_MatSlider"),Jt=new O("_MatSliderThumb"),Ri=new O("_MatSliderRangeThumb"),ei=new O("_MatSliderVisualThumb");var Di=(()=>{class n{_cdr=c($);_ngZone=c(L);_slider=c(Le);_renderer=c(Te);_listenerCleanups;discrete=!1;thumbPosition;valueIndicatorText;_ripple;_knob;_valueIndicatorContainer;_sliderInput;_sliderInputEl;_hoverRippleRef;_focusRippleRef;_activeRippleRef;_isHovered=!1;_isActive=!1;_isValueIndicatorVisible=!1;_hostElement=c(W).nativeElement;_platform=c(Q);constructor(){}ngAfterViewInit(){let e=this._slider._getInput(this.thumbPosition);e&&(this._ripple.radius=24,this._sliderInput=e,this._sliderInputEl=this._sliderInput._hostElement,this._ngZone.runOutsideAngular(()=>{let t=this._sliderInputEl,i=this._renderer;this._listenerCleanups=[i.listen(t,"pointermove",this._onPointerMove),i.listen(t,"pointerdown",this._onDragStart),i.listen(t,"pointerup",this._onDragEnd),i.listen(t,"pointerleave",this._onMouseLeave),i.listen(t,"focus",this._onFocus),i.listen(t,"blur",this._onBlur)]}))}ngOnDestroy(){this._listenerCleanups?.forEach(e=>e())}_onPointerMove=e=>{if(this._sliderInput._isFocused)return;let t=this._hostElement.getBoundingClientRect(),i=this._slider._isCursorOnSliderThumb(e,t);this._isHovered=i,i?this._showHoverRipple():this._hideRipple(this._hoverRippleRef)};_onMouseLeave=()=>{this._isHovered=!1,this._hideRipple(this._hoverRippleRef)};_onFocus=()=>{this._hideRipple(this._hoverRippleRef),this._showFocusRipple(),this._hostElement.classList.add("mdc-slider__thumb--focused")};_onBlur=()=>{this._isActive||this._hideRipple(this._focusRippleRef),this._isHovered&&this._showHoverRipple(),this._hostElement.classList.remove("mdc-slider__thumb--focused")};_onDragStart=e=>{e.button===0&&(this._isActive=!0,this._showActiveRipple())};_onDragEnd=()=>{this._isActive=!1,this._hideRipple(this._activeRippleRef),this._sliderInput._isFocused||this._hideRipple(this._focusRippleRef),this._platform.SAFARI&&this._showHoverRipple()};_showHoverRipple(){this._isShowingRipple(this._hoverRippleRef)||(this._hoverRippleRef=this._showRipple({enterDuration:0,exitDuration:0}),this._hoverRippleRef?.element.classList.add("mat-mdc-slider-hover-ripple"))}_showFocusRipple(){this._isShowingRipple(this._focusRippleRef)||(this._focusRippleRef=this._showRipple({enterDuration:0,exitDuration:0},!0),this._focusRippleRef?.element.classList.add("mat-mdc-slider-focus-ripple"))}_showActiveRipple(){this._isShowingRipple(this._activeRippleRef)||(this._activeRippleRef=this._showRipple({enterDuration:225,exitDuration:400}),this._activeRippleRef?.element.classList.add("mat-mdc-slider-active-ripple"))}_isShowingRipple(e){return e?.state===Pe.FADING_IN||e?.state===Pe.VISIBLE}_showRipple(e,t){if(!this._slider.disabled&&(this._showValueIndicator(),this._slider._isRange&&this._slider._getThumb(this.thumbPosition===m.START?m.END:m.START)._showValueIndicator(),!(this._slider._globalRippleOptions?.disabled&&!t)))return this._ripple.launch({animation:this._slider._noopAnimations?{enterDuration:0,exitDuration:0}:e,centered:!0,persistent:!0})}_hideRipple(e){if(e?.fadeOut(),this._isShowingAnyRipple())return;this._slider._isRange||this._hideValueIndicator();let t=this._getSibling();t._isShowingAnyRipple()||(this._hideValueIndicator(),t._hideValueIndicator())}_showValueIndicator(){this._hostElement.classList.add("mdc-slider__thumb--with-indicator")}_hideValueIndicator(){this._hostElement.classList.remove("mdc-slider__thumb--with-indicator")}_getSibling(){return this._slider._getThumb(this.thumbPosition===m.START?m.END:m.START)}_getValueIndicatorContainer(){return this._valueIndicatorContainer?.nativeElement}_getKnob(){return this._knob.nativeElement}_isShowingAnyRipple(){return this._isShowingRipple(this._hoverRippleRef)||this._isShowingRipple(this._focusRippleRef)||this._isShowingRipple(this._activeRippleRef)}static \u0275fac=function(t){return new(t||n)};static \u0275cmp=y({type:n,selectors:[["mat-slider-visual-thumb"]],viewQuery:function(t,i){if(t&1&&ee(Ve,5)(ki,5)(Mi,5),t&2){let r;S(r=T())&&(i._ripple=r.first),S(r=T())&&(i._knob=r.first),S(r=T())&&(i._valueIndicatorContainer=r.first)}},hostAttrs:[1,"mdc-slider__thumb","mat-mdc-slider-visual-thumb"],inputs:{discrete:"discrete",thumbPosition:"thumbPosition",valueIndicatorText:"valueIndicatorText"},features:[de([{provide:ei,useExisting:n}])],decls:4,vars:2,consts:[["knob",""],["valueIndicatorContainer",""],[1,"mdc-slider__value-indicator-container"],[1,"mdc-slider__thumb-knob"],["matRipple","",1,"mat-focus-indicator",3,"matRippleDisabled"],[1,"mdc-slider__value-indicator"],[1,"mdc-slider__value-indicator-text"]],template:function(t,i){t&1&&(g(0,Ci,5,1,"div",2),k(1,"div",3,0)(3,"div",4)),t&2&&(b(i.discrete?0:-1),d(3),v("matRippleDisabled",!0))},dependencies:[Ve],styles:[`.mat-mdc-slider-visual-thumb .mat-ripple {
  height: 100%;
  width: 100%;
}

.mat-mdc-slider .mdc-slider__tick-marks {
  justify-content: start;
}
.mat-mdc-slider .mdc-slider__tick-marks .mdc-slider__tick-mark--active,
.mat-mdc-slider .mdc-slider__tick-marks .mdc-slider__tick-mark--inactive {
  position: absolute;
  left: 2px;
}
`],encapsulation:2,changeDetection:0})}return n})(),ti=(()=>{class n{_ngZone=c(L);_cdr=c($);_elementRef=c(W);_dir=c(ut,{optional:!0});_globalRippleOptions=c(_t,{optional:!0});_trackActive;_thumbs;_input;_inputs;get disabled(){return this._disabled}set disabled(e){this._disabled=e;let t=this._getInput(m.END),i=this._getInput(m.START);t&&(t.disabled=this._disabled),i&&(i.disabled=this._disabled)}_disabled=!1;get discrete(){return this._discrete}set discrete(e){this._discrete=e,this._updateValueIndicatorUIs()}_discrete=!1;get showTickMarks(){return this._showTickMarks}set showTickMarks(e){this._showTickMarks=e,this._hasViewInitialized&&(this._updateTickMarkUI(),this._updateTickMarkTrackUI())}_showTickMarks=!1;get min(){return this._min}set min(e){let t=e==null||isNaN(e)?this._min:e;this._min!==t&&this._updateMin(t)}_min=0;color;disableRipple=!1;_updateMin(e){let t=this._min;this._min=e,this._isRange?this._updateMinRange({old:t,new:e}):this._updateMinNonRange(e),this._onMinMaxOrStepChange()}_updateMinRange(e){let t=this._getInput(m.END),i=this._getInput(m.START),r=t.value,h=i.value;i.min=e.new,t.min=Math.max(e.new,i.value),i.max=Math.min(t.max,t.value),i._updateWidthInactive(),t._updateWidthInactive(),e.new<e.old?this._onTranslateXChangeBySideEffect(t,i):this._onTranslateXChangeBySideEffect(i,t),r!==t.value&&this._onValueChange(t),h!==i.value&&this._onValueChange(i)}_updateMinNonRange(e){let t=this._getInput(m.END);if(t){let i=t.value;t.min=e,t._updateThumbUIByValue(),this._updateTrackUI(t),i!==t.value&&this._onValueChange(t)}}get max(){return this._max}set max(e){let t=e==null||isNaN(e)?this._max:e;this._max!==t&&this._updateMax(t)}_max=100;_updateMax(e){let t=this._max;this._max=e,this._isRange?this._updateMaxRange({old:t,new:e}):this._updateMaxNonRange(e),this._onMinMaxOrStepChange()}_updateMaxRange(e){let t=this._getInput(m.END),i=this._getInput(m.START),r=t.value,h=i.value;t.max=e.new,i.max=Math.min(e.new,t.value),t.min=i.value,t._updateWidthInactive(),i._updateWidthInactive(),e.new>e.old?this._onTranslateXChangeBySideEffect(i,t):this._onTranslateXChangeBySideEffect(t,i),r!==t.value&&this._onValueChange(t),h!==i.value&&this._onValueChange(i)}_updateMaxNonRange(e){let t=this._getInput(m.END);if(t){let i=t.value;t.max=e,t._updateThumbUIByValue(),this._updateTrackUI(t),i!==t.value&&this._onValueChange(t)}}get step(){return this._step}set step(e){let t=isNaN(e)?this._step:e;this._step!==t&&this._updateStep(t)}_step=1;_updateStep(e){this._step=e,this._isRange?this._updateStepRange():this._updateStepNonRange(),this._onMinMaxOrStepChange()}_updateStepRange(){let e=this._getInput(m.END),t=this._getInput(m.START),i=e.value,r=t.value,h=t.value;e.min=this._min,t.max=this._max,e.step=this._step,t.step=this._step,this._platform.SAFARI&&(e.value=e.value,t.value=t.value),e.min=Math.max(this._min,t.value),t.max=Math.min(this._max,e.value),t._updateWidthInactive(),e._updateWidthInactive(),e.value<h?this._onTranslateXChangeBySideEffect(t,e):this._onTranslateXChangeBySideEffect(e,t),i!==e.value&&this._onValueChange(e),r!==t.value&&this._onValueChange(t)}_updateStepNonRange(){let e=this._getInput(m.END);if(e){let t=e.value;e.step=this._step,this._platform.SAFARI&&(e.value=e.value),e._updateThumbUIByValue(),t!==e.value&&this._onValueChange(e)}}displayWith=e=>`${e}`;_tickMarks;_noopAnimations=te();_resizeObserver=null;_cachedWidth;_cachedLeft;_rippleRadius=24;startValueIndicatorText="";endValueIndicatorText="";_endThumbTransform;_startThumbTransform;_isRange=!1;_isRtl=at(()=>this._dir?.valueSignal()==="rtl");_hasViewInitialized=!1;_tickMarkTrackWidth=0;_hasAnimation=!1;_resizeTimer=null;_platform=c(Q);constructor(){c(mt).load(ft);let e=this._isRtl();ot(()=>{let t=this._isRtl();t!==e&&(e=t,this._isRange?this._onDirChangeRange():this._onDirChangeNonRange(),this._updateTickMarkUI())})}_knobRadius=8;_inputPadding;ngAfterViewInit(){this._platform.isBrowser&&this._updateDimensions();let e=this._getInput(m.END),t=this._getInput(m.START);this._isRange=!!e&&!!t,this._cdr.detectChanges();let i=this._getThumb(m.END);this._rippleRadius=i._ripple.radius,this._inputPadding=this._rippleRadius-this._knobRadius,this._isRange?this._initUIRange(e,t):this._initUINonRange(e),this._updateTrackUI(e),this._updateTickMarkUI(),this._updateTickMarkTrackUI(),this._observeHostResize(),this._cdr.detectChanges()}_initUINonRange(e){e.initProps(),e.initUI(),this._updateValueIndicatorUI(e),this._hasViewInitialized=!0,e._updateThumbUIByValue()}_initUIRange(e,t){e.initProps(),e.initUI(),t.initProps(),t.initUI(),e._updateMinMax(),t._updateMinMax(),e._updateStaticStyles(),t._updateStaticStyles(),this._updateValueIndicatorUIs(),this._hasViewInitialized=!0,e._updateThumbUIByValue(),t._updateThumbUIByValue()}ngOnDestroy(){this._resizeObserver?.disconnect(),this._resizeObserver=null}_onDirChangeRange(){let e=this._getInput(m.END),t=this._getInput(m.START);e._setIsLeftThumb(),t._setIsLeftThumb(),e.translateX=e._calcTranslateXByValue(),t.translateX=t._calcTranslateXByValue(),e._updateStaticStyles(),t._updateStaticStyles(),e._updateWidthInactive(),t._updateWidthInactive(),e._updateThumbUIByValue(),t._updateThumbUIByValue()}_onDirChangeNonRange(){this._getInput(m.END)._updateThumbUIByValue()}_observeHostResize(){typeof ResizeObserver>"u"||!ResizeObserver||this._ngZone.runOutsideAngular(()=>{this._resizeObserver=new ResizeObserver(()=>{this._isActive()||(this._resizeTimer&&clearTimeout(this._resizeTimer),this._onResize())}),this._resizeObserver.observe(this._elementRef.nativeElement)})}_isActive(){return this._getThumb(m.START)._isActive||this._getThumb(m.END)._isActive}_getValue(e=m.END){let t=this._getInput(e);return t?t.value:this.min}_skipUpdate(){return!!(this._getInput(m.START)?._skipUIUpdate||this._getInput(m.END)?._skipUIUpdate)}_updateDimensions(){this._cachedWidth=this._elementRef.nativeElement.offsetWidth,this._cachedLeft=this._elementRef.nativeElement.getBoundingClientRect().left}_setTrackActiveStyles(e){let t=this._trackActive.nativeElement.style;t.left=e.left,t.right=e.right,t.transformOrigin=e.transformOrigin,t.transform=e.transform}_calcTickMarkTransform(e){let t=e*(this._tickMarkTrackWidth/(this._tickMarks.length-1));return`translateX(${this._isRtl()?this._cachedWidth-6-t:t}px)`}_onTranslateXChange(e){this._hasViewInitialized&&(this._updateThumbUI(e),this._updateTrackUI(e),this._updateOverlappingThumbUI(e))}_onTranslateXChangeBySideEffect(e,t){this._hasViewInitialized&&(e._updateThumbUIByValue(),t._updateThumbUIByValue())}_onValueChange(e){this._hasViewInitialized&&(this._updateValueIndicatorUI(e),this._updateTickMarkUI(),this._cdr.detectChanges())}_onMinMaxOrStepChange(){this._hasViewInitialized&&(this._updateTickMarkUI(),this._updateTickMarkTrackUI(),this._cdr.markForCheck())}_onResize(){if(this._hasViewInitialized){if(this._updateDimensions(),this._isRange){let e=this._getInput(m.END),t=this._getInput(m.START);e._updateThumbUIByValue(),t._updateThumbUIByValue(),e._updateStaticStyles(),t._updateStaticStyles(),e._updateMinMax(),t._updateMinMax(),e._updateWidthInactive(),t._updateWidthInactive()}else{let e=this._getInput(m.END);e&&e._updateThumbUIByValue()}this._updateTickMarkUI(),this._updateTickMarkTrackUI(),this._cdr.detectChanges()}}_thumbsOverlap=!1;_areThumbsOverlapping(){let e=this._getInput(m.START),t=this._getInput(m.END);return!e||!t?!1:t.translateX-e.translateX<20}_updateOverlappingThumbClassNames(e){let t=e.getSibling(),i=this._getThumb(e.thumbPosition);this._getThumb(t.thumbPosition)._hostElement.classList.remove("mdc-slider__thumb--top"),i._hostElement.classList.toggle("mdc-slider__thumb--top",this._thumbsOverlap)}_updateOverlappingThumbUI(e){!this._isRange||this._skipUpdate()||this._thumbsOverlap!==this._areThumbsOverlapping()&&(this._thumbsOverlap=!this._thumbsOverlap,this._updateOverlappingThumbClassNames(e))}_updateThumbUI(e){if(this._skipUpdate())return;let t=this._getThumb(e.thumbPosition===m.END?m.END:m.START);t._hostElement.style.transform=`translateX(${e.translateX}px)`}_updateValueIndicatorUI(e){if(this._skipUpdate())return;let t=this.displayWith(e.value);if(this._hasViewInitialized?e._valuetext.set(t):e._hostElement.setAttribute("aria-valuetext",t),this.discrete){e.thumbPosition===m.START?this.startValueIndicatorText=t:this.endValueIndicatorText=t;let i=this._getThumb(e.thumbPosition);t.length<3?i._hostElement.classList.add("mdc-slider__thumb--short-value"):i._hostElement.classList.remove("mdc-slider__thumb--short-value")}}_updateValueIndicatorUIs(){let e=this._getInput(m.END),t=this._getInput(m.START);e&&this._updateValueIndicatorUI(e),t&&this._updateValueIndicatorUI(t)}_updateTickMarkTrackUI(){if(!this.showTickMarks||this._skipUpdate())return;let e=this._step&&this._step>0?this._step:1,i=(Math.floor(this.max/e)*e-this.min)/(this.max-this.min);this._tickMarkTrackWidth=(this._cachedWidth-6)*i}_updateTrackUI(e){this._skipUpdate()||(this._isRange?this._updateTrackUIRange(e):this._updateTrackUINonRange(e))}_updateTrackUIRange(e){let t=e.getSibling();if(!t||!this._cachedWidth)return;let i=Math.abs(t.translateX-e.translateX)/this._cachedWidth;e._isLeftThumb&&this._cachedWidth?this._setTrackActiveStyles({left:"auto",right:`${this._cachedWidth-t.translateX}px`,transformOrigin:"right",transform:`scaleX(${i})`}):this._setTrackActiveStyles({left:`${t.translateX}px`,right:"auto",transformOrigin:"left",transform:`scaleX(${i})`})}_updateTrackUINonRange(e){this._isRtl()?this._setTrackActiveStyles({left:"auto",right:"0px",transformOrigin:"right",transform:`scaleX(${1-e.fillPercentage})`}):this._setTrackActiveStyles({left:"0px",right:"auto",transformOrigin:"left",transform:`scaleX(${e.fillPercentage})`})}_updateTickMarkUI(){if(!this.showTickMarks||this.step===void 0||this.min===void 0||this.max===void 0)return;let e=this.step>0?this.step:1;this._isRange?this._updateTickMarkUIRange(e):this._updateTickMarkUINonRange(e)}_updateTickMarkUINonRange(e){let t=this._getValue(),i=Math.max(Math.round((t-this.min)/e),0)+1,r=Math.max(Math.round((this.max-t)/e),0)-1;this._isRtl()?i++:r++,this._tickMarks=Array(i).fill(Y.ACTIVE).concat(Array(r).fill(Y.INACTIVE))}_updateTickMarkUIRange(e){let t=this._getValue(),i=this._getValue(m.START),r=Math.max(Math.round((i-this.min)/e),0),h=Math.max(Math.round((t-i)/e)+1,0),x=Math.max(Math.round((this.max-t)/e),0);this._tickMarks=Array(r).fill(Y.INACTIVE).concat(Array(h).fill(Y.ACTIVE),Array(x).fill(Y.INACTIVE))}_getInput(e){if(e===m.END&&this._input)return this._input;if(this._inputs?.length)return e===m.START?this._inputs.first:this._inputs.last}_getThumb(e){return e===m.END?this._thumbs?.last:this._thumbs?.first}_setTransition(e){this._hasAnimation=!this._platform.IOS&&e&&!this._noopAnimations,this._elementRef.nativeElement.classList.toggle("mat-mdc-slider-with-animation",this._hasAnimation)}_isCursorOnSliderThumb(e,t){let i=t.width/2,r=t.x+i,h=t.y+i,x=e.clientX-r,A=e.clientY-h;return Math.pow(x,2)+Math.pow(A,2)<Math.pow(i,2)}static \u0275fac=function(t){return new(t||n)};static \u0275cmp=y({type:n,selectors:[["mat-slider"]],contentQueries:function(t,i,r){if(t&1&&Je(r,Jt,5)(r,Ri,4),t&2){let h;S(h=T())&&(i._input=h.first),S(h=T())&&(i._inputs=h)}},viewQuery:function(t,i){if(t&1&&ee(yi,5)(ei,5),t&2){let r;S(r=T())&&(i._trackActive=r.first),S(r=T())&&(i._thumbs=r)}},hostAttrs:[1,"mat-mdc-slider","mdc-slider"],hostVars:12,hostBindings:function(t,i){t&2&&(Re("mat-"+(i.color||"primary")),w("mdc-slider--range",i._isRange)("mdc-slider--disabled",i.disabled)("mdc-slider--discrete",i.discrete)("mdc-slider--tick-marks",i.showTickMarks)("_mat-animation-noopable",i._noopAnimations))},inputs:{disabled:[2,"disabled","disabled",q],discrete:[2,"discrete","discrete",q],showTickMarks:[2,"showTickMarks","showTickMarks",q],min:[2,"min","min",V],color:"color",disableRipple:[2,"disableRipple","disableRipple",q],max:[2,"max","max",V],step:[2,"step","step",V],displayWith:"displayWith"},exportAs:["matSlider"],features:[de([{provide:Le,useExisting:n}])],ngContentSelectors:wi,decls:9,vars:5,consts:[["trackActive",""],["tickMarkContainer",""],[1,"mdc-slider__track"],[1,"mdc-slider__track--inactive"],[1,"mdc-slider__track--active"],[1,"mdc-slider__track--active_fill"],[1,"mdc-slider__tick-marks"],[3,"discrete","thumbPosition","valueIndicatorText"],[3,"class","transform"]],template:function(t,i){t&1&&(Ze(),Ye(0),o(1,"div",2),k(2,"div",3),o(3,"div",4),k(4,"div",5,0),s(),g(6,Ii,3,1,"div",6),s(),g(7,Ei,1,3,"mat-slider-visual-thumb",7),k(8,"mat-slider-visual-thumb",7)),t&2&&(d(6),b(i.showTickMarks?6:-1),d(),b(i._isRange?7:-1),d(),v("discrete",i.discrete)("thumbPosition",2)("valueIndicatorText",i.endValueIndicatorText))},dependencies:[Di],styles:[`.mdc-slider__track {
  position: absolute;
  top: 50%;
  transform: translateY(-50%);
  width: 100%;
  pointer-events: none;
  height: var(--mat-slider-inactive-track-height, 4px);
}

.mdc-slider__track--active,
.mdc-slider__track--inactive {
  display: flex;
  height: 100%;
  position: absolute;
  width: 100%;
}

.mdc-slider__track--active {
  overflow: hidden;
  border-radius: var(--mat-slider-active-track-shape, var(--mat-sys-corner-full));
  height: var(--mat-slider-active-track-height, 4px);
  top: calc((var(--mat-slider-inactive-track-height, 4px) - var(--mat-slider-active-track-height, 4px)) / 2);
}

.mdc-slider__track--active_fill {
  border-top-style: solid;
  box-sizing: border-box;
  height: 100%;
  width: 100%;
  position: relative;
  transform-origin: left;
  transition: transform 80ms ease;
  border-color: var(--mat-slider-active-track-color, var(--mat-sys-primary));
  border-top-width: var(--mat-slider-active-track-height, 4px);
}
.mdc-slider--disabled .mdc-slider__track--active_fill {
  border-color: var(--mat-slider-disabled-active-track-color, var(--mat-sys-on-surface));
}
[dir=rtl] .mdc-slider__track--active_fill {
  -webkit-transform-origin: right;
  transform-origin: right;
}

.mdc-slider__track--inactive {
  left: 0;
  top: 0;
  opacity: 0.24;
  background-color: var(--mat-slider-inactive-track-color, var(--mat-sys-surface-variant));
  height: var(--mat-slider-inactive-track-height, 4px);
  border-radius: var(--mat-slider-inactive-track-shape, var(--mat-sys-corner-full));
}
.mdc-slider--disabled .mdc-slider__track--inactive {
  background-color: var(--mat-slider-disabled-inactive-track-color, var(--mat-sys-on-surface));
  opacity: 0.24;
}
.mdc-slider__track--inactive::before {
  position: absolute;
  box-sizing: border-box;
  width: 100%;
  height: 100%;
  top: 0;
  left: 0;
  border: 1px solid transparent;
  border-radius: inherit;
  content: "";
  pointer-events: none;
}
@media (forced-colors: active) {
  .mdc-slider__track--inactive::before {
    border-color: CanvasText;
  }
}

.mdc-slider__value-indicator-container {
  bottom: 44px;
  left: 50%;
  pointer-events: none;
  position: absolute;
  transform: var(--mat-slider-value-indicator-container-transform, translateX(-50%) rotate(-45deg));
}
.mdc-slider__thumb--with-indicator .mdc-slider__value-indicator-container {
  pointer-events: auto;
}

.mdc-slider__value-indicator {
  display: flex;
  align-items: center;
  transform: scale(0);
  transform-origin: var(--mat-slider-value-indicator-transform-origin, 0 28px);
  transition: transform 100ms cubic-bezier(0.4, 0, 1, 1);
  word-break: normal;
  background-color: var(--mat-slider-label-container-color, var(--mat-sys-primary));
  color: var(--mat-slider-label-label-text-color, var(--mat-sys-on-primary));
  width: var(--mat-slider-value-indicator-width, 28px);
  height: var(--mat-slider-value-indicator-height, 28px);
  padding: var(--mat-slider-value-indicator-padding, 0);
  opacity: var(--mat-slider-value-indicator-opacity, 1);
  border-radius: var(--mat-slider-value-indicator-border-radius, 50% 50% 50% 0);
}
.mdc-slider__thumb--with-indicator .mdc-slider__value-indicator {
  transition: transform 100ms cubic-bezier(0, 0, 0.2, 1);
  transform: scale(1);
}
.mdc-slider__value-indicator::before {
  border-left: 6px solid transparent;
  border-right: 6px solid transparent;
  border-top: 6px solid;
  bottom: -5px;
  content: "";
  height: 0;
  left: 50%;
  position: absolute;
  transform: translateX(-50%);
  width: 0;
  display: var(--mat-slider-value-indicator-caret-display, none);
  border-top-color: var(--mat-slider-label-container-color, var(--mat-sys-primary));
}
.mdc-slider__value-indicator::after {
  position: absolute;
  box-sizing: border-box;
  width: 100%;
  height: 100%;
  top: 0;
  left: 0;
  border: 1px solid transparent;
  border-radius: inherit;
  content: "";
  pointer-events: none;
}
@media (forced-colors: active) {
  .mdc-slider__value-indicator::after {
    border-color: CanvasText;
  }
}

.mdc-slider__value-indicator-text {
  text-align: center;
  width: var(--mat-slider-value-indicator-width, 28px);
  transform: var(--mat-slider-value-indicator-text-transform, rotate(45deg));
  font-family: var(--mat-slider-label-label-text-font, var(--mat-sys-label-medium-font));
  font-size: var(--mat-slider-label-label-text-size, var(--mat-sys-label-medium-size));
  font-weight: var(--mat-slider-label-label-text-weight, var(--mat-sys-label-medium-weight));
  line-height: var(--mat-slider-label-label-text-line-height, var(--mat-sys-label-medium-line-height));
  letter-spacing: var(--mat-slider-label-label-text-tracking, var(--mat-sys-label-medium-tracking));
}

.mdc-slider__thumb {
  -webkit-user-select: none;
  user-select: none;
  display: flex;
  left: -24px;
  outline: none;
  position: absolute;
  height: 48px;
  width: 48px;
  pointer-events: none;
}
.mdc-slider--discrete .mdc-slider__thumb {
  transition: transform 80ms ease;
}
.mdc-slider--disabled .mdc-slider__thumb {
  pointer-events: none;
}

.mdc-slider__thumb--top {
  z-index: 1;
}

.mdc-slider__thumb-knob {
  position: absolute;
  box-sizing: border-box;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  border-style: solid;
  width: var(--mat-slider-handle-width, 20px);
  height: var(--mat-slider-handle-height, 20px);
  border-width: calc(var(--mat-slider-handle-height, 20px) / 2) calc(var(--mat-slider-handle-width, 20px) / 2);
  box-shadow: var(--mat-slider-handle-elevation, var(--mat-sys-level1));
  background-color: var(--mat-slider-handle-color, var(--mat-sys-primary));
  border-color: var(--mat-slider-handle-color, var(--mat-sys-primary));
  border-radius: var(--mat-slider-handle-shape, var(--mat-sys-corner-full));
}
.mdc-slider__thumb:hover .mdc-slider__thumb-knob {
  background-color: var(--mat-slider-hover-handle-color, var(--mat-sys-primary));
  border-color: var(--mat-slider-hover-handle-color, var(--mat-sys-primary));
}
.mdc-slider__thumb--focused .mdc-slider__thumb-knob {
  background-color: var(--mat-slider-focus-handle-color, var(--mat-sys-primary));
  border-color: var(--mat-slider-focus-handle-color, var(--mat-sys-primary));
}
.mdc-slider--disabled .mdc-slider__thumb-knob {
  background-color: var(--mat-slider-disabled-handle-color, var(--mat-sys-on-surface));
  border-color: var(--mat-slider-disabled-handle-color, var(--mat-sys-on-surface));
}
.mdc-slider__thumb--top .mdc-slider__thumb-knob, .mdc-slider__thumb--top.mdc-slider__thumb:hover .mdc-slider__thumb-knob, .mdc-slider__thumb--top.mdc-slider__thumb--focused .mdc-slider__thumb-knob {
  border: solid 1px #fff;
  box-sizing: content-box;
  border-color: var(--mat-slider-with-overlap-handle-outline-color, var(--mat-sys-on-primary));
  border-width: var(--mat-slider-with-overlap-handle-outline-width, 1px);
}

.mdc-slider__tick-marks {
  align-items: center;
  box-sizing: border-box;
  display: flex;
  height: 100%;
  justify-content: space-between;
  padding: 0 1px;
  position: absolute;
  width: 100%;
}

.mdc-slider__tick-mark--active,
.mdc-slider__tick-mark--inactive {
  width: var(--mat-slider-with-tick-marks-container-size, 2px);
  height: var(--mat-slider-with-tick-marks-container-size, 2px);
  border-radius: var(--mat-slider-with-tick-marks-container-shape, var(--mat-sys-corner-full));
}

.mdc-slider__tick-mark--inactive {
  opacity: var(--mat-slider-with-tick-marks-inactive-container-opacity, 0.38);
  background-color: var(--mat-slider-with-tick-marks-inactive-container-color, var(--mat-sys-on-surface-variant));
}
.mdc-slider--disabled .mdc-slider__tick-mark--inactive {
  opacity: var(--mat-slider-with-tick-marks-inactive-container-opacity, 0.38);
  background-color: var(--mat-slider-with-tick-marks-disabled-container-color, var(--mat-sys-on-surface));
}

.mdc-slider__tick-mark--active {
  opacity: var(--mat-slider-with-tick-marks-active-container-opacity, 0.38);
  background-color: var(--mat-slider-with-tick-marks-active-container-color, var(--mat-sys-on-primary));
}

.mdc-slider__input {
  cursor: pointer;
  left: 2px;
  margin: 0;
  height: 44px;
  opacity: 0;
  position: absolute;
  top: 2px;
  width: 44px;
  box-sizing: content-box;
}
.mdc-slider__input.mat-mdc-slider-input-no-pointer-events {
  pointer-events: none;
}
.mdc-slider__input.mat-slider__right-input {
  left: auto;
  right: 0;
}

.mat-mdc-slider {
  display: inline-block;
  box-sizing: border-box;
  outline: none;
  vertical-align: middle;
  cursor: pointer;
  height: 48px;
  margin: 0 8px;
  position: relative;
  touch-action: pan-y;
  width: auto;
  min-width: 112px;
  -webkit-tap-highlight-color: transparent;
}
.mat-mdc-slider.mdc-slider--disabled {
  cursor: auto;
  opacity: 0.38;
}
.mat-mdc-slider.mdc-slider--disabled .mdc-slider__input {
  cursor: auto;
}
.mat-mdc-slider .mdc-slider__thumb,
.mat-mdc-slider .mdc-slider__track--active_fill {
  transition-duration: 0ms;
}
.mat-mdc-slider.mat-mdc-slider-with-animation .mdc-slider__thumb,
.mat-mdc-slider.mat-mdc-slider-with-animation .mdc-slider__track--active_fill {
  transition-duration: 80ms;
}
.mat-mdc-slider.mdc-slider--discrete .mdc-slider__thumb,
.mat-mdc-slider.mdc-slider--discrete .mdc-slider__track--active_fill {
  transition-duration: 0ms;
}
.mat-mdc-slider.mat-mdc-slider-with-animation .mdc-slider__thumb,
.mat-mdc-slider.mat-mdc-slider-with-animation .mdc-slider__track--active_fill {
  transition-duration: 80ms;
}
.mat-mdc-slider .mat-ripple .mat-ripple-element {
  background-color: var(--mat-slider-ripple-color, var(--mat-sys-primary));
}
.mat-mdc-slider .mat-ripple .mat-mdc-slider-hover-ripple {
  background-color: var(--mat-slider-hover-state-layer-color, color-mix(in srgb, var(--mat-sys-primary) 5%, transparent));
}
.mat-mdc-slider .mat-ripple .mat-mdc-slider-focus-ripple,
.mat-mdc-slider .mat-ripple .mat-mdc-slider-active-ripple {
  background-color: var(--mat-slider-focus-state-layer-color, color-mix(in srgb, var(--mat-sys-primary) 20%, transparent));
}
.mat-mdc-slider._mat-animation-noopable.mdc-slider--discrete .mdc-slider__thumb, .mat-mdc-slider._mat-animation-noopable.mdc-slider--discrete .mdc-slider__track--active_fill,
.mat-mdc-slider._mat-animation-noopable .mdc-slider__value-indicator {
  transition: none;
}
.mat-mdc-slider .mat-focus-indicator::before {
  border-radius: 50%;
}

.mdc-slider__thumb--focused .mat-focus-indicator::before {
  content: "";
}
`],encapsulation:2,changeDetection:0})}return n})();var Pi={provide:gt,useExisting:je(()=>We),multi:!0};var We=(()=>{class n{_ngZone=c(L);_elementRef=c(W);_cdr=c($);_slider=c(Le);_platform=c(Q);_listenerCleanups;get value(){return V(this._hostElement.value,0)}set value(e){e===null&&(e=this._getDefaultValue()),e=isNaN(e)?0:e;let t=e+"";if(!this._hasSetInitialValue){this._initialValue=t;return}this._isActive||this._setValue(t)}_setValue(e){this._hostElement.value=e,this._updateThumbUIByValue(),this._slider._onValueChange(this),this._cdr.detectChanges(),this._slider._cdr.markForCheck()}valueChange=new oe;dragStart=new oe;dragEnd=new oe;get translateX(){return this._slider.min>=this._slider.max?(this._translateX=this._tickMarkOffset,this._translateX):(this._translateX===void 0&&(this._translateX=this._calcTranslateXByValue()),this._translateX)}set translateX(e){this._translateX=e}_translateX;thumbPosition=m.END;get min(){return V(this._hostElement.min,0)}set min(e){this._hostElement.min=e+"",this._cdr.detectChanges()}get max(){return V(this._hostElement.max,0)}set max(e){this._hostElement.max=e+"",this._cdr.detectChanges()}get step(){return V(this._hostElement.step,0)}set step(e){this._hostElement.step=e+"",this._cdr.detectChanges()}get disabled(){return q(this._hostElement.disabled)}set disabled(e){this._hostElement.disabled=e,this._cdr.detectChanges(),this._slider.disabled!==this.disabled&&(this._slider.disabled=this.disabled)}get percentage(){return this._slider.min>=this._slider.max?this._slider._isRtl()?1:0:(this.value-this._slider.min)/(this._slider.max-this._slider.min)}get fillPercentage(){return this._slider._cachedWidth?this._translateX===0?0:this.translateX/this._slider._cachedWidth:this._slider._isRtl()?1:0}_hostElement=this._elementRef.nativeElement;_valuetext=M("");_knobRadius=8;_tickMarkOffset=3;_isActive=!1;_isFocused=!1;_setIsFocused(e){this._isFocused=e}_hasSetInitialValue=!1;_initialValue;_formControl;_destroyed=new R;_skipUIUpdate=!1;_onChangeFn;_onTouchedFn=()=>{};_isControlInitialized=!1;constructor(){let e=c(Te);this._ngZone.runOutsideAngular(()=>{this._listenerCleanups=[e.listen(this._hostElement,"pointerdown",this._onPointerDown.bind(this)),e.listen(this._hostElement,"pointermove",this._onPointerMove.bind(this)),e.listen(this._hostElement,"pointerup",this._onPointerUp.bind(this))]})}ngOnDestroy(){this._listenerCleanups.forEach(e=>e()),this._destroyed.next(),this._destroyed.complete(),this.dragStart.complete(),this.dragEnd.complete()}initProps(){this._updateWidthInactive(),this.disabled!==this._slider.disabled&&(this._slider.disabled=!0),this.step=this._slider.step,this.min=this._slider.min,this.max=this._slider.max,this._initValue()}initUI(){this._updateThumbUIByValue()}_initValue(){this._hasSetInitialValue=!0,this._initialValue===void 0?this.value=this._getDefaultValue():(this._hostElement.value=this._initialValue,this._updateThumbUIByValue(),this._slider._onValueChange(this),this._cdr.detectChanges())}_getDefaultValue(){return this.min}_onBlur(){this._setIsFocused(!1),this._onTouchedFn()}_onFocus(){this._slider._setTransition(!1),this._slider._updateTrackUI(this),this._setIsFocused(!0)}_onChange(){this.valueChange.emit(this.value),this._isActive&&this._updateThumbUIByValue({withAnimation:!0})}_onInput(){this._onChangeFn?.(this.value),(this._slider.step||!this._isActive)&&this._updateThumbUIByValue({withAnimation:!0}),this._slider._onValueChange(this)}_onNgControlValueChange(){(!this._isActive||!this._isFocused)&&(this._slider._onValueChange(this),this._updateThumbUIByValue()),this._slider.disabled=this._formControl.disabled}_onPointerDown(e){if(!(this.disabled||e.button!==0)){if(this._platform.IOS){let t=this._slider._isCursorOnSliderThumb(e,this._slider._getThumb(this.thumbPosition)._hostElement.getBoundingClientRect());this._isActive=t,this._updateWidthActive(),this._slider._updateDimensions();return}this._isActive=!0,this._setIsFocused(!0),this._updateWidthActive(),this._slider._updateDimensions(),this._slider.step||this._updateThumbUIByPointerEvent(e,{withAnimation:!0}),this.disabled||(this._handleValueCorrection(e),this.dragStart.emit({source:this,parent:this._slider,value:this.value}))}}_handleValueCorrection(e){this._skipUIUpdate=!0,setTimeout(()=>{this._skipUIUpdate=!1,this._fixValue(e)},0)}_fixValue(e){let t=e.clientX-this._slider._cachedLeft,i=this._slider._cachedWidth,r=this._slider.step===0?1:this._slider.step,h=Math.floor((this._slider.max-this._slider.min)/r),x=this._slider._isRtl()?1-t/i:t/i,ye=Math.round(x*h)/h*(this._slider.max-this._slider.min)+this._slider.min,ae=Math.round(ye/r)*r,mi=this.value;if(ae===mi){this._slider._onValueChange(this),this._slider.step>0?this._updateThumbUIByValue():this._updateThumbUIByPointerEvent(e,{withAnimation:this._slider._hasAnimation});return}this.value=ae,this.valueChange.emit(this.value),this._onChangeFn?.(this.value),this._slider._onValueChange(this),this._slider.step>0?this._updateThumbUIByValue():this._updateThumbUIByPointerEvent(e,{withAnimation:this._slider._hasAnimation})}_onPointerMove(e){!this._slider.step&&this._isActive&&this._updateThumbUIByPointerEvent(e)}_onPointerUp(){this._isActive&&(this._isActive=!1,this._platform.SAFARI&&this._setIsFocused(!1),this.dragEnd.emit({source:this,parent:this._slider,value:this.value}),setTimeout(()=>this._updateWidthInactive(),this._platform.IOS?10:0))}_clamp(e){let t=this._tickMarkOffset,i=this._slider._cachedWidth-this._tickMarkOffset;return Math.max(Math.min(e,i),t)}_calcTranslateXByValue(){return this._slider._isRtl()?(1-this.percentage)*(this._slider._cachedWidth-this._tickMarkOffset*2)+this._tickMarkOffset:this.percentage*(this._slider._cachedWidth-this._tickMarkOffset*2)+this._tickMarkOffset}_calcTranslateXByPointerEvent(e){return e.clientX-this._slider._cachedLeft}_updateWidthActive(){}_updateWidthInactive(){this._hostElement.style.padding=`0 ${this._slider._inputPadding}px`,this._hostElement.style.width=`calc(100% + ${this._slider._inputPadding-this._tickMarkOffset*2}px)`,this._hostElement.style.left=`-${this._slider._rippleRadius-this._tickMarkOffset}px`}_updateThumbUIByValue(e){this.translateX=this._clamp(this._calcTranslateXByValue()),this._updateThumbUI(e)}_updateThumbUIByPointerEvent(e,t){this.translateX=this._clamp(this._calcTranslateXByPointerEvent(e)),this._updateThumbUI(t)}_updateThumbUI(e){this._slider._setTransition(!!e?.withAnimation),this._slider._onTranslateXChange(this)}writeValue(e){(this._isControlInitialized||e!==null)&&(this.value=e)}registerOnChange(e){this._onChangeFn=e,this._isControlInitialized=!0}registerOnTouched(e){this._onTouchedFn=e}setDisabledState(e){this.disabled=e}focus(){this._hostElement.focus()}blur(){this._hostElement.blur()}static \u0275fac=function(t){return new(t||n)};static \u0275dir=z({type:n,selectors:[["input","matSliderThumb",""]],hostAttrs:["type","range",1,"mdc-slider__input"],hostVars:1,hostBindings:function(t,i){t&1&&u("change",function(){return i._onChange()})("input",function(){return i._onInput()})("blur",function(){return i._onBlur()})("focus",function(){return i._onFocus()}),t&2&&U("aria-valuetext",i._valuetext())},inputs:{value:[2,"value","value",V]},outputs:{valueChange:"valueChange",dragStart:"dragStart",dragEnd:"dragEnd"},exportAs:["matSliderThumb"],features:[de([Pi,{provide:Jt,useExisting:n}])]})}return n})();var ii=(()=>{class n{static \u0275fac=function(t){return new(t||n)};static \u0275mod=se({type:n});static \u0275inj=re({imports:[vt,ce]})}return n})();var Ce=class n{mapService=c(ge);dungeonService=c(be);toggleGrid(){this.mapService.toggleGrid()}toggleFogOfWar(){this.mapService.toggleFogOfWar()}toggleMarkers(){this.mapService.toggleMarkers()}toggleSubmapPins(){this.mapService.toggleSubmapPins()}toggleDungeon(){this.dungeonService.toggle()}clearDungeon(){this.dungeonService.clear()}setFogOpacity(a){a!==null&&this.mapService.setFogOpacity(a)}resetFog(){this.mapService.resetFogOfWar()}static \u0275fac=function(e){return new(e||n)};static \u0275cmp=y({type:n,selectors:[["app-map-config-panel"]],decls:27,vars:0,consts:[[1,"config-panel"],[1,"panel-title"],[1,"checkbox-group"],[3,"change"],[1,"slider-section"],[1,"slider-label"],["min","0","max","1","step","0.1"],["matSliderThumb","",3,"valueChange"],["mat-stroked-button","",1,"reset-btn",3,"click"],["mat-stroked-button","",1,"reset-btn","danger",3,"click"]],template:function(e,t){e&1&&(o(0,"div",0)(1,"h3",1),l(2,"Camadas"),s(),o(3,"div",2)(4,"mat-checkbox",3),u("change",function(){return t.toggleGrid()}),l(5," Grade "),s(),o(6,"mat-checkbox",3),u("change",function(){return t.toggleFogOfWar()}),l(7," N\xE9voa da Guerra "),s(),o(8,"mat-checkbox",3),u("change",function(){return t.toggleMarkers()}),l(9," Pontos de Interesse "),s(),o(10,"mat-checkbox",3),u("change",function(){return t.toggleSubmapPins()}),l(11," Pins de Submapa "),s(),o(12,"mat-checkbox",3),u("change",function(){return t.toggleDungeon()}),l(13," Desenho da Masmorra "),s()(),o(14,"div",4)(15,"label",5),l(16,"Opacidade da N\xE9voa"),s(),o(17,"mat-slider",6)(18,"input",7),u("valueChange",function(r){return t.setFogOpacity(r)}),s()()(),o(19,"button",8),u("click",function(){return t.resetFog()}),o(20,"mat-icon"),l(21,"refresh"),s(),l(22," Reset Fog of War "),s(),o(23,"button",9),u("click",function(){return t.clearDungeon()}),o(24,"mat-icon"),l(25,"delete_sweep"),s(),l(26," Limpar desenho da masmorra "),s()())},dependencies:[jt,Xt,ii,ti,We,N,B,K,G],styles:["[_nghost-%COMP%]{position:absolute;right:16px;top:64px;z-index:10;width:min(240px,100vw - 32px)}.config-panel[_ngcontent-%COMP%]{background:#1e1e1e;border:1px solid rgba(255,255,255,.12);border-radius:12px;padding:16px;display:flex;flex-direction:column;gap:12px;box-shadow:0 4px 24px #0006}.panel-title[_ngcontent-%COMP%]{margin:0;font-size:.875rem;font-weight:500;color:#ffffffde;text-transform:uppercase;letter-spacing:.5px}.checkbox-group[_ngcontent-%COMP%]{display:flex;flex-direction:column;gap:8px}[_nghost-%COMP%]     .checkbox-group .mdc-checkbox{flex-shrink:0}.slider-section[_ngcontent-%COMP%]{display:flex;flex-direction:column;gap:4px}.slider-label[_ngcontent-%COMP%]{font-size:.75rem;color:#fff9}.reset-btn[_ngcontent-%COMP%]{width:100%}.reset-btn.danger[_ngcontent-%COMP%]{color:#ff8a80;border-color:#ff8a8066}"]})};var Ni=()=>["image/"],Ui=(n,a)=>a.id,Fi=(n,a)=>a.value;function Li(n,a){if(n&1&&(o(0,"mat-option",7),l(1),s()),n&2){let e=a.$implicit;v("value",e.id),d(),D(e.name)}}function Wi(n,a){if(n&1&&(o(0,"mat-option",7),l(1),s()),n&2){let e=a.$implicit;v("value",e.value),d(),D(e.label)}}function zi(n,a){if(n&1&&(o(0,"div",20),k(1,"img",21),o(2,"span"),l(3),s()()),n&2){let e=p(2);d(),v("src",e.submapImage().dataUrl,qe),d(2),it("",e.submapImage().width," \xD7 ",e.submapImage().height," px")}}function Xi(n,a){if(n&1){let e=C();o(0,"div",9)(1,"mat-form-field",2)(2,"mat-label"),l(3,"Nome do novo submapa"),s(),o(4,"input",18),H("ngModelChange",function(i){_(e);let r=p();return j(r.newSubmapName,i)||(r.newSubmapName=i),f(i)}),s()(),o(5,"mat-form-field",2)(6,"mat-label"),l(7,"Tipo do novo submapa"),s(),o(8,"mat-select",5),H("ngModelChange",function(i){_(e);let r=p();return j(r.newSubmapKind,i)||(r.newSubmapKind=i),f(i)}),I(9,Wi,2,2,"mat-option",7,Fi),s()(),o(11,"label",10),l(12,"Imagem do submapa (opcional)"),s(),o(13,"app-file-upload",19),u("fileChange",function(i){_(e);let r=p();return f(r.onSubmapImageSelected(i))}),s(),g(14,zi,4,3,"div",20),s()}if(n&2){let e=p();d(4),X("ngModel",e.newSubmapName),d(4),X("ngModel",e.newSubmapKind),d(),E(e.submapKinds),d(4),v("acceptedTypes",nt(6,Ni))("maxSize",15*1024*1024)("showPreview",!1),d(),b(e.submapImage()?14:-1)}}function ji(n,a){if(n&1){let e=C();o(0,"button",22),u("click",function(){let i=_(e).$implicit,r=p();return f(r.selectedColor=i)}),s()}if(n&2){let e=a.$implicit,t=p();le("background",e),w("selected",t.selectedColor===e),U("aria-label","Cor "+e)}}function Hi(n,a){if(n&1){let e=C();o(0,"button",23),u("click",function(){let i=_(e).$implicit,r=p();return f(r.selectedIcon=i)}),o(1,"span",24),l(2),s()()}if(n&2){let e=a.$implicit,t=p();w("selected",t.selectedIcon===e),U("aria-label","\xCDcone "+e),d(2),D(e)}}function $i(n,a){if(n&1){let e=C();o(0,"button",27),u("click",function(){_(e);let i=p(2);return f(i.onOpen())}),o(1,"mat-icon"),l(2,"open_in_new"),s(),l(3," Abrir mapa "),s()}}function qi(n,a){if(n&1){let e=C();o(0,"button",25),u("click",function(){_(e);let i=p();return f(i.onDelete())}),o(1,"mat-icon"),l(2,"delete"),s(),l(3," Excluir "),s(),g(4,$i,4,0,"button",26)}if(n&2){let e=p();d(4),b(e.existing.targetMapId?4:-1)}}var oi="__new__",si=["#7c4dff","#e53935","#ff6d00","#ffd600","#00c853","#2979ff","#00bcd4","#ff4081","#6d4c41","#78909c"],li=["\u{1F37A}","\u{1F377}","\u{1F356}","\u{1F6CF}\uFE0F","\u{1F3F0}","\u{1F3DB}\uFE0F","\u26EA","\u{1F3E0}","\u{1F3D8}\uFE0F","\u{1F6D2}","\u2692\uFE0F","\u{1F9D9}","\u{1F47A}","\u{1F479}","\u{1F9CC}","\u{1F409}","\u{1F480}","\u{1F9DF}","\u{1F577}\uFE0F","\u{1F43A}","\u{1F987}","\u2694\uFE0F","\u{1F6E1}\uFE0F","\u{1F3F9}","\u{1F525}","\u{1F573}\uFE0F","\u{1F5DD}\uFE0F","\u{1F48E}","\u{1FA99}","\u{1F9EA}","\u{1F4DC}","\u{1F6AA}","\u2693","\u26F5","\u{1F332}","\u26F0\uFE0F","\u{1F3D5}\uFE0F","\u{1F56F}\uFE0F","\u{1FAA6}","\u{1F9ED}"],Qi=[{value:"dungeon",label:"Masmorra"},{value:"city",label:"Cidade"},{value:"local",label:"Local (loja, taverna...)"},{value:"world",label:"Mundo / Regi\xE3o"}],ne=class n{dialogRef=c(Et);data=c(Rt);colors=si;icons=li;submapKinds=Qi;newSubmapValue=oi;existing=this.data.existing;availableMaps=this.data.availableMaps;label=this.data.existing?.label??"";description=this.data.existing?.description??"";destination=this.data.existing?.targetMapId??"";newSubmapName="";newSubmapKind="dungeon";submapImage=M(null);selectedColor=this.data.existing?.color??si[0];selectedIcon=this.data.existing?.icon??li[0];onCancel(){this.dialogRef.close(null)}onDelete(){this.dialogRef.close({action:"delete"})}onSubmapImageSelected(a){let e=new FileReader;e.onload=()=>{let t=e.result,i=new Image;i.onload=()=>{this.submapImage.set({dataUrl:t,width:i.naturalWidth,height:i.naturalHeight})},i.src=t},e.readAsDataURL(a)}onOpen(){this.dialogRef.close({action:"open"})}onSave(){if(!this.label.trim())return;let a=this.destination===oi;if(a&&!this.newSubmapName.trim())return;let e={id:this.existing?.id??crypto.randomUUID(),x:this.data.x,y:this.data.y,label:this.label.trim(),description:this.description.trim()||void 0,icon:this.selectedIcon,color:this.selectedColor,targetMapId:!a&&this.destination?this.destination:void 0};this.dialogRef.close({action:"save",marker:e,newSubmap:a?{name:this.newSubmapName.trim(),kind:this.newSubmapKind,image:this.submapImage()??void 0}:void 0})}static \u0275fac=function(e){return new(e||n)};static \u0275cmp=y({type:n,selectors:[["app-poi-dialog"]],decls:45,vars:8,consts:[["mat-dialog-title",""],[1,"poi-form"],["appearance","fill",1,"full-width"],["matInput","","placeholder","Ex: Porto de Neverwinter",3,"ngModelChange","ngModel"],["matInput","","rows","3","placeholder","O que os aventureiros veem ao chegar aqui?",3,"ngModelChange","ngModel"],[3,"ngModelChange","ngModel"],["value",""],[3,"value"],[1,"opt-icon"],[1,"new-submap"],[1,"section-label"],[1,"color-options"],["type","button",1,"color-swatch",3,"selected","background"],[1,"icon-options"],["type","button",1,"icon-option",3,"selected"],["align","end"],["mat-button","",3,"click"],["mat-raised-button","","color","primary",3,"click","disabled"],["matInput","","placeholder","Ex: Cripta subterr\xE2nea",3,"ngModelChange","ngModel"],[3,"fileChange","acceptedTypes","maxSize","showPreview"],[1,"submap-preview"],["alt","Preview do submapa",3,"src"],["type","button",1,"color-swatch",3,"click"],["type","button",1,"icon-option",3,"click"],[1,"icon-emoji"],["mat-button","","color","warn",3,"click"],["mat-stroked-button",""],["mat-stroked-button","",3,"click"]],template:function(e,t){e&1&&(o(0,"h2",0),l(1),s(),o(2,"mat-dialog-content")(3,"div",1)(4,"mat-form-field",2)(5,"mat-label"),l(6,"Nome do local"),s(),o(7,"input",3),H("ngModelChange",function(r){return j(t.label,r)||(t.label=r),r}),s()(),o(8,"mat-form-field",2)(9,"mat-label"),l(10,"Descri\xE7\xE3o"),s(),o(11,"textarea",4),H("ngModelChange",function(r){return j(t.description,r)||(t.description=r),r}),s()(),o(12,"mat-form-field",2)(13,"mat-label"),l(14,"Destino do ponto"),s(),o(15,"mat-select",5),H("ngModelChange",function(r){return j(t.destination,r)||(t.destination=r),r}),o(16,"mat-option",6),l(17,"Nenhum"),s(),o(18,"mat-option",7)(19,"mat-icon",8),l(20,"add"),s(),l(21," Criar novo submapa... "),s(),I(22,Li,2,2,"mat-option",7,Ui),s(),o(24,"mat-hint"),l(25,"Ao clicar no ponto, abre este mapa."),s()(),g(26,Xi,15,7,"div",9),o(27,"label",10),l(28,"Cor do pin"),s(),o(29,"div",11),I(30,ji,1,5,"button",12,Ee),s(),o(32,"label",10),l(33,"\xCDcone"),s(),o(34,"div",13),I(35,Hi,3,4,"button",14,Ee),s()()(),o(37,"mat-dialog-actions",15),g(38,qi,5,1),o(39,"button",16),u("click",function(){return t.onCancel()}),l(40,"Cancelar"),s(),o(41,"button",17),u("click",function(){return t.onSave()}),o(42,"mat-icon"),l(43,"check"),s(),l(44," Salvar "),s()()),e&2&&(d(),P(" ",t.existing?"Editar Ponto de Interesse":"Novo Ponto de Interesse"," "),d(6),X("ngModel",t.label),d(4),X("ngModel",t.description),d(4),X("ngModel",t.destination),d(3),v("value",t.newSubmapValue),d(4),E(t.availableMaps),d(4),b(t.destination===t.newSubmapValue?26:-1),d(4),E(t.colors),d(5),E(t.icons),d(3),b(t.existing?38:-1),d(3),v("disabled",!t.label.trim()||t.destination===t.newSubmapValue&&!t.newSubmapName.trim()))},dependencies:[he,bt,me,pe,N,B,Ot,Pt,At,Vt,fe,_e,ue,Bt,K,G,Ut,Nt,ke,xe,ve,Qt],styles:[".poi-form[_ngcontent-%COMP%]{display:flex;flex-direction:column;gap:16px;min-width:min(380px,90vw);padding:8px 0}.full-width[_ngcontent-%COMP%]{width:100%}.opt-icon[_ngcontent-%COMP%]{font-size:1.1rem;width:1.1rem;height:1.1rem;margin-right:8px;vertical-align:middle}.new-submap[_ngcontent-%COMP%]{display:flex;flex-direction:column;gap:8px;padding:12px;border:1px dashed rgba(124,77,255,.5);border-radius:10px;background:#7c4dff0f}.submap-preview[_ngcontent-%COMP%]{display:flex;flex-direction:column;align-items:center;gap:4px}.submap-preview[_ngcontent-%COMP%]   img[_ngcontent-%COMP%]{max-width:100%;max-height:160px;object-fit:contain;border-radius:8px;border:1px solid rgba(255,255,255,.12)}.submap-preview[_ngcontent-%COMP%]   span[_ngcontent-%COMP%]{font-size:.7rem;color:#ffffffb3}.section-label[_ngcontent-%COMP%]{font-size:.8125rem;font-weight:500;color:#ffffffb3;margin-bottom:4px}.color-options[_ngcontent-%COMP%]{display:flex;gap:8px;flex-wrap:wrap}.color-swatch[_ngcontent-%COMP%]{width:32px;height:32px;border-radius:50%;border:2px solid transparent;cursor:pointer;transition:transform .15s,border-color .15s}.color-swatch[_ngcontent-%COMP%]:hover{transform:scale(1.15)}.color-swatch.selected[_ngcontent-%COMP%]{border-color:#fff;transform:scale(1.2)}.icon-options[_ngcontent-%COMP%]{display:flex;gap:8px;flex-wrap:wrap}.icon-option[_ngcontent-%COMP%]{width:40px;height:40px;border-radius:8px;border:2px solid transparent;background:#ffffff14;cursor:pointer;display:flex;align-items:center;justify-content:center;transition:background .15s,border-color .15s}.icon-option[_ngcontent-%COMP%]:hover{background:#ffffff29}.icon-option.selected[_ngcontent-%COMP%]{border-color:#7c4dff;background:#7c4dff33}.icon-emoji[_ngcontent-%COMP%]{font-size:1.35rem;line-height:1}"]})};var Gi=["mapContainer"],Ki=(n,a)=>a.tool,Zi=(n,a)=>a.id;function Yi(n,a){n&1&&(o(0,"div",4),k(1,"app-loading-spinner",5),s()),n&2&&(d(),v("isLoading",!0))}function Ji(n,a){if(n&1){let e=C();o(0,"div",4)(1,"app-empty-state",6),u("action",function(){_(e);let i=p();return f(i.goBack())}),s()()}if(n&2){let e=p();d(),v("message",e.error())}}function en(n,a){if(n&1){let e=C();o(0,"button",16),u("click",function(){_(e);let i=p(2);return f(i.goToParent())}),o(1,"mat-icon"),l(2,"arrow_back"),s(),l(3," Voltar ao mapa pai "),s()}}function tn(n,a){n&1&&(o(0,"span",12),l(1,"Clique no mapa para posicionar o ponto"),s())}function nn(n,a){n&1&&(o(0,"span",12),l(1,"Clique e arraste para pintar as c\xE9lulas"),s())}function an(n,a){if(n&1){let e=C();k(0,"span",17),o(1,"button",18),u("click",function(){_(e);let i=p(2);return f(i.saveCurrentMap())}),o(2,"mat-icon"),l(3),s(),l(4),s(),o(5,"button",16),u("click",function(){_(e);let i=p(2);return f(i.openEditMapDialog())}),o(6,"mat-icon"),l(7,"edit"),s(),l(8," Editar Mapa "),s(),o(9,"button",19),u("click",function(){_(e);let i=p(2);return f(i.deleteMap())}),o(10,"mat-icon"),l(11,"delete"),s(),l(12," Excluir "),s()}if(n&2){let e=p(2);d(),v("disabled",e.saving()),d(2),D(e.saving()?"hourglass_top":"save"),d(),P(" ",e.saving()?"Salvando...":"Salvar"," ")}}function rn(n,a){if(n&1){let e=C();o(0,"button",21),u("click",function(){let i=_(e).$implicit,r=p(3);return f(r.selectTool(i.tool))}),o(1,"mat-icon"),l(2),s(),l(3),s()}if(n&2){let e=a.$implicit,t=p(3);w("active",t.dungeonTool()===e.tool),v("matTooltip",e.label),d(2),D(e.icon),d(),P(" ",e.label," ")}}function on(n,a){if(n&1&&(o(0,"mat-option",24),l(1),s()),n&2){let e=a.$implicit;v("value",e.id),d(),P(" ",e.name," ")}}function sn(n,a){n&1&&(o(0,"span",12),l(1,"Escolha um personagem para carimbar"),s())}function ln(n,a){if(n&1){let e=C();o(0,"mat-form-field",22)(1,"mat-label"),l(2,"Personagem"),s(),o(3,"mat-select",23),u("ngModelChange",function(i){_(e);let r=p(3);return f(r.onCharacterSelected(i))}),I(4,on,2,2,"mat-option",24,Zi),s()(),g(6,sn,2,0,"span",12)}if(n&2){let e=p(3);d(3),v("ngModel",e.selectedCharacterId()),d(),E(e.characters()),d(2),b(e.selectedCharacterId()?-1:6)}}function dn(n,a){if(n&1&&(o(0,"div",13),I(1,rn,4,5,"button",20,Ki),g(3,ln,7,2),s()),n&2){let e=p(2);d(),E(e.dungeonTools),d(2),b(e.dungeonTool()==="character"?3:-1)}}function cn(n,a){n&1&&k(0,"app-map-config-panel")}function mn(n,a){if(n&1){let e=C();o(0,"div",7),g(1,en,4,0,"button",8),o(2,"button",9),u("click",function(){_(e);let i=p();return f(i.togglePinPlacement())}),o(3,"mat-icon"),l(4,"add_location"),s(),l(5," Adicionar Ponto "),s(),o(6,"button",10),u("click",function(){_(e);let i=p();return f(i.toggleDrawMode())}),o(7,"mat-icon"),l(8,"draw"),s(),l(9," Desenhar Masmorra "),s(),o(10,"button",11),u("click",function(){_(e);let i=p();return f(i.toggleGrid())}),o(11,"mat-icon"),l(12,"grid_4x4"),s()(),g(13,tn,2,0,"span",12),g(14,nn,2,0,"span",12),g(15,an,13,3),s(),g(16,dn,4,1,"div",13),g(17,cn,1,0,"app-map-config-panel"),o(18,"button",14),u("click",function(){_(e);let i=p();return f(i.showConfigPanel=!i.showConfigPanel)}),o(19,"mat-icon"),l(20,"layers"),s()(),o(21,"button",15),u("click",function(){_(e);let i=p();return f(i.toggleFullscreen())}),o(22,"mat-icon"),l(23,"fullscreen"),s()()}if(n&2){let e=p();d(),b(e.parentMapId?1:-1),d(),w("active",e.pinPlacementMode()),v("disabled",e.drawMode()),d(4),w("active",e.drawMode()),v("disabled",!e.hasImage())("matTooltip",e.hasImage()?"Desenhar a masmorra sobre a imagem":"Adicione uma imagem ao mapa para desenhar"),d(4),w("active",e.gridVisible()),d(3),b(e.pinPlacementMode()?13:-1),d(),b(e.drawMode()?14:-1),d(),b(e.currentMapData?15:-1),d(),b(e.drawMode()?16:-1),d(),b(e.showConfigPanel?17:-1)}}var di=["#7c4dff","#e53935","#00c853","#2979ff","#ff6d00","#00bcd4","#ff4081","#ffd600"],ci=class n{router=c(lt);route=c(st);dialog=c(Dt);mapService=c(ge);dungeonService=c(be);mapsService=c(qt);snackBar=c(Fe);store=c(Be);characterStore=c(Be);mapContainer=rt.required("mapContainer");showConfigPanel=!1;pinPlacementMode=M(!1);drawMode=M(!1);hasImage=M(!1);gridVisible=M(!1);saving=M(!1);dungeonTool=M("floor");characters=M([]);selectedCharacterId=M(null);mapTitle="Mapa";breadcrumbs=[];parentMapId=null;currentMapData=null;loading=M(!0);error=M(null);dungeonTools=[{tool:"floor",icon:"crop_square",label:"Piso"},{tool:"wall",icon:"border_outer",label:"Parede"},{tool:"false_wall",icon:"border_style",label:"Parede falsa"},{tool:"water",icon:"water_drop",label:"\xC1gua"},{tool:"difficult",icon:"grass",label:"Terreno dif\xEDcil"},{tool:"rubble",icon:"scatter_plot",label:"Escombros"},{tool:"door",icon:"door_front",label:"Porta"},{tool:"secret_door",icon:"key",label:"Porta secreta"},{tool:"trap",icon:"warning",label:"Armadilha"},{tool:"chest",icon:"redeem",label:"Ba\xFA"},{tool:"mimic",icon:"pest_control",label:"M\xEDmico"},{tool:"character",icon:"person_add",label:"Personagem"},{tool:"erase",icon:"cleaning_services",label:"Apagar"}];mapId=null;routeSub=null;characterSub=null;clickUnregister=null;featureUnregister=null;async ngAfterViewInit(){this.loading.set(!0),this.error.set(null);try{await this.mapService.initialize(this.mapContainer().nativeElement,{zoom:10,center:[-46.6333,-23.5505]})}catch(a){console.error("Falha ao inicializar mapa:",a),this.error.set("Erro ao carregar mapa"),this.loading.set(!1);return}this.setupClickHandler(),this.characterSub=this.characterStore.subscribe("characters",a=>{this.characters.set(a),this.applyCharacter()}),this.routeSub=this.route.paramMap.subscribe(a=>{this.mapId=a.get("id"),this.loadMapData()})}ngOnDestroy(){this.clickUnregister?.(),this.featureUnregister?.(),this.routeSub?.unsubscribe(),this.characterSub?.unsubscribe(),this.dungeonService.destroy(),this.mapService.destroy()}toggleFullscreen(){this.mapService.toggleFullscreen(this.mapContainer().nativeElement)}toggleGrid(){this.mapService.toggleGrid().then(()=>{this.gridVisible.set(this.mapService.isGridVisible())})}togglePinPlacement(){this.pinPlacementMode.update(a=>!a)}async toggleDrawMode(){if(this.drawMode()){this.dungeonService.disable(),this.drawMode.set(!1);return}this.hasImage()&&(this.pinPlacementMode.set(!1),this.dungeonService.setTool(this.dungeonTool()),await this.dungeonService.enable(),this.drawMode.set(!0))}selectTool(a){this.dungeonTool.set(a),this.dungeonService.setTool(a),a==="character"&&this.applyCharacter()}onCharacterSelected(a){this.selectedCharacterId.set(a),this.applyCharacter()}applyCharacter(){let a=this.selectedCharacterId(),e=this.characters(),t=e.find(h=>h.id===a);if(!t){this.dungeonService.setCharacter(null);return}let i=di[e.indexOf(t)%di.length],r={id:t.id,name:t.name,color:i};this.dungeonService.setCharacter(r)}goToParent(){this.parentMapId&&this.router.navigate(["/mapa",this.parentMapId])}goBack(){this.router.navigate(["/mapa"])}openEditMapDialog(){if(!this.currentMapData)return;this.dialog.open(Gt,{width:"520px",maxWidth:"95vw",data:{map:this.currentMapData}}).afterClosed().subscribe(e=>{e&&this.loadMapData()})}deleteMap(){if(!this.currentMapData)return;this.dialog.open(Ft,{width:"420px",data:{title:"Excluir Mapa",message:`Tem certeza que deseja excluir "${this.currentMapData.name}"?`,confirmText:"Excluir",cancelText:"Cancelar"}}).afterClosed().subscribe(e=>{e&&this.currentMapData&&(this.store.delete("maps",this.currentMapData.id),this.router.navigate(["/mapa"]))})}async saveCurrentMap(){if(!this.mapId)return;let a=this.store.snapshot("maps").find(e=>e.id===this.mapId);if(!a){this.snackBar.open("Mapa n\xE3o encontrado para salvar.","Fechar",{duration:4e3});return}this.saving.set(!0);try{await we(this.mapsService.update(a.id,a)),this.snackBar.open("Mapa salvo com sucesso!","OK",{duration:3e3})}catch(e){if(e instanceof De&&e.status===404)try{await we(this.mapsService.create(a)),this.snackBar.open("Mapa salvo com sucesso!","OK",{duration:3e3})}catch(t){this.showSaveError(t)}else this.showSaveError(e)}finally{this.saving.set(!1)}}showSaveError(a){this.snackBar.open(`Falha ao salvar: ${pn(a)}`,"Fechar",{duration:6e3})}async loadMapData(){if(this.loading.set(!0),this.dungeonService.disable(),this.drawMode.set(!1),this.mapId){let a=this.store.snapshot("maps").find(e=>e.id===this.mapId);if(a){this.currentMapData=a,this.mapService.setCurrentMapId(this.mapId),this.mapTitle=a.name;let e=this.mapService.getMapHierarchy(this.mapId);this.breadcrumbs=e.map((t,i)=>({label:t.name,route:i<e.length-1?`/mapa/${t.id}`:void 0})),this.parentMapId=this.mapService.getParentMapId(this.mapId),a.backgroundImage?(await this.mapService.setImageBackground(a.backgroundImage,a.width||1024,a.height||768),this.hasImage.set(!0)):(await this.mapService.clearImageBackground(),this.hasImage.set(!1)),await this.mapService.setGridConfig(a.grid),await this.mapService.setGridVisible(!1),this.gridVisible.set(this.mapService.isGridVisible()),await this.dungeonService.configure({mapId:this.mapId,extent:[0,0,a.width||1024,a.height||768],cellSize:a.grid?.cellSize??50,columns:a.grid?.columns??24,rows:a.grid?.rows??18}),await this.dungeonService.load(a.dungeon),await this.mapService.renderPois(a.markers??[]),await this.mapService.showPoisLayer(),await this.mapService.renderSubmapPins(a.submaps??[]),await this.mapService.showSubmapPinsLayer(),this.loading.set(!1);return}this.currentMapData=null,this.mapService.setCurrentMapId(null),this.error.set("Mapa n\xE3o encontrado"),this.loading.set(!1);return}this.currentMapData=null,this.mapService.setCurrentMapId(null),this.mapTitle="Mapa",this.breadcrumbs=[],this.parentMapId=null,this.loading.set(!1)}setupClickHandler(){this.featureUnregister=this.mapService.onFeatureClick(a=>{this.pinPlacementMode()||this.drawMode()||(a.type==="poi"&&a.marker?this.openPoiDialog(a.marker):a.type==="submap"&&a.targetMapId&&this.router.navigate(["/mapa",a.targetMapId]))}),this.clickUnregister=this.mapService.onMapClick(a=>{if(!this.pinPlacementMode()||this.drawMode())return;this.pinPlacementMode.set(!1),this.dialog.open(ne,{data:{x:a[0],y:a[1],availableMaps:this.mapService.getAllMaps().filter(t=>t.id!==this.mapId),currentMapId:this.mapId},width:"460px",maxWidth:"95vw"}).afterClosed().subscribe(t=>{t?.action==="save"&&t.marker&&(this.applyNewSubmap(t),this.mapService.addPoi(t.marker))})})}openPoiDialog(a){this.dialog.open(ne,{data:{x:a.x,y:a.y,existing:a,availableMaps:this.mapService.getAllMaps().filter(t=>t.id!==this.mapId),currentMapId:this.mapId},width:"460px",maxWidth:"95vw"}).afterClosed().subscribe(t=>{t?.action==="save"&&t.marker?(this.applyNewSubmap(t),this.mapService.updatePoi(t.marker)):t?.action==="delete"?this.mapService.deletePoi(a.id):t?.action==="open"&&a.targetMapId&&this.router.navigate(["/mapa",a.targetMapId])})}applyNewSubmap(a){if(!a||a.action!=="save"||!a.newSubmap)return;let e=this.mapService.createSubmap(a.newSubmap.name,a.newSubmap.kind,a.newSubmap.image);a.marker.targetMapId=e.id}static \u0275fac=function(e){return new(e||n)};static \u0275cmp=y({type:n,selectors:[["app-map-view"]],viewQuery:function(e,t){e&1&&et(t.mapContainer,Gi,5),e&2&&tt()},decls:7,vars:3,consts:[["mapContainer",""],["icon","map",3,"title","breadcrumbs"],[1,"map-stage"],[1,"map-container"],[1,"overlay"],["message","Carregando mapa...",3,"isLoading"],["icon","map","actionLabel","Voltar",3,"action","message"],[1,"toolbar-row"],["mat-stroked-button",""],["mat-stroked-button","",3,"click","disabled"],["mat-stroked-button","",3,"click","disabled","matTooltip"],["mat-icon-button","","matTooltip","Mostrar/ocultar grade",3,"click"],[1,"hint"],[1,"tool-row"],["mat-fab","","aria-label","Configura\xE7\xF5es do mapa",1,"fab-btn","config-btn",3,"click"],["mat-fab","","aria-label","Alternar tela cheia",1,"fab-btn","fullscreen-btn",3,"click"],["mat-stroked-button","",3,"click"],[1,"toolbar-spacer"],["mat-raised-button","","color","primary",3,"click","disabled"],["mat-stroked-button","","color","warn",3,"click"],["mat-stroked-button","",1,"tool-btn",3,"active","matTooltip"],["mat-stroked-button","",1,"tool-btn",3,"click","matTooltip"],["appearance","outline","subscriptSizing","dynamic",1,"char-select"],[3,"ngModelChange","ngModel"],[3,"value"]],template:function(e,t){e&1&&(k(0,"app-page-header",1),o(1,"div",2),k(2,"div",3,0),g(4,Yi,2,1,"div",4)(5,Ji,2,1,"div",4)(6,mn,24,15),s()),e&2&&(v("title",t.mapTitle)("breadcrumbs",t.breadcrumbs),d(4),b(t.loading()?4:t.error()?5:6))},dependencies:[he,me,pe,N,B,xt,kt,fe,_e,ue,K,G,ke,xe,ve,Yt,$t,Ht,Ce,zt,Wt,Lt],styles:["[_nghost-%COMP%]{display:flex;flex-direction:column;width:100%;height:calc(100vh - 112px);position:relative;overflow:hidden}.map-stage[_ngcontent-%COMP%]{position:relative;flex:1;min-height:0}.toolbar-row[_ngcontent-%COMP%], .tool-row[_ngcontent-%COMP%]{position:relative;display:flex;flex-wrap:wrap;align-items:center;gap:8px;row-gap:8px;padding:8px 16px;background:#0000004d;z-index:10;flex-shrink:0}.tool-row[_ngcontent-%COMP%]{padding-top:0;background:#00000038}.toolbar-row[_ngcontent-%COMP%]   button.active[_ngcontent-%COMP%], .tool-row[_ngcontent-%COMP%]   button.active[_ngcontent-%COMP%], button.active[_ngcontent-%COMP%]{background:#7c4dff40;border-color:#7c4dff}.hint[_ngcontent-%COMP%]{font-size:.8125rem;color:#fff9;font-style:italic}.toolbar-spacer[_ngcontent-%COMP%]{flex:1}.tool-btn[_ngcontent-%COMP%]{min-width:0}.char-select[_ngcontent-%COMP%]{width:180px;font-size:.85rem}.map-container[_ngcontent-%COMP%]{position:absolute;inset:0;width:100%;height:100%}.overlay[_ngcontent-%COMP%]{position:absolute;inset:0;z-index:20;display:flex;align-items:center;justify-content:center;background:#0a0a1299}.fab-btn[_ngcontent-%COMP%]{position:absolute;z-index:10}.fullscreen-btn[_ngcontent-%COMP%]{bottom:24px;right:24px}.config-btn[_ngcontent-%COMP%]{top:16px;right:16px}@media(max-width:480px){.config-btn[_ngcontent-%COMP%]{top:8px;right:8px}.fullscreen-btn[_ngcontent-%COMP%]{bottom:16px;right:16px}}[_nghost-%COMP%]     app-page-header{flex-shrink:0}"]})};function pn(n){if(n instanceof De){let a=n.error?.message;return a?`${n.status} ${Array.isArray(a)?a.join(", "):String(a)}`:`${n.status} ${n.statusText||n.message}`}return n instanceof Error?n.message:"erro desconhecido"}export{ci as MapViewComponent};
