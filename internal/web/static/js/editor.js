(function(){
  'use strict';
  var ta=document.getElementById('body'),prev=document.getElementById('preview'),split=document.querySelector('.editor-split'),
      status=document.getElementById('upload-status'),wc=document.getElementById('wc'),fileIn=document.getElementById('upload-input'),form=document.getElementById('editor');
  if(!ta) return;

  function insert(before,after,placeholder){
    var s=ta.selectionStart,e=ta.selectionEnd,sel=ta.value.substring(s,e)||placeholder||'';
    ta.setRangeText(before+sel+after,s,e,'end');
    if(!ta.value.substring(s,e)&&placeholder){ta.selectionStart=s+before.length;ta.selectionEnd=s+before.length+placeholder.length}
    ta.focus();changed();
  }
  function linePrefix(p){
    var s=ta.selectionStart,ls=ta.value.lastIndexOf('\n',s-1)+1;
    ta.setRangeText(p,ls,ls,'end');ta.selectionStart=ta.selectionEnd=s+p.length;ta.focus();changed();
  }
  function insertAtCursor(text){
    var s=ta.selectionStart;var pre=s>0&&ta.value[s-1]!=='\n'?'\n':'';
    ta.setRangeText(pre+text+'\n',s,ta.selectionEnd,'end');ta.focus();changed();
  }

  document.querySelectorAll('.toolbar button').forEach(function(b){b.addEventListener('click',function(){
    if(b.dataset.wrap!==undefined) insert(b.dataset.wrap,b.dataset.wrap,'text');
    else if(b.dataset.line!==undefined) linePrefix(b.dataset.line);
    else if(b.dataset.block!==undefined) insert('\n'+b.dataset.block,'\n```\n','code');
    else if(b.dataset.link!==undefined){var u=prompt('Link URL','https://');if(u)insert('[',']('+u+')','link text')}
    else if(b.dataset.upload){fileIn.accept=b.dataset.upload==='video'?'video/*':'image/*';fileIn.click()}
    else if(b.dataset.youtube!==undefined){var y=prompt('YouTube URL or video id');if(y){var id=(y.match(/(?:v=|youtu\.be\/|embed\/)([A-Za-z0-9_-]{6,})/)||[])[1]||y.trim();insertAtCursor('<iframe src="https://www.youtube-nocookie.com/embed/'+id+'" title="YouTube video" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen loading="lazy"></iframe>')}}
    else if(b.dataset.preview!==undefined){togglePreview(b)}
  })});

  var previewOn=false,ptimer;
  function togglePreview(b){previewOn=!previewOn;b.classList.toggle('on',previewOn);prev.hidden=!previewOn;split.classList.toggle('split',previewOn);if(previewOn)renderPreview()}
  function renderPreview(){
    fetch('/admin/api/preview',{method:'POST',headers:{'Content-Type':'application/x-www-form-urlencoded'},body:'body='+encodeURIComponent(ta.value)})
      .then(function(r){return r.json()}).then(function(j){prev.innerHTML=j.html;wc.textContent=words()+' words · '+j.minutes+' min read'}).catch(function(){});
  }
  function words(){return (ta.value.match(/\S+/g)||[]).length}
  function changed(){wc.textContent=words()+' words';dirty=true;if(previewOn){clearTimeout(ptimer);ptimer=setTimeout(renderPreview,400)}}
  ta.addEventListener('input',changed);changed();

  // Uploads: button, paste, drag-and-drop. Progress via XHR.
  function upload(file){
    var fd=new FormData();fd.append('file',file);
    var xhr=new XMLHttpRequest();xhr.open('POST','/admin/api/upload');
    status.textContent='Uploading '+file.name+'…';
    xhr.upload.onprogress=function(e){if(e.lengthComputable)status.textContent='Uploading '+file.name+' — '+Math.round(e.loaded/e.total*100)+'%'};
    xhr.onload=function(){
      var j={};try{j=JSON.parse(xhr.responseText)}catch(e){}
      if(xhr.status!==200||!j.url){status.textContent='Upload failed: '+(j.error||xhr.status);return}
      status.textContent='Uploaded '+file.name;
      if(j.kind==='video') insertAtCursor('<video controls preload="metadata" src="'+j.url+'"></video>');
      else insertAtCursor('![]('+j.url+')');
      setTimeout(function(){status.textContent=''},3000);
    };
    xhr.onerror=function(){status.textContent='Upload failed (network)'};
    xhr.send(fd);
  }
  fileIn.addEventListener('change',function(){Array.from(fileIn.files).forEach(upload);fileIn.value=''});
  ta.addEventListener('paste',function(e){var items=e.clipboardData&&e.clipboardData.items;if(!items)return;for(var i=0;i<items.length;i++){if(items[i].kind==='file'){e.preventDefault();upload(items[i].getAsFile())}}});
  ta.addEventListener('dragover',function(e){e.preventDefault();ta.classList.add('drop')});
  ta.addEventListener('dragleave',function(){ta.classList.remove('drop')});
  ta.addEventListener('drop',function(e){e.preventDefault();ta.classList.remove('drop');Array.from(e.dataTransfer.files).forEach(upload)});

  // Shortcuts.
  ta.addEventListener('keydown',function(e){
    if(!(e.ctrlKey||e.metaKey))return;
    if(e.key==='b'){e.preventDefault();insert('**','**','bold')}
    else if(e.key==='i'){e.preventDefault();insert('_','_','italic')}
    else if(e.key==='k'){e.preventDefault();var u=prompt('Link URL','https://');if(u)insert('[',']('+u+')','link text')}
    else if(e.key==='Tab'){e.preventDefault();insert('  ','')}
  });
  var dirty=false;
  document.addEventListener('keydown',function(e){if((e.ctrlKey||e.metaKey)&&e.key==='s'){e.preventDefault();dirty=false;form.querySelector('button[value=save]').click()}});
  form.addEventListener('submit',function(){dirty=false});
  window.addEventListener('beforeunload',function(e){if(dirty){e.preventDefault();e.returnValue=''}});

  // Cover preview from URL field / file picker.
  var coverURL=form.querySelector('input[name=cover]'),coverFile=form.querySelector('input[name=cover_file]'),cp=document.getElementById('cover-preview');
  function showCover(src){cp.innerHTML=src?'<img src="'+src+'" alt="">':''}
  if(coverURL)coverURL.addEventListener('change',function(){showCover(coverURL.value)});
  if(coverFile)coverFile.addEventListener('change',function(){if(coverFile.files[0])showCover(URL.createObjectURL(coverFile.files[0]))});
})();
