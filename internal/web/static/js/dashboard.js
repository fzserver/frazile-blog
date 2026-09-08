(function(){
  'use strict';
  var panel=document.querySelector('.chart-panel');if(!panel)return;
  var period=panel.dataset.period||'30d';
  var accent=getComputedStyle(document.documentElement).getPropertyValue('--accent').trim()||'#7c5cff';
  var ok=getComputedStyle(document.documentElement).getPropertyValue('--ok').trim()||'#1f9d55';
  var muted=getComputedStyle(document.documentElement).getPropertyValue('--muted').trim()||'#888';
  var NS='http://www.w3.org/2000/svg';
  function el(n,a){var e=document.createElementNS(NS,n);for(var k in a)e.setAttribute(k,a[k]);return e}

  function lineChart(host,series){
    host.innerHTML='';
    var W=host.clientWidth||800,H=host.clientHeight||220,padL=36,padR=8,padT=10,padB=24;
    var svg=el('svg',{viewBox:'0 0 '+W+' '+H});
    var max=Math.max(1,Math.max.apply(null,series.map(function(p){return p.views})));
    var n=series.length,step=(W-padL-padR)/Math.max(1,n-1);
    var x=function(i){return padL+i*step},y=function(v){return padT+(H-padT-padB)*(1-v/max)};
    var ticks=Math.min(4,max);for(var g=0;g<=ticks;g++){var v=Math.round(max*g/ticks),yy=y(v);svg.appendChild(el('line',{x1:padL,x2:W-padR,y1:yy,y2:yy,stroke:muted,'stroke-opacity':.2}));var t=el('text',{x:padL-6,y:yy+4,'text-anchor':'end','font-size':10,fill:muted});t.textContent=Math.round(v);svg.appendChild(t)}
    function path(key){return series.map(function(p,i){return (i?'L':'M')+x(i)+' '+y(p[key])}).join(' ')}
    var area=el('path',{d:path('views')+' L'+x(n-1)+' '+y(0)+' L'+x(0)+' '+y(0)+' Z',fill:accent,'fill-opacity':.12});svg.appendChild(area);
    svg.appendChild(el('path',{d:path('views'),fill:'none',stroke:accent,'stroke-width':2.2,'stroke-linejoin':'round'}));
    svg.appendChild(el('path',{d:path('uniques'),fill:'none',stroke:ok,'stroke-width':1.8,'stroke-dasharray':'4 3'}));
    var every=Math.ceil(n/8);
    series.forEach(function(p,i){
      if(i%every===0||i===n-1){var t=el('text',{x:x(i),y:H-6,'text-anchor':i===0?'start':i===n-1?'end':'middle','font-size':10,fill:muted});t.textContent=p.label;svg.appendChild(t)}
      var c=el('circle',{cx:x(i),cy:y(p.views),r:3,fill:accent});var ti=el('title');ti.textContent=p.label+': '+p.views+' views, '+p.uniques+' unique';c.appendChild(ti);svg.appendChild(c);
    });
    host.appendChild(svg);
  }
  function barChart(host,vals){
    host.innerHTML='';
    var W=host.clientWidth||800,H=host.clientHeight||120,padB=18,padT=6;
    var svg=el('svg',{viewBox:'0 0 '+W+' '+H});
    var max=Math.max(1,Math.max.apply(null,vals)),bw=W/vals.length;
    vals.forEach(function(v,i){var h=(H-padT-padB)*v/max;var r=el('rect',{x:i*bw+2,y:H-padB-h,width:Math.max(1,bw-4),height:h,rx:2,fill:accent,'fill-opacity':.8});var t=el('title');t.textContent=i+':00 — '+v+' views';r.appendChild(t);svg.appendChild(r);
      if(i%3===0){var l=el('text',{x:i*bw+bw/2,y:H-4,'text-anchor':'middle','font-size':10,fill:muted});l.textContent=i;svg.appendChild(l)}});
    host.appendChild(svg);
  }
  var data;
  function draw(){if(!data)return;lineChart(document.getElementById('chart'),data.series||[]);barChart(document.getElementById('hours'),data.hours||[])}
  fetch('/admin/api/summary?period='+encodeURIComponent(period)).then(function(r){return r.json()}).then(function(j){data=j;draw()}).catch(function(){});
  var rt;window.addEventListener('resize',function(){clearTimeout(rt);rt=setTimeout(draw,150)});
})();
