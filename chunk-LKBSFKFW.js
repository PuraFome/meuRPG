import{a as ft}from"./chunk-MYQ2VCXX.js";import{a as oe,b as se,c as le}from"./chunk-FDDT7NK5.js";import"./chunk-NI3SGP4Y.js";import"./chunk-WSTWQX6A.js";import{a as gt}from"./chunk-LML7UBRY.js";import"./chunk-B4BXTK7O.js";import{a as ct}from"./chunk-T7NXLVR3.js";import{a as dt}from"./chunk-2CXGHQ4Z.js";import{a as pt,b as ht}from"./chunk-536STK6I.js";import{a as ut,b as _t}from"./chunk-55U2CPJU.js";import"./chunk-MRL5XM53.js";import{a as mt}from"./chunk-64LVDGQ6.js";import{a as ae,b as re}from"./chunk-GUUZ3PJT.js";import"./chunk-WAVAGPFD.js";import{a as lt}from"./chunk-6BYCDSVS.js";import"./chunk-S37IYZOW.js";import"./chunk-JP6UHVMF.js";import"./chunk-R2AY6QGU.js";import"./chunk-YGFD53MG.js";import"./chunk-JBIIEIM2.js";import{a as ot,b as st}from"./chunk-ZGGS5B4G.js";import{C as te,E as rt,J as ie,K as ne,a as Qe,b as Ge,g as Y,l as J,z as ee}from"./chunk-SR2IWCP6.js";import{a as ge}from"./chunk-P2JSLVFL.js";import"./chunk-O26QDNLQ.js";import{a as Ye,b as Je,c as et,d as tt,e as it,f as nt,g as at}from"./chunk-VAQ7FK3L.js";import"./chunk-5WCN6Z4U.js";import{a as Fe,c as We}from"./chunk-MLIBP3GQ.js";import{$ as B,D as ze,E as Xe,N as He,O as ue,Q as $e,R as _e,S as je,T as qe,U as O,V as U,X as Ke,Y as N,Z as Ze,a as Z,o as Le}from"./chunk-BWSTZCQU.js";import{$ as p,Bb as m,Ca as ye,Cb as Ee,Db as Ve,Eb as Re,Fb as pe,Gb as I,Ha as d,Hb as E,Jb as Pe,Kb as De,Ma as ce,Mb as G,Nb as y,Ob as he,Pb as l,Qb as V,Rb as X,Sb as Ae,Ta as S,Ua as Se,Ub as R,V as we,Va as Te,Vb as P,Wb as D,X as ke,Z as L,Zb as q,_b as Oe,d as Ce,ea as u,fa as _,gb as z,hb as g,ic as Ue,jb as f,kb as Ie,lb as me,ma as $,mb as w,na as j,nb as k,ob as b,oc as Ne,pb as r,qa as M,qb as o,qc as K,rb as x,rc as A,sc as T,tc as Be,va as Q,wb as C,zb as h}from"./chunk-N7CELVYH.js";import"./chunk-QXIBXHVB.js";var Pt=["knob"],Dt=["valueIndicatorContainer"];function At(n,a){if(n&1&&(r(0,"div",2,1)(2,"div",5)(3,"span",6),l(4),o()()()),n&2){let e=m();d(4),V(e.valueIndicatorText)}}var Ot=["trackActive"],Ut=["*"];function Nt(n,a){if(n&1&&x(0,"div"),n&2){let e=a.$implicit,t=a.$index,i=m(3);he(e===0?"mdc-slider__tick-mark--active":"mdc-slider__tick-mark--inactive"),G("transform",i._calcTickMarkTransform(t))}}function Bt(n,a){if(n&1&&w(0,Nt,1,4,"div",8,Ie),n&2){let e=m(2);k(e._tickMarks)}}function Ft(n,a){if(n&1&&(r(0,"div",6,1),g(2,Bt,2,0),o()),n&2){let e=m();d(2),f(e._cachedWidth?2:-1)}}function Wt(n,a){if(n&1&&x(0,"mat-slider-visual-thumb",7),n&2){let e=m();b("discrete",e.discrete)("thumbPosition",1)("valueIndicatorText",e.startValueIndicatorText)}}var c=(function(n){return n[n.START=1]="START",n[n.END=2]="END",n})(c||{}),F=(function(n){return n[n.ACTIVE=0]="ACTIVE",n[n.INACTIVE=1]="INACTIVE",n})(F||{}),be=new L("_MatSlider"),bt=new L("_MatSliderThumb"),Lt=new L("_MatSliderRangeThumb"),vt=new L("_MatSliderVisualThumb");var zt=(()=>{class n{_cdr=p(K);_ngZone=p(j);_slider=p(be);_renderer=p(ce);_listenerCleanups;discrete=!1;thumbPosition;valueIndicatorText;_ripple;_knob;_valueIndicatorContainer;_sliderInput;_sliderInputEl;_hoverRippleRef;_focusRippleRef;_activeRippleRef;_isHovered=!1;_isActive=!1;_isValueIndicatorVisible=!1;_hostElement=p(Q).nativeElement;_platform=p(Z);constructor(){}ngAfterViewInit(){let e=this._slider._getInput(this.thumbPosition);e&&(this._ripple.radius=24,this._sliderInput=e,this._sliderInputEl=this._sliderInput._hostElement,this._ngZone.runOutsideAngular(()=>{let t=this._sliderInputEl,i=this._renderer;this._listenerCleanups=[i.listen(t,"pointermove",this._onPointerMove),i.listen(t,"pointerdown",this._onDragStart),i.listen(t,"pointerup",this._onDragEnd),i.listen(t,"pointerleave",this._onMouseLeave),i.listen(t,"focus",this._onFocus),i.listen(t,"blur",this._onBlur)]}))}ngOnDestroy(){this._listenerCleanups?.forEach(e=>e())}_onPointerMove=e=>{if(this._sliderInput._isFocused)return;let t=this._hostElement.getBoundingClientRect(),i=this._slider._isCursorOnSliderThumb(e,t);this._isHovered=i,i?this._showHoverRipple():this._hideRipple(this._hoverRippleRef)};_onMouseLeave=()=>{this._isHovered=!1,this._hideRipple(this._hoverRippleRef)};_onFocus=()=>{this._hideRipple(this._hoverRippleRef),this._showFocusRipple(),this._hostElement.classList.add("mdc-slider__thumb--focused")};_onBlur=()=>{this._isActive||this._hideRipple(this._focusRippleRef),this._isHovered&&this._showHoverRipple(),this._hostElement.classList.remove("mdc-slider__thumb--focused")};_onDragStart=e=>{e.button===0&&(this._isActive=!0,this._showActiveRipple())};_onDragEnd=()=>{this._isActive=!1,this._hideRipple(this._activeRippleRef),this._sliderInput._isFocused||this._hideRipple(this._focusRippleRef),this._platform.SAFARI&&this._showHoverRipple()};_showHoverRipple(){this._isShowingRipple(this._hoverRippleRef)||(this._hoverRippleRef=this._showRipple({enterDuration:0,exitDuration:0}),this._hoverRippleRef?.element.classList.add("mat-mdc-slider-hover-ripple"))}_showFocusRipple(){this._isShowingRipple(this._focusRippleRef)||(this._focusRippleRef=this._showRipple({enterDuration:0,exitDuration:0},!0),this._focusRippleRef?.element.classList.add("mat-mdc-slider-focus-ripple"))}_showActiveRipple(){this._isShowingRipple(this._activeRippleRef)||(this._activeRippleRef=this._showRipple({enterDuration:225,exitDuration:400}),this._activeRippleRef?.element.classList.add("mat-mdc-slider-active-ripple"))}_isShowingRipple(e){return e?.state===ue.FADING_IN||e?.state===ue.VISIBLE}_showRipple(e,t){if(!this._slider.disabled&&(this._showValueIndicator(),this._slider._isRange&&this._slider._getThumb(this.thumbPosition===c.START?c.END:c.START)._showValueIndicator(),!(this._slider._globalRippleOptions?.disabled&&!t)))return this._ripple.launch({animation:this._slider._noopAnimations?{enterDuration:0,exitDuration:0}:e,centered:!0,persistent:!0})}_hideRipple(e){if(e?.fadeOut(),this._isShowingAnyRipple())return;this._slider._isRange||this._hideValueIndicator();let t=this._getSibling();t._isShowingAnyRipple()||(this._hideValueIndicator(),t._hideValueIndicator())}_showValueIndicator(){this._hostElement.classList.add("mdc-slider__thumb--with-indicator")}_hideValueIndicator(){this._hostElement.classList.remove("mdc-slider__thumb--with-indicator")}_getSibling(){return this._slider._getThumb(this.thumbPosition===c.START?c.END:c.START)}_getValueIndicatorContainer(){return this._valueIndicatorContainer?.nativeElement}_getKnob(){return this._knob.nativeElement}_isShowingAnyRipple(){return this._isShowingRipple(this._hoverRippleRef)||this._isShowingRipple(this._focusRippleRef)||this._isShowingRipple(this._activeRippleRef)}static \u0275fac=function(t){return new(t||n)};static \u0275cmp=S({type:n,selectors:[["mat-slider-visual-thumb"]],viewQuery:function(t,i){if(t&1&&pe(_e,5)(Pt,5)(Dt,5),t&2){let s;I(s=E())&&(i._ripple=s.first),I(s=E())&&(i._knob=s.first),I(s=E())&&(i._valueIndicatorContainer=s.first)}},hostAttrs:[1,"mdc-slider__thumb","mat-mdc-slider-visual-thumb"],inputs:{discrete:"discrete",thumbPosition:"thumbPosition",valueIndicatorText:"valueIndicatorText"},features:[q([{provide:vt,useExisting:n}])],decls:4,vars:2,consts:[["knob",""],["valueIndicatorContainer",""],[1,"mdc-slider__value-indicator-container"],[1,"mdc-slider__thumb-knob"],["matRipple","",1,"mat-focus-indicator",3,"matRippleDisabled"],[1,"mdc-slider__value-indicator"],[1,"mdc-slider__value-indicator-text"]],template:function(t,i){t&1&&(g(0,At,5,1,"div",2),x(1,"div",3,0)(3,"div",4)),t&2&&(f(i.discrete?0:-1),d(3),b("matRippleDisabled",!0))},dependencies:[_e],styles:[`.mat-mdc-slider-visual-thumb .mat-ripple {
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
`],encapsulation:2,changeDetection:0})}return n})(),xt=(()=>{class n{_ngZone=p(j);_cdr=p(K);_elementRef=p(Q);_dir=p(ze,{optional:!0});_globalRippleOptions=p($e,{optional:!0});_trackActive;_thumbs;_input;_inputs;get disabled(){return this._disabled}set disabled(e){this._disabled=e;let t=this._getInput(c.END),i=this._getInput(c.START);t&&(t.disabled=this._disabled),i&&(i.disabled=this._disabled)}_disabled=!1;get discrete(){return this._discrete}set discrete(e){this._discrete=e,this._updateValueIndicatorUIs()}_discrete=!1;get showTickMarks(){return this._showTickMarks}set showTickMarks(e){this._showTickMarks=e,this._hasViewInitialized&&(this._updateTickMarkUI(),this._updateTickMarkTrackUI())}_showTickMarks=!1;get min(){return this._min}set min(e){let t=e==null||isNaN(e)?this._min:e;this._min!==t&&this._updateMin(t)}_min=0;color;disableRipple=!1;_updateMin(e){let t=this._min;this._min=e,this._isRange?this._updateMinRange({old:t,new:e}):this._updateMinNonRange(e),this._onMinMaxOrStepChange()}_updateMinRange(e){let t=this._getInput(c.END),i=this._getInput(c.START),s=t.value,v=i.value;i.min=e.new,t.min=Math.max(e.new,i.value),i.max=Math.min(t.max,t.value),i._updateWidthInactive(),t._updateWidthInactive(),e.new<e.old?this._onTranslateXChangeBySideEffect(t,i):this._onTranslateXChangeBySideEffect(i,t),s!==t.value&&this._onValueChange(t),v!==i.value&&this._onValueChange(i)}_updateMinNonRange(e){let t=this._getInput(c.END);if(t){let i=t.value;t.min=e,t._updateThumbUIByValue(),this._updateTrackUI(t),i!==t.value&&this._onValueChange(t)}}get max(){return this._max}set max(e){let t=e==null||isNaN(e)?this._max:e;this._max!==t&&this._updateMax(t)}_max=100;_updateMax(e){let t=this._max;this._max=e,this._isRange?this._updateMaxRange({old:t,new:e}):this._updateMaxNonRange(e),this._onMinMaxOrStepChange()}_updateMaxRange(e){let t=this._getInput(c.END),i=this._getInput(c.START),s=t.value,v=i.value;t.max=e.new,i.max=Math.min(e.new,t.value),t.min=i.value,t._updateWidthInactive(),i._updateWidthInactive(),e.new>e.old?this._onTranslateXChangeBySideEffect(i,t):this._onTranslateXChangeBySideEffect(t,i),s!==t.value&&this._onValueChange(t),v!==i.value&&this._onValueChange(i)}_updateMaxNonRange(e){let t=this._getInput(c.END);if(t){let i=t.value;t.max=e,t._updateThumbUIByValue(),this._updateTrackUI(t),i!==t.value&&this._onValueChange(t)}}get step(){return this._step}set step(e){let t=isNaN(e)?this._step:e;this._step!==t&&this._updateStep(t)}_step=1;_updateStep(e){this._step=e,this._isRange?this._updateStepRange():this._updateStepNonRange(),this._onMinMaxOrStepChange()}_updateStepRange(){let e=this._getInput(c.END),t=this._getInput(c.START),i=e.value,s=t.value,v=t.value;e.min=this._min,t.max=this._max,e.step=this._step,t.step=this._step,this._platform.SAFARI&&(e.value=e.value,t.value=t.value),e.min=Math.max(this._min,t.value),t.max=Math.min(this._max,e.value),t._updateWidthInactive(),e._updateWidthInactive(),e.value<v?this._onTranslateXChangeBySideEffect(t,e):this._onTranslateXChangeBySideEffect(e,t),i!==e.value&&this._onValueChange(e),s!==t.value&&this._onValueChange(t)}_updateStepNonRange(){let e=this._getInput(c.END);if(e){let t=e.value;e.step=this._step,this._platform.SAFARI&&(e.value=e.value),e._updateThumbUIByValue(),t!==e.value&&this._onValueChange(e)}}displayWith=e=>`${e}`;_tickMarks;_noopAnimations=He();_resizeObserver=null;_cachedWidth;_cachedLeft;_rippleRadius=24;startValueIndicatorText="";endValueIndicatorText="";_endThumbTransform;_startThumbTransform;_isRange=!1;_isRtl=Ue(()=>this._dir?.valueSignal()==="rtl");_hasViewInitialized=!1;_tickMarkTrackWidth=0;_hasAnimation=!1;_resizeTimer=null;_platform=p(Z);constructor(){p(Le).load(je);let e=this._isRtl();Be(()=>{let t=this._isRtl();t!==e&&(e=t,this._isRange?this._onDirChangeRange():this._onDirChangeNonRange(),this._updateTickMarkUI())})}_knobRadius=8;_inputPadding;ngAfterViewInit(){this._platform.isBrowser&&this._updateDimensions();let e=this._getInput(c.END),t=this._getInput(c.START);this._isRange=!!e&&!!t,this._cdr.detectChanges();let i=this._getThumb(c.END);this._rippleRadius=i._ripple.radius,this._inputPadding=this._rippleRadius-this._knobRadius,this._isRange?this._initUIRange(e,t):this._initUINonRange(e),this._updateTrackUI(e),this._updateTickMarkUI(),this._updateTickMarkTrackUI(),this._observeHostResize(),this._cdr.detectChanges()}_initUINonRange(e){e.initProps(),e.initUI(),this._updateValueIndicatorUI(e),this._hasViewInitialized=!0,e._updateThumbUIByValue()}_initUIRange(e,t){e.initProps(),e.initUI(),t.initProps(),t.initUI(),e._updateMinMax(),t._updateMinMax(),e._updateStaticStyles(),t._updateStaticStyles(),this._updateValueIndicatorUIs(),this._hasViewInitialized=!0,e._updateThumbUIByValue(),t._updateThumbUIByValue()}ngOnDestroy(){this._resizeObserver?.disconnect(),this._resizeObserver=null}_onDirChangeRange(){let e=this._getInput(c.END),t=this._getInput(c.START);e._setIsLeftThumb(),t._setIsLeftThumb(),e.translateX=e._calcTranslateXByValue(),t.translateX=t._calcTranslateXByValue(),e._updateStaticStyles(),t._updateStaticStyles(),e._updateWidthInactive(),t._updateWidthInactive(),e._updateThumbUIByValue(),t._updateThumbUIByValue()}_onDirChangeNonRange(){this._getInput(c.END)._updateThumbUIByValue()}_observeHostResize(){typeof ResizeObserver>"u"||!ResizeObserver||this._ngZone.runOutsideAngular(()=>{this._resizeObserver=new ResizeObserver(()=>{this._isActive()||(this._resizeTimer&&clearTimeout(this._resizeTimer),this._onResize())}),this._resizeObserver.observe(this._elementRef.nativeElement)})}_isActive(){return this._getThumb(c.START)._isActive||this._getThumb(c.END)._isActive}_getValue(e=c.END){let t=this._getInput(e);return t?t.value:this.min}_skipUpdate(){return!!(this._getInput(c.START)?._skipUIUpdate||this._getInput(c.END)?._skipUIUpdate)}_updateDimensions(){this._cachedWidth=this._elementRef.nativeElement.offsetWidth,this._cachedLeft=this._elementRef.nativeElement.getBoundingClientRect().left}_setTrackActiveStyles(e){let t=this._trackActive.nativeElement.style;t.left=e.left,t.right=e.right,t.transformOrigin=e.transformOrigin,t.transform=e.transform}_calcTickMarkTransform(e){let t=e*(this._tickMarkTrackWidth/(this._tickMarks.length-1));return`translateX(${this._isRtl()?this._cachedWidth-6-t:t}px)`}_onTranslateXChange(e){this._hasViewInitialized&&(this._updateThumbUI(e),this._updateTrackUI(e),this._updateOverlappingThumbUI(e))}_onTranslateXChangeBySideEffect(e,t){this._hasViewInitialized&&(e._updateThumbUIByValue(),t._updateThumbUIByValue())}_onValueChange(e){this._hasViewInitialized&&(this._updateValueIndicatorUI(e),this._updateTickMarkUI(),this._cdr.detectChanges())}_onMinMaxOrStepChange(){this._hasViewInitialized&&(this._updateTickMarkUI(),this._updateTickMarkTrackUI(),this._cdr.markForCheck())}_onResize(){if(this._hasViewInitialized){if(this._updateDimensions(),this._isRange){let e=this._getInput(c.END),t=this._getInput(c.START);e._updateThumbUIByValue(),t._updateThumbUIByValue(),e._updateStaticStyles(),t._updateStaticStyles(),e._updateMinMax(),t._updateMinMax(),e._updateWidthInactive(),t._updateWidthInactive()}else{let e=this._getInput(c.END);e&&e._updateThumbUIByValue()}this._updateTickMarkUI(),this._updateTickMarkTrackUI(),this._cdr.detectChanges()}}_thumbsOverlap=!1;_areThumbsOverlapping(){let e=this._getInput(c.START),t=this._getInput(c.END);return!e||!t?!1:t.translateX-e.translateX<20}_updateOverlappingThumbClassNames(e){let t=e.getSibling(),i=this._getThumb(e.thumbPosition);this._getThumb(t.thumbPosition)._hostElement.classList.remove("mdc-slider__thumb--top"),i._hostElement.classList.toggle("mdc-slider__thumb--top",this._thumbsOverlap)}_updateOverlappingThumbUI(e){!this._isRange||this._skipUpdate()||this._thumbsOverlap!==this._areThumbsOverlapping()&&(this._thumbsOverlap=!this._thumbsOverlap,this._updateOverlappingThumbClassNames(e))}_updateThumbUI(e){if(this._skipUpdate())return;let t=this._getThumb(e.thumbPosition===c.END?c.END:c.START);t._hostElement.style.transform=`translateX(${e.translateX}px)`}_updateValueIndicatorUI(e){if(this._skipUpdate())return;let t=this.displayWith(e.value);if(this._hasViewInitialized?e._valuetext.set(t):e._hostElement.setAttribute("aria-valuetext",t),this.discrete){e.thumbPosition===c.START?this.startValueIndicatorText=t:this.endValueIndicatorText=t;let i=this._getThumb(e.thumbPosition);t.length<3?i._hostElement.classList.add("mdc-slider__thumb--short-value"):i._hostElement.classList.remove("mdc-slider__thumb--short-value")}}_updateValueIndicatorUIs(){let e=this._getInput(c.END),t=this._getInput(c.START);e&&this._updateValueIndicatorUI(e),t&&this._updateValueIndicatorUI(t)}_updateTickMarkTrackUI(){if(!this.showTickMarks||this._skipUpdate())return;let e=this._step&&this._step>0?this._step:1,i=(Math.floor(this.max/e)*e-this.min)/(this.max-this.min);this._tickMarkTrackWidth=(this._cachedWidth-6)*i}_updateTrackUI(e){this._skipUpdate()||(this._isRange?this._updateTrackUIRange(e):this._updateTrackUINonRange(e))}_updateTrackUIRange(e){let t=e.getSibling();if(!t||!this._cachedWidth)return;let i=Math.abs(t.translateX-e.translateX)/this._cachedWidth;e._isLeftThumb&&this._cachedWidth?this._setTrackActiveStyles({left:"auto",right:`${this._cachedWidth-t.translateX}px`,transformOrigin:"right",transform:`scaleX(${i})`}):this._setTrackActiveStyles({left:`${t.translateX}px`,right:"auto",transformOrigin:"left",transform:`scaleX(${i})`})}_updateTrackUINonRange(e){this._isRtl()?this._setTrackActiveStyles({left:"auto",right:"0px",transformOrigin:"right",transform:`scaleX(${1-e.fillPercentage})`}):this._setTrackActiveStyles({left:"0px",right:"auto",transformOrigin:"left",transform:`scaleX(${e.fillPercentage})`})}_updateTickMarkUI(){if(!this.showTickMarks||this.step===void 0||this.min===void 0||this.max===void 0)return;let e=this.step>0?this.step:1;this._isRange?this._updateTickMarkUIRange(e):this._updateTickMarkUINonRange(e)}_updateTickMarkUINonRange(e){let t=this._getValue(),i=Math.max(Math.round((t-this.min)/e),0)+1,s=Math.max(Math.round((this.max-t)/e),0)-1;this._isRtl()?i++:s++,this._tickMarks=Array(i).fill(F.ACTIVE).concat(Array(s).fill(F.INACTIVE))}_updateTickMarkUIRange(e){let t=this._getValue(),i=this._getValue(c.START),s=Math.max(Math.round((i-this.min)/e),0),v=Math.max(Math.round((t-i)/e)+1,0),W=Math.max(Math.round((this.max-t)/e),0);this._tickMarks=Array(s).fill(F.INACTIVE).concat(Array(v).fill(F.ACTIVE),Array(W).fill(F.INACTIVE))}_getInput(e){if(e===c.END&&this._input)return this._input;if(this._inputs?.length)return e===c.START?this._inputs.first:this._inputs.last}_getThumb(e){return e===c.END?this._thumbs?.last:this._thumbs?.first}_setTransition(e){this._hasAnimation=!this._platform.IOS&&e&&!this._noopAnimations,this._elementRef.nativeElement.classList.toggle("mat-mdc-slider-with-animation",this._hasAnimation)}_isCursorOnSliderThumb(e,t){let i=t.width/2,s=t.x+i,v=t.y+i,W=e.clientX-s,xe=e.clientY-v;return Math.pow(W,2)+Math.pow(xe,2)<Math.pow(i,2)}static \u0275fac=function(t){return new(t||n)};static \u0275cmp=S({type:n,selectors:[["mat-slider"]],contentQueries:function(t,i,s){if(t&1&&Re(s,bt,5)(s,Lt,4),t&2){let v;I(v=E())&&(i._input=v.first),I(v=E())&&(i._inputs=v)}},viewQuery:function(t,i){if(t&1&&pe(Ot,5)(vt,5),t&2){let s;I(s=E())&&(i._trackActive=s.first),I(s=E())&&(i._thumbs=s)}},hostAttrs:[1,"mat-mdc-slider","mdc-slider"],hostVars:12,hostBindings:function(t,i){t&2&&(he("mat-"+(i.color||"primary")),y("mdc-slider--range",i._isRange)("mdc-slider--disabled",i.disabled)("mdc-slider--discrete",i.discrete)("mdc-slider--tick-marks",i.showTickMarks)("_mat-animation-noopable",i._noopAnimations))},inputs:{disabled:[2,"disabled","disabled",A],discrete:[2,"discrete","discrete",A],showTickMarks:[2,"showTickMarks","showTickMarks",A],min:[2,"min","min",T],color:"color",disableRipple:[2,"disableRipple","disableRipple",A],max:[2,"max","max",T],step:[2,"step","step",T],displayWith:"displayWith"},exportAs:["matSlider"],features:[q([{provide:be,useExisting:n}])],ngContentSelectors:Ut,decls:9,vars:5,consts:[["trackActive",""],["tickMarkContainer",""],[1,"mdc-slider__track"],[1,"mdc-slider__track--inactive"],[1,"mdc-slider__track--active"],[1,"mdc-slider__track--active_fill"],[1,"mdc-slider__tick-marks"],[3,"discrete","thumbPosition","valueIndicatorText"],[3,"class","transform"]],template:function(t,i){t&1&&(Ee(),Ve(0),r(1,"div",2),x(2,"div",3),r(3,"div",4),x(4,"div",5,0),o(),g(6,Ft,3,1,"div",6),o(),g(7,Wt,1,3,"mat-slider-visual-thumb",7),x(8,"mat-slider-visual-thumb",7)),t&2&&(d(6),f(i.showTickMarks?6:-1),d(),f(i._isRange?7:-1),d(),b("discrete",i.discrete)("thumbPosition",2)("valueIndicatorText",i.endValueIndicatorText))},dependencies:[zt],styles:[`.mdc-slider__track {
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
`],encapsulation:2,changeDetection:0})}return n})();var Xt={provide:Qe,useExisting:we(()=>ve),multi:!0};var ve=(()=>{class n{_ngZone=p(j);_elementRef=p(Q);_cdr=p(K);_slider=p(be);_platform=p(Z);_listenerCleanups;get value(){return T(this._hostElement.value,0)}set value(e){e===null&&(e=this._getDefaultValue()),e=isNaN(e)?0:e;let t=e+"";if(!this._hasSetInitialValue){this._initialValue=t;return}this._isActive||this._setValue(t)}_setValue(e){this._hostElement.value=e,this._updateThumbUIByValue(),this._slider._onValueChange(this),this._cdr.detectChanges(),this._slider._cdr.markForCheck()}valueChange=new $;dragStart=new $;dragEnd=new $;get translateX(){return this._slider.min>=this._slider.max?(this._translateX=this._tickMarkOffset,this._translateX):(this._translateX===void 0&&(this._translateX=this._calcTranslateXByValue()),this._translateX)}set translateX(e){this._translateX=e}_translateX;thumbPosition=c.END;get min(){return T(this._hostElement.min,0)}set min(e){this._hostElement.min=e+"",this._cdr.detectChanges()}get max(){return T(this._hostElement.max,0)}set max(e){this._hostElement.max=e+"",this._cdr.detectChanges()}get step(){return T(this._hostElement.step,0)}set step(e){this._hostElement.step=e+"",this._cdr.detectChanges()}get disabled(){return A(this._hostElement.disabled)}set disabled(e){this._hostElement.disabled=e,this._cdr.detectChanges(),this._slider.disabled!==this.disabled&&(this._slider.disabled=this.disabled)}get percentage(){return this._slider.min>=this._slider.max?this._slider._isRtl()?1:0:(this.value-this._slider.min)/(this._slider.max-this._slider.min)}get fillPercentage(){return this._slider._cachedWidth?this._translateX===0?0:this.translateX/this._slider._cachedWidth:this._slider._isRtl()?1:0}_hostElement=this._elementRef.nativeElement;_valuetext=M("");_knobRadius=8;_tickMarkOffset=3;_isActive=!1;_isFocused=!1;_setIsFocused(e){this._isFocused=e}_hasSetInitialValue=!1;_initialValue;_formControl;_destroyed=new Ce;_skipUIUpdate=!1;_onChangeFn;_onTouchedFn=()=>{};_isControlInitialized=!1;constructor(){let e=p(ce);this._ngZone.runOutsideAngular(()=>{this._listenerCleanups=[e.listen(this._hostElement,"pointerdown",this._onPointerDown.bind(this)),e.listen(this._hostElement,"pointermove",this._onPointerMove.bind(this)),e.listen(this._hostElement,"pointerup",this._onPointerUp.bind(this))]})}ngOnDestroy(){this._listenerCleanups.forEach(e=>e()),this._destroyed.next(),this._destroyed.complete(),this.dragStart.complete(),this.dragEnd.complete()}initProps(){this._updateWidthInactive(),this.disabled!==this._slider.disabled&&(this._slider.disabled=!0),this.step=this._slider.step,this.min=this._slider.min,this.max=this._slider.max,this._initValue()}initUI(){this._updateThumbUIByValue()}_initValue(){this._hasSetInitialValue=!0,this._initialValue===void 0?this.value=this._getDefaultValue():(this._hostElement.value=this._initialValue,this._updateThumbUIByValue(),this._slider._onValueChange(this),this._cdr.detectChanges())}_getDefaultValue(){return this.min}_onBlur(){this._setIsFocused(!1),this._onTouchedFn()}_onFocus(){this._slider._setTransition(!1),this._slider._updateTrackUI(this),this._setIsFocused(!0)}_onChange(){this.valueChange.emit(this.value),this._isActive&&this._updateThumbUIByValue({withAnimation:!0})}_onInput(){this._onChangeFn?.(this.value),(this._slider.step||!this._isActive)&&this._updateThumbUIByValue({withAnimation:!0}),this._slider._onValueChange(this)}_onNgControlValueChange(){(!this._isActive||!this._isFocused)&&(this._slider._onValueChange(this),this._updateThumbUIByValue()),this._slider.disabled=this._formControl.disabled}_onPointerDown(e){if(!(this.disabled||e.button!==0)){if(this._platform.IOS){let t=this._slider._isCursorOnSliderThumb(e,this._slider._getThumb(this.thumbPosition)._hostElement.getBoundingClientRect());this._isActive=t,this._updateWidthActive(),this._slider._updateDimensions();return}this._isActive=!0,this._setIsFocused(!0),this._updateWidthActive(),this._slider._updateDimensions(),this._slider.step||this._updateThumbUIByPointerEvent(e,{withAnimation:!0}),this.disabled||(this._handleValueCorrection(e),this.dragStart.emit({source:this,parent:this._slider,value:this.value}))}}_handleValueCorrection(e){this._skipUIUpdate=!0,setTimeout(()=>{this._skipUIUpdate=!1,this._fixValue(e)},0)}_fixValue(e){let t=e.clientX-this._slider._cachedLeft,i=this._slider._cachedWidth,s=this._slider.step===0?1:this._slider.step,v=Math.floor((this._slider.max-this._slider.min)/s),W=this._slider._isRtl()?1-t/i:t/i,Vt=Math.round(W*v)/v*(this._slider.max-this._slider.min)+this._slider.min,Me=Math.round(Vt/s)*s,Rt=this.value;if(Me===Rt){this._slider._onValueChange(this),this._slider.step>0?this._updateThumbUIByValue():this._updateThumbUIByPointerEvent(e,{withAnimation:this._slider._hasAnimation});return}this.value=Me,this.valueChange.emit(this.value),this._onChangeFn?.(this.value),this._slider._onValueChange(this),this._slider.step>0?this._updateThumbUIByValue():this._updateThumbUIByPointerEvent(e,{withAnimation:this._slider._hasAnimation})}_onPointerMove(e){!this._slider.step&&this._isActive&&this._updateThumbUIByPointerEvent(e)}_onPointerUp(){this._isActive&&(this._isActive=!1,this._platform.SAFARI&&this._setIsFocused(!1),this.dragEnd.emit({source:this,parent:this._slider,value:this.value}),setTimeout(()=>this._updateWidthInactive(),this._platform.IOS?10:0))}_clamp(e){let t=this._tickMarkOffset,i=this._slider._cachedWidth-this._tickMarkOffset;return Math.max(Math.min(e,i),t)}_calcTranslateXByValue(){return this._slider._isRtl()?(1-this.percentage)*(this._slider._cachedWidth-this._tickMarkOffset*2)+this._tickMarkOffset:this.percentage*(this._slider._cachedWidth-this._tickMarkOffset*2)+this._tickMarkOffset}_calcTranslateXByPointerEvent(e){return e.clientX-this._slider._cachedLeft}_updateWidthActive(){}_updateWidthInactive(){this._hostElement.style.padding=`0 ${this._slider._inputPadding}px`,this._hostElement.style.width=`calc(100% + ${this._slider._inputPadding-this._tickMarkOffset*2}px)`,this._hostElement.style.left=`-${this._slider._rippleRadius-this._tickMarkOffset}px`}_updateThumbUIByValue(e){this.translateX=this._clamp(this._calcTranslateXByValue()),this._updateThumbUI(e)}_updateThumbUIByPointerEvent(e,t){this.translateX=this._clamp(this._calcTranslateXByPointerEvent(e)),this._updateThumbUI(t)}_updateThumbUI(e){this._slider._setTransition(!!e?.withAnimation),this._slider._onTranslateXChange(this)}writeValue(e){(this._isControlInitialized||e!==null)&&(this.value=e)}registerOnChange(e){this._onChangeFn=e,this._isControlInitialized=!0}registerOnTouched(e){this._onTouchedFn=e}setDisabledState(e){this.disabled=e}focus(){this._hostElement.focus()}blur(){this._hostElement.blur()}static \u0275fac=function(t){return new(t||n)};static \u0275dir=Te({type:n,selectors:[["input","matSliderThumb",""]],hostAttrs:["type","range",1,"mdc-slider__input"],hostVars:1,hostBindings:function(t,i){t&1&&h("change",function(){return i._onChange()})("input",function(){return i._onInput()})("blur",function(){return i._onBlur()})("focus",function(){return i._onFocus()}),t&2&&z("aria-valuetext",i._valuetext())},inputs:{value:[2,"value","value",T]},outputs:{valueChange:"valueChange",dragStart:"dragStart",dragEnd:"dragEnd"},exportAs:["matSliderThumb"],features:[q([Xt,{provide:bt,useExisting:n}])]})}return n})();var Mt=(()=>{class n{static \u0275fac=function(t){return new(t||n)};static \u0275mod=Se({type:n});static \u0275inj=ke({imports:[qe,Xe]})}return n})();var de=class n{mapService=p(ae);dungeonService=p(re);toggleGrid(){this.mapService.toggleGrid()}toggleFogOfWar(){this.mapService.toggleFogOfWar()}toggleMarkers(){this.mapService.toggleMarkers()}toggleSubmapPins(){this.mapService.toggleSubmapPins()}toggleDungeon(){this.dungeonService.toggle()}clearDungeon(){this.dungeonService.clear()}setFogOpacity(a){a!==null&&this.mapService.setFogOpacity(a)}resetFog(){this.mapService.resetFogOfWar()}static \u0275fac=function(e){return new(e||n)};static \u0275cmp=S({type:n,selectors:[["app-map-config-panel"]],decls:27,vars:0,consts:[[1,"config-panel"],[1,"panel-title"],[1,"checkbox-group"],[3,"change"],[1,"slider-section"],[1,"slider-label"],["min","0","max","1","step","0.1"],["matSliderThumb","",3,"valueChange"],["mat-stroked-button","",1,"reset-btn",3,"click"],["mat-stroked-button","",1,"reset-btn","danger",3,"click"]],template:function(e,t){e&1&&(r(0,"div",0)(1,"h3",1),l(2,"Camadas"),o(),r(3,"div",2)(4,"mat-checkbox",3),h("change",function(){return t.toggleGrid()}),l(5," Grade "),o(),r(6,"mat-checkbox",3),h("change",function(){return t.toggleFogOfWar()}),l(7," N\xE9voa da Guerra "),o(),r(8,"mat-checkbox",3),h("change",function(){return t.toggleMarkers()}),l(9," Pontos de Interesse "),o(),r(10,"mat-checkbox",3),h("change",function(){return t.toggleSubmapPins()}),l(11," Pins de Submapa "),o(),r(12,"mat-checkbox",3),h("change",function(){return t.toggleDungeon()}),l(13," Desenho da Masmorra "),o()(),r(14,"div",4)(15,"label",5),l(16,"Opacidade da N\xE9voa"),o(),r(17,"mat-slider",6)(18,"input",7),h("valueChange",function(s){return t.setFogOpacity(s)}),o()()(),r(19,"button",8),h("click",function(){return t.resetFog()}),r(20,"mat-icon"),l(21,"refresh"),o(),l(22," Reset Fog of War "),o(),r(23,"button",9),h("click",function(){return t.clearDungeon()}),r(24,"mat-icon"),l(25,"delete_sweep"),o(),l(26," Limpar desenho da masmorra "),o()())},dependencies:[ht,pt,Mt,xt,ve,B,N,U,O],styles:["[_nghost-%COMP%]{position:absolute;right:16px;top:64px;z-index:10;width:min(240px,100vw - 32px)}.config-panel[_ngcontent-%COMP%]{background:#1e1e1e;border:1px solid rgba(255,255,255,.12);border-radius:12px;padding:16px;display:flex;flex-direction:column;gap:12px;box-shadow:0 4px 24px #0006}.panel-title[_ngcontent-%COMP%]{margin:0;font-size:.875rem;font-weight:500;color:#ffffffde;text-transform:uppercase;letter-spacing:.5px}.checkbox-group[_ngcontent-%COMP%]{display:flex;flex-direction:column;gap:8px}[_nghost-%COMP%]     .checkbox-group .mdc-checkbox{flex-shrink:0}.slider-section[_ngcontent-%COMP%]{display:flex;flex-direction:column;gap:4px}.slider-label[_ngcontent-%COMP%]{font-size:.75rem;color:#fff9}.reset-btn[_ngcontent-%COMP%]{width:100%}.reset-btn.danger[_ngcontent-%COMP%]{color:#ff8a80;border-color:#ff8a8066}"]})};var Gt=()=>["image/"],qt=(n,a)=>a.id,Kt=(n,a)=>a.value;function Zt(n,a){if(n&1&&(r(0,"mat-option",7),l(1),o()),n&2){let e=a.$implicit;b("value",e.id),d(),V(e.name)}}function Yt(n,a){if(n&1&&(r(0,"mat-option",7),l(1),o()),n&2){let e=a.$implicit;b("value",e.value),d(),V(e.label)}}function Jt(n,a){if(n&1&&(r(0,"div",20),x(1,"img",21),r(2,"span"),l(3),o()()),n&2){let e=m(2);d(),b("src",e.submapImage().dataUrl,ye),d(2),Ae("",e.submapImage().width," \xD7 ",e.submapImage().height," px")}}function ei(n,a){if(n&1){let e=C();r(0,"div",9)(1,"mat-form-field",2)(2,"mat-label"),l(3,"Nome do novo submapa"),o(),r(4,"input",18),D("ngModelChange",function(i){u(e);let s=m();return P(s.newSubmapName,i)||(s.newSubmapName=i),_(i)}),o()(),r(5,"mat-form-field",2)(6,"mat-label"),l(7,"Tipo do novo submapa"),o(),r(8,"mat-select",5),D("ngModelChange",function(i){u(e);let s=m();return P(s.newSubmapKind,i)||(s.newSubmapKind=i),_(i)}),w(9,Yt,2,2,"mat-option",7,Kt),o()(),r(11,"label",10),l(12,"Imagem do submapa (opcional)"),o(),r(13,"app-file-upload",19),h("fileChange",function(i){u(e);let s=m();return _(s.onSubmapImageSelected(i))}),o(),g(14,Jt,4,3,"div",20),o()}if(n&2){let e=m();d(4),R("ngModel",e.newSubmapName),d(4),R("ngModel",e.newSubmapKind),d(),k(e.submapKinds),d(4),b("acceptedTypes",Oe(6,Gt))("maxSize",15*1024*1024)("showPreview",!1),d(),f(e.submapImage()?14:-1)}}function ti(n,a){if(n&1){let e=C();r(0,"button",22),h("click",function(){let i=u(e).$implicit,s=m();return _(s.selectedColor=i)}),o()}if(n&2){let e=a.$implicit,t=m();G("background",e),y("selected",t.selectedColor===e),z("aria-label","Cor "+e)}}function ii(n,a){if(n&1){let e=C();r(0,"button",23),h("click",function(){let i=u(e).$implicit,s=m();return _(s.selectedIcon=i)}),r(1,"span",24),l(2),o()()}if(n&2){let e=a.$implicit,t=m();y("selected",t.selectedIcon===e),z("aria-label","\xCDcone "+e),d(2),V(e)}}function ni(n,a){if(n&1){let e=C();r(0,"button",27),h("click",function(){u(e);let i=m(2);return _(i.onOpen())}),r(1,"mat-icon"),l(2,"open_in_new"),o(),l(3," Abrir mapa "),o()}}function ai(n,a){if(n&1){let e=C();r(0,"button",25),h("click",function(){u(e);let i=m();return _(i.onDelete())}),r(1,"mat-icon"),l(2,"delete"),o(),l(3," Excluir "),o(),g(4,ni,4,0,"button",26)}if(n&2){let e=m();d(4),f(e.existing.targetMapId?4:-1)}}var yt="__new__",St=["#7c4dff","#e53935","#ff6d00","#ffd600","#00c853","#2979ff","#00bcd4","#ff4081","#6d4c41","#78909c"],Tt=["\u{1F37A}","\u{1F377}","\u{1F356}","\u{1F6CF}\uFE0F","\u{1F3F0}","\u{1F3DB}\uFE0F","\u26EA","\u{1F3E0}","\u{1F3D8}\uFE0F","\u{1F6D2}","\u2692\uFE0F","\u{1F9D9}","\u{1F47A}","\u{1F479}","\u{1F9CC}","\u{1F409}","\u{1F480}","\u{1F9DF}","\u{1F577}\uFE0F","\u{1F43A}","\u{1F987}","\u2694\uFE0F","\u{1F6E1}\uFE0F","\u{1F3F9}","\u{1F525}","\u{1F573}\uFE0F","\u{1F5DD}\uFE0F","\u{1F48E}","\u{1FA99}","\u{1F9EA}","\u{1F4DC}","\u{1F6AA}","\u2693","\u26F5","\u{1F332}","\u26F0\uFE0F","\u{1F3D5}\uFE0F","\u{1F56F}\uFE0F","\u{1FAA6}","\u{1F9ED}"],ri=[{value:"dungeon",label:"Masmorra"},{value:"city",label:"Cidade"},{value:"local",label:"Local (loja, taverna...)"},{value:"world",label:"Mundo / Regi\xE3o"}],H=class n{dialogRef=p(Ye);data=p(Je);colors=St;icons=Tt;submapKinds=ri;newSubmapValue=yt;existing=this.data.existing;availableMaps=this.data.availableMaps;label=this.data.existing?.label??"";description=this.data.existing?.description??"";destination=this.data.existing?.targetMapId??"";newSubmapName="";newSubmapKind="dungeon";submapImage=M(null);selectedColor=this.data.existing?.color??St[0];selectedIcon=this.data.existing?.icon??Tt[0];onCancel(){this.dialogRef.close(null)}onDelete(){this.dialogRef.close({action:"delete"})}onSubmapImageSelected(a){let e=new FileReader;e.onload=()=>{let t=e.result,i=new Image;i.onload=()=>{this.submapImage.set({dataUrl:t,width:i.naturalWidth,height:i.naturalHeight})},i.src=t},e.readAsDataURL(a)}onOpen(){this.dialogRef.close({action:"open"})}onSave(){if(!this.label.trim())return;let a=this.destination===yt;if(a&&!this.newSubmapName.trim())return;let e={id:this.existing?.id??crypto.randomUUID(),x:this.data.x,y:this.data.y,label:this.label.trim(),description:this.description.trim()||void 0,icon:this.selectedIcon,color:this.selectedColor,targetMapId:!a&&this.destination?this.destination:void 0};this.dialogRef.close({action:"save",marker:e,newSubmap:a?{name:this.newSubmapName.trim(),kind:this.newSubmapKind,image:this.submapImage()??void 0}:void 0})}static \u0275fac=function(e){return new(e||n)};static \u0275cmp=S({type:n,selectors:[["app-poi-dialog"]],decls:45,vars:8,consts:[["mat-dialog-title",""],[1,"poi-form"],["appearance","fill",1,"full-width"],["matInput","","placeholder","Ex: Porto de Neverwinter",3,"ngModelChange","ngModel"],["matInput","","rows","3","placeholder","O que os aventureiros veem ao chegar aqui?",3,"ngModelChange","ngModel"],[3,"ngModelChange","ngModel"],["value",""],[3,"value"],[1,"opt-icon"],[1,"new-submap"],[1,"section-label"],[1,"color-options"],["type","button",1,"color-swatch",3,"selected","background"],[1,"icon-options"],["type","button",1,"icon-option",3,"selected"],["align","end"],["mat-button","",3,"click"],["mat-raised-button","","color","primary",3,"click","disabled"],["matInput","","placeholder","Ex: Cripta subterr\xE2nea",3,"ngModelChange","ngModel"],[3,"fileChange","acceptedTypes","maxSize","showPreview"],[1,"submap-preview"],["alt","Preview do submapa",3,"src"],["type","button",1,"color-swatch",3,"click"],["type","button",1,"icon-option",3,"click"],[1,"icon-emoji"],["mat-button","","color","warn",3,"click"],["mat-stroked-button",""],["mat-stroked-button","",3,"click"]],template:function(e,t){e&1&&(r(0,"h2",0),l(1),o(),r(2,"mat-dialog-content")(3,"div",1)(4,"mat-form-field",2)(5,"mat-label"),l(6,"Nome do local"),o(),r(7,"input",3),D("ngModelChange",function(s){return P(t.label,s)||(t.label=s),s}),o()(),r(8,"mat-form-field",2)(9,"mat-label"),l(10,"Descri\xE7\xE3o"),o(),r(11,"textarea",4),D("ngModelChange",function(s){return P(t.description,s)||(t.description=s),s}),o()(),r(12,"mat-form-field",2)(13,"mat-label"),l(14,"Destino do ponto"),o(),r(15,"mat-select",5),D("ngModelChange",function(s){return P(t.destination,s)||(t.destination=s),s}),r(16,"mat-option",6),l(17,"Nenhum"),o(),r(18,"mat-option",7)(19,"mat-icon",8),l(20,"add"),o(),l(21," Criar novo submapa... "),o(),w(22,Zt,2,2,"mat-option",7,qt),o(),r(24,"mat-hint"),l(25,"Ao clicar no ponto, abre este mapa."),o()(),g(26,ei,15,7,"div",9),r(27,"label",10),l(28,"Cor do pin"),o(),r(29,"div",11),w(30,ti,1,5,"button",12,me),o(),r(32,"label",10),l(33,"\xCDcone"),o(),r(34,"div",13),w(35,ii,3,4,"button",14,me),o()()(),r(37,"mat-dialog-actions",15),g(38,ai,5,1),r(39,"button",16),h("click",function(){return t.onCancel()}),l(40,"Cancelar"),o(),r(41,"button",17),h("click",function(){return t.onSave()}),r(42,"mat-icon"),l(43,"check"),o(),l(44," Salvar "),o()()),e&2&&(d(),X(" ",t.existing?"Editar Ponto de Interesse":"Novo Ponto de Interesse"," "),d(6),R("ngModel",t.label),d(4),R("ngModel",t.description),d(4),R("ngModel",t.destination),d(3),b("value",t.newSubmapValue),d(4),k(t.availableMaps),d(4),f(t.destination===t.newSubmapValue?26:-1),d(4),k(t.colors),d(5),k(t.icons),d(3),f(t.existing?38:-1),d(3),b("disabled",!t.label.trim()||t.destination===t.newSubmapValue&&!t.newSubmapName.trim()))},dependencies:[ee,Ge,Y,J,B,N,at,tt,nt,it,ne,ie,te,rt,U,O,st,ot,le,se,oe,gt],styles:[".poi-form[_ngcontent-%COMP%]{display:flex;flex-direction:column;gap:16px;min-width:min(380px,90vw);padding:8px 0}.full-width[_ngcontent-%COMP%]{width:100%}.opt-icon[_ngcontent-%COMP%]{font-size:1.1rem;width:1.1rem;height:1.1rem;margin-right:8px;vertical-align:middle}.new-submap[_ngcontent-%COMP%]{display:flex;flex-direction:column;gap:8px;padding:12px;border:1px dashed rgba(124,77,255,.5);border-radius:10px;background:#7c4dff0f}.submap-preview[_ngcontent-%COMP%]{display:flex;flex-direction:column;align-items:center;gap:4px}.submap-preview[_ngcontent-%COMP%]   img[_ngcontent-%COMP%]{max-width:100%;max-height:160px;object-fit:contain;border-radius:8px;border:1px solid rgba(255,255,255,.12)}.submap-preview[_ngcontent-%COMP%]   span[_ngcontent-%COMP%]{font-size:.7rem;color:#ffffffb3}.section-label[_ngcontent-%COMP%]{font-size:.8125rem;font-weight:500;color:#ffffffb3;margin-bottom:4px}.color-options[_ngcontent-%COMP%]{display:flex;gap:8px;flex-wrap:wrap}.color-swatch[_ngcontent-%COMP%]{width:32px;height:32px;border-radius:50%;border:2px solid transparent;cursor:pointer;transition:transform .15s,border-color .15s}.color-swatch[_ngcontent-%COMP%]:hover{transform:scale(1.15)}.color-swatch.selected[_ngcontent-%COMP%]{border-color:#fff;transform:scale(1.2)}.icon-options[_ngcontent-%COMP%]{display:flex;gap:8px;flex-wrap:wrap}.icon-option[_ngcontent-%COMP%]{width:40px;height:40px;border-radius:8px;border:2px solid transparent;background:#ffffff14;cursor:pointer;display:flex;align-items:center;justify-content:center;transition:background .15s,border-color .15s}.icon-option[_ngcontent-%COMP%]:hover{background:#ffffff29}.icon-option.selected[_ngcontent-%COMP%]{border-color:#7c4dff;background:#7c4dff33}.icon-emoji[_ngcontent-%COMP%]{font-size:1.35rem;line-height:1}"]})};var oi=["mapContainer"],si=(n,a)=>a.tool,li=(n,a)=>a.id;function di(n,a){n&1&&(r(0,"div",4),x(1,"app-loading-spinner",5),o()),n&2&&(d(),b("isLoading",!0))}function ci(n,a){if(n&1){let e=C();r(0,"div",4)(1,"app-empty-state",6),h("action",function(){u(e);let i=m();return _(i.goBack())}),o()()}if(n&2){let e=m();d(),b("message",e.error())}}function mi(n,a){if(n&1){let e=C();r(0,"button",16),h("click",function(){u(e);let i=m(2);return _(i.goToParent())}),r(1,"mat-icon"),l(2,"arrow_back"),o(),l(3," Voltar ao mapa pai "),o()}}function pi(n,a){n&1&&(r(0,"span",12),l(1,"Clique no mapa para posicionar o ponto"),o())}function hi(n,a){n&1&&(r(0,"span",12),l(1,"Clique e arraste para pintar as c\xE9lulas"),o())}function ui(n,a){if(n&1){let e=C();x(0,"span",17),r(1,"button",16),h("click",function(){u(e);let i=m(2);return _(i.openEditMapDialog())}),r(2,"mat-icon"),l(3,"edit"),o(),l(4," Editar Mapa "),o(),r(5,"button",18),h("click",function(){u(e);let i=m(2);return _(i.deleteMap())}),r(6,"mat-icon"),l(7,"delete"),o(),l(8," Excluir "),o()}}function _i(n,a){if(n&1){let e=C();r(0,"button",20),h("click",function(){let i=u(e).$implicit,s=m(3);return _(s.selectTool(i.tool))}),r(1,"mat-icon"),l(2),o(),l(3),o()}if(n&2){let e=a.$implicit,t=m(3);y("active",t.dungeonTool()===e.tool),b("matTooltip",e.label),d(2),V(e.icon),d(),X(" ",e.label," ")}}function gi(n,a){if(n&1&&(r(0,"mat-option",23),l(1),o()),n&2){let e=a.$implicit;b("value",e.id),d(),X(" ",e.name," ")}}function fi(n,a){n&1&&(r(0,"span",12),l(1,"Escolha um personagem para carimbar"),o())}function bi(n,a){if(n&1){let e=C();r(0,"mat-form-field",21)(1,"mat-label"),l(2,"Personagem"),o(),r(3,"mat-select",22),h("ngModelChange",function(i){u(e);let s=m(3);return _(s.onCharacterSelected(i))}),w(4,gi,2,2,"mat-option",23,li),o()(),g(6,fi,2,0,"span",12)}if(n&2){let e=m(3);d(3),b("ngModel",e.selectedCharacterId()),d(),k(e.characters()),d(2),f(e.selectedCharacterId()?-1:6)}}function vi(n,a){if(n&1&&(r(0,"div",13),w(1,_i,4,5,"button",19,si),g(3,bi,7,2),o()),n&2){let e=m(2);d(),k(e.dungeonTools),d(2),f(e.dungeonTool()==="character"?3:-1)}}function xi(n,a){n&1&&x(0,"app-map-config-panel")}function Mi(n,a){if(n&1){let e=C();r(0,"div",7),g(1,mi,4,0,"button",8),r(2,"button",9),h("click",function(){u(e);let i=m();return _(i.togglePinPlacement())}),r(3,"mat-icon"),l(4,"add_location"),o(),l(5," Adicionar Ponto "),o(),r(6,"button",10),h("click",function(){u(e);let i=m();return _(i.toggleDrawMode())}),r(7,"mat-icon"),l(8,"draw"),o(),l(9," Desenhar Masmorra "),o(),r(10,"button",11),h("click",function(){u(e);let i=m();return _(i.toggleGrid())}),r(11,"mat-icon"),l(12,"grid_4x4"),o()(),g(13,pi,2,0,"span",12),g(14,hi,2,0,"span",12),g(15,ui,9,0),o(),g(16,vi,4,1,"div",13),g(17,xi,1,0,"app-map-config-panel"),r(18,"button",14),h("click",function(){u(e);let i=m();return _(i.showConfigPanel=!i.showConfigPanel)}),r(19,"mat-icon"),l(20,"layers"),o()(),r(21,"button",15),h("click",function(){u(e);let i=m();return _(i.toggleFullscreen())}),r(22,"mat-icon"),l(23,"fullscreen"),o()()}if(n&2){let e=m();d(),f(e.parentMapId?1:-1),d(),y("active",e.pinPlacementMode()),b("disabled",e.drawMode()),d(4),y("active",e.drawMode()),b("disabled",!e.hasImage())("matTooltip",e.hasImage()?"Desenhar a masmorra sobre a imagem":"Adicione uma imagem ao mapa para desenhar"),d(4),y("active",e.gridVisible()),d(3),f(e.pinPlacementMode()?13:-1),d(),f(e.drawMode()?14:-1),d(),f(e.currentMapData?15:-1),d(),f(e.drawMode()?16:-1),d(),f(e.showConfigPanel?17:-1)}}var It=["#7c4dff","#e53935","#00c853","#2979ff","#ff6d00","#00bcd4","#ff4081","#ffd600"],Et=class n{router=p(We);route=p(Fe);dialog=p(et);mapService=p(ae);dungeonService=p(re);store=p(ge);characterStore=p(ge);mapContainer=Ne.required("mapContainer");showConfigPanel=!1;pinPlacementMode=M(!1);drawMode=M(!1);hasImage=M(!1);gridVisible=M(!1);dungeonTool=M("floor");characters=M([]);selectedCharacterId=M(null);mapTitle="Mapa";breadcrumbs=[];parentMapId=null;currentMapData=null;loading=M(!0);error=M(null);dungeonTools=[{tool:"floor",icon:"crop_square",label:"Piso"},{tool:"wall",icon:"border_outer",label:"Parede"},{tool:"false_wall",icon:"border_style",label:"Parede falsa"},{tool:"water",icon:"water_drop",label:"\xC1gua"},{tool:"difficult",icon:"grass",label:"Terreno dif\xEDcil"},{tool:"rubble",icon:"scatter_plot",label:"Escombros"},{tool:"door",icon:"door_front",label:"Porta"},{tool:"secret_door",icon:"key",label:"Porta secreta"},{tool:"trap",icon:"warning",label:"Armadilha"},{tool:"chest",icon:"redeem",label:"Ba\xFA"},{tool:"mimic",icon:"pest_control",label:"M\xEDmico"},{tool:"character",icon:"person_add",label:"Personagem"},{tool:"erase",icon:"cleaning_services",label:"Apagar"}];mapId=null;routeSub=null;characterSub=null;clickUnregister=null;featureUnregister=null;async ngAfterViewInit(){this.loading.set(!0),this.error.set(null);try{await this.mapService.initialize(this.mapContainer().nativeElement,{zoom:10,center:[-46.6333,-23.5505]})}catch(a){console.error("Falha ao inicializar mapa:",a),this.error.set("Erro ao carregar mapa"),this.loading.set(!1);return}this.setupClickHandler(),this.characterSub=this.characterStore.subscribe("characters",a=>{this.characters.set(a),this.applyCharacter()}),this.routeSub=this.route.paramMap.subscribe(a=>{this.mapId=a.get("id"),this.loadMapData()})}ngOnDestroy(){this.clickUnregister?.(),this.featureUnregister?.(),this.routeSub?.unsubscribe(),this.characterSub?.unsubscribe(),this.dungeonService.destroy(),this.mapService.destroy()}toggleFullscreen(){this.mapService.toggleFullscreen(this.mapContainer().nativeElement)}toggleGrid(){this.mapService.toggleGrid().then(()=>{this.gridVisible.set(this.mapService.isGridVisible())})}togglePinPlacement(){this.pinPlacementMode.update(a=>!a)}async toggleDrawMode(){if(this.drawMode()){this.dungeonService.disable(),this.drawMode.set(!1);return}this.hasImage()&&(this.pinPlacementMode.set(!1),this.dungeonService.setTool(this.dungeonTool()),await this.dungeonService.enable(),this.drawMode.set(!0))}selectTool(a){this.dungeonTool.set(a),this.dungeonService.setTool(a),a==="character"&&this.applyCharacter()}onCharacterSelected(a){this.selectedCharacterId.set(a),this.applyCharacter()}applyCharacter(){let a=this.selectedCharacterId(),e=this.characters(),t=e.find(v=>v.id===a);if(!t){this.dungeonService.setCharacter(null);return}let i=It[e.indexOf(t)%It.length],s={id:t.id,name:t.name,color:i};this.dungeonService.setCharacter(s)}goToParent(){this.parentMapId&&this.router.navigate(["/mapa",this.parentMapId])}goBack(){this.router.navigate(["/mapa"])}openEditMapDialog(){if(!this.currentMapData)return;this.dialog.open(ft,{width:"520px",maxWidth:"95vw",data:{map:this.currentMapData}}).afterClosed().subscribe(e=>{e&&this.loadMapData()})}deleteMap(){if(!this.currentMapData)return;this.dialog.open(lt,{width:"420px",data:{title:"Excluir Mapa",message:`Tem certeza que deseja excluir "${this.currentMapData.name}"?`,confirmText:"Excluir",cancelText:"Cancelar"}}).afterClosed().subscribe(e=>{e&&this.currentMapData&&(this.store.delete("maps",this.currentMapData.id),this.router.navigate(["/mapa"]))})}async loadMapData(){if(this.loading.set(!0),this.dungeonService.disable(),this.drawMode.set(!1),this.mapId){let a=this.store.snapshot("maps").find(e=>e.id===this.mapId);if(a){this.currentMapData=a,this.mapService.setCurrentMapId(this.mapId),this.mapTitle=a.name;let e=this.mapService.getMapHierarchy(this.mapId);this.breadcrumbs=e.map((t,i)=>({label:t.name,route:i<e.length-1?`/mapa/${t.id}`:void 0})),this.parentMapId=this.mapService.getParentMapId(this.mapId),a.backgroundImage?(await this.mapService.setImageBackground(a.backgroundImage,a.width||1024,a.height||768),this.hasImage.set(!0)):(await this.mapService.clearImageBackground(),this.hasImage.set(!1)),await this.mapService.setGridConfig(a.grid),await this.mapService.setGridVisible(!1),this.gridVisible.set(this.mapService.isGridVisible()),await this.dungeonService.configure({mapId:this.mapId,extent:[0,0,a.width||1024,a.height||768],cellSize:a.grid?.cellSize??50,columns:a.grid?.columns??24,rows:a.grid?.rows??18}),await this.dungeonService.load(a.dungeon),await this.mapService.renderPois(a.markers??[]),await this.mapService.showPoisLayer(),await this.mapService.renderSubmapPins(a.submaps??[]),await this.mapService.showSubmapPinsLayer(),this.loading.set(!1);return}this.currentMapData=null,this.mapService.setCurrentMapId(null),this.error.set("Mapa n\xE3o encontrado"),this.loading.set(!1);return}this.currentMapData=null,this.mapService.setCurrentMapId(null),this.mapTitle="Mapa",this.breadcrumbs=[],this.parentMapId=null,this.loading.set(!1)}setupClickHandler(){this.featureUnregister=this.mapService.onFeatureClick(a=>{this.pinPlacementMode()||this.drawMode()||(a.type==="poi"&&a.marker?this.openPoiDialog(a.marker):a.type==="submap"&&a.targetMapId&&this.router.navigate(["/mapa",a.targetMapId]))}),this.clickUnregister=this.mapService.onMapClick(a=>{if(!this.pinPlacementMode()||this.drawMode())return;this.pinPlacementMode.set(!1),this.dialog.open(H,{data:{x:a[0],y:a[1],availableMaps:this.mapService.getAllMaps().filter(t=>t.id!==this.mapId),currentMapId:this.mapId},width:"460px",maxWidth:"95vw"}).afterClosed().subscribe(t=>{t?.action==="save"&&t.marker&&(this.applyNewSubmap(t),this.mapService.addPoi(t.marker))})})}openPoiDialog(a){this.dialog.open(H,{data:{x:a.x,y:a.y,existing:a,availableMaps:this.mapService.getAllMaps().filter(t=>t.id!==this.mapId),currentMapId:this.mapId},width:"460px",maxWidth:"95vw"}).afterClosed().subscribe(t=>{t?.action==="save"&&t.marker?(this.applyNewSubmap(t),this.mapService.updatePoi(t.marker)):t?.action==="delete"?this.mapService.deletePoi(a.id):t?.action==="open"&&a.targetMapId&&this.router.navigate(["/mapa",a.targetMapId])})}applyNewSubmap(a){if(!a||a.action!=="save"||!a.newSubmap)return;let e=this.mapService.createSubmap(a.newSubmap.name,a.newSubmap.kind,a.newSubmap.image);a.marker.targetMapId=e.id}static \u0275fac=function(e){return new(e||n)};static \u0275cmp=S({type:n,selectors:[["app-map-view"]],viewQuery:function(e,t){e&1&&Pe(t.mapContainer,oi,5),e&2&&De()},decls:7,vars:3,consts:[["mapContainer",""],["icon","map",3,"title","breadcrumbs"],[1,"map-stage"],[1,"map-container"],[1,"overlay"],["message","Carregando mapa...",3,"isLoading"],["icon","map","actionLabel","Voltar",3,"action","message"],[1,"toolbar-row"],["mat-stroked-button",""],["mat-stroked-button","",3,"click","disabled"],["mat-stroked-button","",3,"click","disabled","matTooltip"],["mat-icon-button","","matTooltip","Mostrar/ocultar grade",3,"click"],[1,"hint"],[1,"tool-row"],["mat-fab","","aria-label","Configura\xE7\xF5es do mapa",1,"fab-btn","config-btn",3,"click"],["mat-fab","","aria-label","Alternar tela cheia",1,"fab-btn","fullscreen-btn",3,"click"],["mat-stroked-button","",3,"click"],[1,"toolbar-spacer"],["mat-stroked-button","","color","warn",3,"click"],["mat-stroked-button","",1,"tool-btn",3,"active","matTooltip"],["mat-stroked-button","",1,"tool-btn",3,"click","matTooltip"],["appearance","outline","subscriptSizing","dynamic",1,"char-select"],[3,"ngModelChange","ngModel"],[3,"value"]],template:function(e,t){e&1&&(x(0,"app-page-header",1),r(1,"div",2),x(2,"div",3,0),g(4,di,2,1,"div",4)(5,ci,2,1,"div",4)(6,Mi,24,15),o()),e&2&&(b("title",t.mapTitle)("breadcrumbs",t.breadcrumbs),d(4),f(t.loading()?4:t.error()?5:6))},dependencies:[ee,Y,J,B,N,Ke,Ze,ne,ie,te,U,O,le,se,oe,_t,ut,de,mt,ct,dt],styles:["[_nghost-%COMP%]{display:flex;flex-direction:column;width:100%;height:calc(100vh - 112px);position:relative;overflow:hidden}.map-stage[_ngcontent-%COMP%]{position:relative;flex:1;min-height:0}.toolbar-row[_ngcontent-%COMP%], .tool-row[_ngcontent-%COMP%]{position:relative;display:flex;flex-wrap:wrap;align-items:center;gap:8px;row-gap:8px;padding:8px 16px;background:#0000004d;z-index:10;flex-shrink:0}.tool-row[_ngcontent-%COMP%]{padding-top:0;background:#00000038}.toolbar-row[_ngcontent-%COMP%]   button.active[_ngcontent-%COMP%], .tool-row[_ngcontent-%COMP%]   button.active[_ngcontent-%COMP%], button.active[_ngcontent-%COMP%]{background:#7c4dff40;border-color:#7c4dff}.hint[_ngcontent-%COMP%]{font-size:.8125rem;color:#fff9;font-style:italic}.toolbar-spacer[_ngcontent-%COMP%]{flex:1}.tool-btn[_ngcontent-%COMP%]{min-width:0}.char-select[_ngcontent-%COMP%]{width:180px;font-size:.85rem}.map-container[_ngcontent-%COMP%]{position:absolute;inset:0;width:100%;height:100%}.overlay[_ngcontent-%COMP%]{position:absolute;inset:0;z-index:20;display:flex;align-items:center;justify-content:center;background:#0a0a1299}.fab-btn[_ngcontent-%COMP%]{position:absolute;z-index:10}.fullscreen-btn[_ngcontent-%COMP%]{bottom:24px;right:24px}.config-btn[_ngcontent-%COMP%]{top:16px;right:16px}@media(max-width:480px){.config-btn[_ngcontent-%COMP%]{top:8px;right:8px}.fullscreen-btn[_ngcontent-%COMP%]{bottom:16px;right:16px}}[_nghost-%COMP%]     app-page-header{flex-shrink:0}"]})};export{Et as MapViewComponent};
