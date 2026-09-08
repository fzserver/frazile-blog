(function(){
  'use strict';
  // Theme toggle: explicit choice wins over the system preference.
  var tog=document.querySelector('.theme-toggle');
  if(tog){tog.addEventListener('click',function(){
    var root=document.documentElement;
    var cur=root.dataset.theme||(matchMedia('(prefers-color-scheme:dark)').matches?'dark':'light');
    var next=cur==='dark'?'light':'dark';
    root.dataset.theme=next;
    try{localStorage.setItem('theme',next)}catch(e){}
  });}

  // Flash dismiss + auto-fade.
  document.querySelectorAll('.flash-x').forEach(function(b){b.addEventListener('click',function(){b.closest('.flash').remove()})});
  var fl=document.querySelector('.flash'); if(fl) setTimeout(function(){fl.style.transition='opacity .6s';fl.style.opacity='0';setTimeout(function(){fl.remove()},700)},6000);

  // Close user menu when clicking elsewhere.
  document.addEventListener('click',function(e){document.querySelectorAll('details.menu[open]').forEach(function(d){if(!d.contains(e.target))d.removeAttribute('open')})});

  // Likes.
  var like=document.querySelector('.like');
  if(like){like.addEventListener('click',function(){
    if(like.dataset.login){location.href='/login?next='+encodeURIComponent(location.pathname);return}
    like.disabled=true;
    fetch('/api/posts/'+like.dataset.id+'/like',{method:'POST',headers:{'Accept':'application/json'}})
      .then(function(r){return r.json()})
      .then(function(j){if(j.likes!==undefined){like.querySelector('.count').textContent=j.likes;like.classList.toggle('on',!!j.liked);like.setAttribute('aria-pressed',String(!!j.liked))}})
      .catch(function(){})
      .finally(function(){like.disabled=false});
  });}

  // Copy link buttons.
  document.querySelectorAll('.copy').forEach(function(b){b.addEventListener('click',function(){
    var txt=b.dataset.url||'';if(txt.indexOf('video:')===0)txt='<video controls src="'+txt.slice(6)+'"></video>';var old=b.textContent;
    (navigator.clipboard?navigator.clipboard.writeText(txt):Promise.reject()).then(function(){b.textContent='Copied!';setTimeout(function(){b.textContent=old},1500)},function(){prompt('Copy:',txt)});
  })});

  // Comment replies: point the form at the parent and scroll to it.
  var form=document.getElementById('comment-form');
  if(form){
    var parent=document.getElementById('parent'),note=form.querySelector('.replying-to'),ta=form.querySelector('textarea');
    document.querySelectorAll('.reply-btn').forEach(function(b){b.addEventListener('click',function(){
      parent.value=b.dataset.id;note.hidden=false;note.querySelector('strong').textContent=b.dataset.name;
      form.scrollIntoView({behavior:'smooth',block:'center'});ta.focus();
    })});
    form.querySelector('.cancel-reply').addEventListener('click',function(){parent.value='0';note.hidden=true});
  }
})();
