// Ticket Landing — Three.js grid animation.
// Single merged BufferGeometry with vertex colours — one draw call for the whole grid.
// Adapted from pixel's landing.js — word changed to TICKET.
(function () {
  'use strict';

  // Skip animation if user already has a valid session
  try {
    if (localStorage.getItem('tk-authed') === '1') { document.documentElement.classList.remove('landing-active'); return; }
    var _r = sessionStorage.getItem('site2.auth');
    if (_r) { var _a = JSON.parse(_r); if (_a && _a.token) { document.documentElement.classList.remove('landing-active'); return; } }
  } catch(e) {}

  // ── Perlin noise (classic 2D) ──────────────────────────────────────
  var perm = new Uint8Array(512);
  var grad3 = [[1,1,0],[-1,1,0],[1,-1,0],[-1,-1,0],[1,0,1],[-1,0,1],[1,0,-1],[-1,0,-1],[0,1,1],[0,-1,1],[0,1,-1],[0,-1,-1]];
  (function () {
    var p = []; for (var i = 0; i < 256; i++) p[i] = i;
    for (var i = 255; i > 0; i--) { var j = Math.floor(Math.random()*(i+1)); var t = p[i]; p[i] = p[j]; p[j] = t; }
    for (var i = 0; i < 512; i++) perm[i] = p[i & 255];
  })();
  function dot2(g,x,y){return g[0]*x+g[1]*y;}
  function fade(t){return t*t*t*(t*(t*6-15)+10);}
  function lerp(a,b,t){return a+t*(b-a);}
  function perlin2(x,y){
    var X=Math.floor(x)&255,Y=Math.floor(y)&255; x-=Math.floor(x); y-=Math.floor(y);
    var u=fade(x),v=fade(y);
    var aa=perm[perm[X]+Y],ab=perm[perm[X]+Y+1],ba=perm[perm[X+1]+Y],bb=perm[perm[X+1]+Y+1];
    return lerp(lerp(dot2(grad3[aa%12],x,y),dot2(grad3[ba%12],x-1,y),u),lerp(dot2(grad3[ab%12],x,y-1),dot2(grad3[bb%12],x-1,y-1),u),v);
  }

  // ── 8×8 pixel font ────────────────────────────────────────────────
  var font = {
    T:[0xFF,0x18,0x18,0x18,0x18,0x18,0x18,0x18],
    I:[0xFF,0x18,0x18,0x18,0x18,0x18,0x18,0xFF],
    C:[0x7E,0xC0,0xC0,0xC0,0xC0,0xC0,0xC0,0x7E],
    K:[0xC6,0xCC,0xD8,0xF0,0xD8,0xCC,0xC6,0xC3],
    E:[0xFF,0xC0,0xC0,0xFC,0xC0,0xC0,0xC0,0xFF]
  };
  function isLetter(ch,r,c){return(font[ch][r]>>(7-c))&1;}

  // ── Colour ─────────────────────────────────────────────────────────
  function hsl(h,s,l){
    if(s===0)return[l,l,l];
    function f(p,q,t){if(t<0)t+=1;if(t>1)t-=1;if(t<1/6)return p+(q-p)*6*t;if(t<1/2)return q;if(t<2/3)return p+(q-p)*(2/3-t)*6;return p;}
    var q=l<0.5?l*(1+s):l+s-l*s, p=2*l-q;
    return[f(p,q,h+1/3),f(p,q,h),f(p,q,h-1/3)];
  }

  // ── Setup ──────────────────────────────────────────────────────────
  var canvas = document.getElementById('landing-canvas');
  if (!canvas) return;

  var renderer;
  try { renderer = new THREE.WebGLRenderer({canvas:canvas, antialias:false, alpha:false}); }
  catch(e) { finishLogin(); return; }
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
  renderer.setClearColor(0x000000, 1);
  renderer.setSize(window.innerWidth, window.innerHeight);

  var scene = new THREE.Scene();
  var camera = new THREE.OrthographicCamera(-1,1,1,-1,0.1,100);
  camera.position.z = 10;

  // ── Grid ───────────────────────────────────────────────────────────
  var WORD='TICKET', LW=8, LH=8, GAP=1;
  var WCOLS=WORD.length*LW+(WORD.length-1)*GAP, WROWS=LH;
  var GCOLS, GROWS, CS;
  var wsc, wsr;
  var cells, mesh, posArr, colArr;

  function build() {
    var aspect = window.innerWidth / window.innerHeight;
    GCOLS = Math.round(WCOLS / 0.5);
    CS = 2.0 / GCOLS;
    GROWS = Math.max(Math.round(GCOLS / aspect), WROWS * 3);

    camera.left = -(GCOLS*CS)/2; camera.right = (GCOLS*CS)/2;
    camera.top = (GROWS*CS)/2; camera.bottom = -(GROWS*CS)/2;
    camera.updateProjectionMatrix();
    renderer.setSize(window.innerWidth, window.innerHeight);

    wsc = Math.floor((GCOLS - WCOLS) / 2);
    wsr = Math.floor((GROWS - WROWS) / 2);

    if (mesh) { scene.remove(mesh); mesh.geometry.dispose(); }

    var N = GCOLS * GROWS;
    posArr = new Float32Array(N * 6 * 3);
    colArr = new Float32Array(N * 6 * 3);
    cells = new Array(N);

    var gap = CS * 0.08;
    var half = (CS - gap) / 2;

    for (var r = 0; r < GROWS; r++) {
      for (var c = 0; c < GCOLS; c++) {
        var idx = r * GCOLS + c;
        var cx = (c - GCOLS/2 + 0.5) * CS;
        var cy = (GROWS/2 - r - 0.5) * CS;

        var isW = false;
        var wr = r - wsr, wc = c - wsc;
        if (wr >= 0 && wr < WROWS && wc >= 0 && wc < WCOLS) {
          var li = Math.floor(wc / (LW + GAP));
          var inside = wc - li * (LW + GAP);
          if (li < WORD.length && inside < LW) isW = isLetter(WORD[li], wr, inside);
        }

        var vi = idx * 18;
        posArr[vi]   = cx-half; posArr[vi+1] = cy-half; posArr[vi+2] = 0;
        posArr[vi+3] = cx+half; posArr[vi+4] = cy-half; posArr[vi+5] = 0;
        posArr[vi+6] = cx+half; posArr[vi+7] = cy+half; posArr[vi+8] = 0;
        posArr[vi+9]  = cx-half; posArr[vi+10] = cy-half; posArr[vi+11] = 0;
        posArr[vi+12] = cx+half; posArr[vi+13] = cy+half; posArr[vi+14] = 0;
        posArr[vi+15] = cx-half; posArr[vi+16] = cy+half; posArr[vi+17] = 0;

        for (var v = 0; v < 18; v++) colArr[vi + v] = 0;

        cells[idx] = {
          c:c, r:r, isW:isW, vi:vi, cx:cx, cy:cy,
          phase: Math.random()*Math.PI*2,
          hue:0, sat:0, lgt:0, op:0,
          vy:0, posY:cy,
          alive:true, fallDelay:0, half:half
        };
      }
    }

    var geom = new THREE.BufferGeometry();
    geom.setAttribute('position', new THREE.BufferAttribute(posArr, 3));
    geom.setAttribute('color', new THREE.BufferAttribute(colArr, 3));
    geom.attributes.color.usage = THREE.DynamicDrawUsage;
    geom.attributes.position.usage = THREE.DynamicDrawUsage;

    var mat = new THREE.MeshBasicMaterial({ vertexColors: true });
    mesh = new THREE.Mesh(geom, mat);
    scene.add(mesh);
  }

  build();

  // ── State machine ──────────────────────────────────────────────────
  var S_INTRO=0, S_HOLD=1, S_FADEOUT=2, S_XFADE=3;
  var state=S_INTRO, st=0;
  var clock = new THREE.Clock();
  var INTRO_DUR=2.5, HOLD_DUR=0.075, FADE_DUR=1.25, XFADE_DUR=0.5;

  var WH=0.48, WS=0.85, WL=0.6;

  var mix=0, miy=0;
  var done = false;

  document.addEventListener('mousemove',function(e){mix=(e.clientX/window.innerWidth-0.5)*2;miy=(e.clientY/window.innerHeight-0.5)*2;});
  document.addEventListener('touchmove',function(e){if(e.touches.length){mix=(e.touches[0].clientX/window.innerWidth-0.5)*2;miy=(e.touches[0].clientY/window.innerHeight-0.5)*2;}});
  window.addEventListener('resize',function(){if(done)return;build();st=0;state=S_INTRO;});

  // ── Helpers ────────────────────────────────────────────────────────
  function setCol(cell, r, g, b, op) {
    var R=r*op, G=g*op, B=b*op;
    var vi = cell.vi;
    for (var v = 0; v < 6; v++) {
      colArr[vi + v*3]   = R;
      colArr[vi + v*3+1] = G;
      colArr[vi + v*3+2] = B;
    }
  }

  function setScale(cell, s) {
    var cx=cell.cx, cy=cell.posY, h=cell.half*s;
    var vi=cell.vi;
    posArr[vi]=cx-h;   posArr[vi+1]=cy-h;
    posArr[vi+3]=cx+h;  posArr[vi+4]=cy-h;
    posArr[vi+6]=cx+h;  posArr[vi+7]=cy+h;
    posArr[vi+9]=cx-h;  posArr[vi+10]=cy-h;
    posArr[vi+12]=cx+h; posArr[vi+13]=cy+h;
    posArr[vi+15]=cx-h; posArr[vi+16]=cy+h;
  }

  // ── Animate ────────────────────────────────────────────────────────
  function animate() {
    if (done) return;
    requestAnimationFrame(animate);

    var dt = clock.getDelta();
    if (dt > 0.1) dt = 0.1;
    st += dt;
    var time = clock.elapsedTime;
    var N = cells.length;
    var posDirty = false;

    camera.position.x = mix * CS * 0.5;
    camera.position.y = -miy * CS * 0.5;

    if (state === S_INTRO) {
      var t = Math.min(st/INTRO_DUR, 1);
      for (var i = 0; i < N; i++) {
        var cl = cells[i];
        if (!cl.isW) { setCol(cl,0,0,0,0); continue; }
        var dx = cl.c - GCOLS/2, dy = cl.r - GROWS/2;
        var dist = Math.sqrt(dx*dx+dy*dy) / (GCOLS*0.5);
        var ct = Math.max(0, Math.min(1, (t - dist*0.3)*2.5));

        if (ct > 0 && ct < 0.8) {
          var sc = 0.85 + 0.15*Math.sin((ct*3+cl.phase)*Math.PI*4);
          setScale(cl, sc);
          posDirty = true;
        } else if (ct >= 0.8) {
          setScale(cl, 1);
          posDirty = true;
        }

        var h=lerp(0,WH,ct), s=lerp(0,WS,ct), l=lerp(1,WL,ct*ct);
        var rgb = hsl(h,s,l);
        setCol(cl, rgb[0], rgb[1], rgb[2], ct);
        cl.op=ct; cl.hue=h; cl.sat=s; cl.lgt=l;
      }
      if (t >= 1) { state=S_HOLD; st=0; }
    }

    if (state === S_HOLD) {
      var t = Math.min(st/HOLD_DUR, 1);
      for (var i = 0; i < N; i++) {
        var cl = cells[i];
        if (!cl.isW) continue;
        var n = (perlin2(cl.c*0.2+time*0.5, cl.r*0.2)+1)/2;
        var rgb = hsl(WH, WS, WL+n*0.08-0.04);
        setCol(cl, rgb[0], rgb[1], rgb[2], 1);
        cl.op = 1; cl.hue = WH; cl.sat = WS; cl.lgt = WL+n*0.08-0.04;
      }
      if (t >= 1) { state=S_FADEOUT; st=0; }
    }

    if (state === S_FADEOUT) {
      var t = Math.min(st/FADE_DUR, 1);
      for (var i = 0; i < N; i++) {
        var cl = cells[i];
        if (!cl.isW) continue;
        var dx = cl.c - GCOLS/2, dy = cl.r - GROWS/2;
        var dist = Math.sqrt(dx*dx+dy*dy) / (GCOLS*0.5);
        var ct = Math.max(0, Math.min(1, (t - dist*0.3)*2.5));

        if (ct > 0.2 && ct < 1) {
          var sc = 0.85 + 0.15*Math.sin(((1-ct)*3+cl.phase)*Math.PI*4);
          setScale(cl, sc * (1-ct));
          posDirty = true;
        } else if (ct <= 0.2) {
          setScale(cl, 1);
          posDirty = true;
        }

        var h = lerp(WH, 0, ct);
        var s = lerp(WS, 0, ct);
        var l = lerp(WL, 1, ct*ct);
        var op = 1 - ct;
        var rgb = hsl(h, s, l);
        setCol(cl, rgb[0], rgb[1], rgb[2], op);
      }
      if (t >= 1) { state = S_XFADE; st = 0; }
    }

    if (state === S_XFADE) {
      var t = Math.min(st / XFADE_DUR, 1);
      var ease = t * t * (3 - 2 * t);

      canvas.style.opacity = String(1 - ease);

      if (!loginRevealed) {
        loginRevealed = true;
        document.documentElement.classList.remove('landing-active');
        var el = document.getElementById('login-form');
        if (el) { el.style.opacity = '0'; }
      }
      var el = document.getElementById('login-form');
      if (el) el.style.opacity = String(ease);

      if (t >= 1) { finishLogin(); return; }
    }

    mesh.geometry.attributes.color.needsUpdate = true;
    if (posDirty) mesh.geometry.attributes.position.needsUpdate = true;
    renderer.render(scene, camera);
  }

  var loginRevealed = false;

  function finishLogin() {
    done = true;
    try { renderer.dispose(); renderer.forceContextLoss(); } catch(e){}
    canvas.style.display = 'none';
    document.documentElement.classList.remove('landing-active');
    var el = document.getElementById('login-form');
    if (el) el.style.opacity = '1';
    var usr = document.getElementById('login-username');
    if (usr) usr.focus();
  }

  canvas.addEventListener('click', function(){ if(state!==S_FADEOUT){state=S_FADEOUT;st=0;} });
  canvas.addEventListener('touchstart', function(){ if(state!==S_FADEOUT){state=S_FADEOUT;st=0;} });

  animate();
})();
