// The helper is a request/reply transport, not a realtime stream. Show only the
// newest acknowledged output, never loop or replay old frames as live motion.
function createLiveView({paint, note, now=()=>performance.now()}){
  let sequence=-1, receivedAt=0, ageAtReceive=-1, state='unavailable', lastError='';
  function label(){
    if(state==='unavailable'){note('Ещё нет подтверждённого кадра AirPlay.');return;}
    const age=ageAtReceive<0?0:ageAtReceive+Math.max(0,now()-receivedAt);
    const prefix=state==='released'?'Управление возвращено Luna':
      state==='error'?'Ошибка отправки; показан предыдущий кадр':'Последний подтверждённый кадр';
    const suffix=age>3000?' · данные устарели':'';
    note(prefix+' №'+sequence+' · '+(age/1000).toFixed(1)+' с назад'+suffix+
      (lastError?' · '+lastError:'')+'. Опрос через helper, не синхронная трансляция.');
  }
  function accept(live){
    if(!live||!Number.isSafeInteger(live.sequence)||live.sequence<0)return;
    // Older responses must not roll the page back to an earlier frame.
    if(live.sequence<sequence)return;
    const fresh=live.sequence>sequence;
    sequence=live.sequence;
    receivedAt=now();
    ageAtReceive=Number.isFinite(live.age_ms)?Math.max(-1,live.age_ms):-1;
    state=live.state||'unavailable';
    lastError=typeof live.error==='string'?live.error:'';
    if(fresh&&Array.isArray(live.colors)&&live.colors.length){
      const colors=live.colors.filter(c=>typeof c==='string'&&/^#[0-9a-f]{6}$/i.test(c));
      if(colors.length===live.colors.length){
        const bands=Array.from({length:4},(_,i)=>{
          const value=live.bands?.[i];
          return Number.isFinite(value)?Math.max(0,Math.min(1,value)):0;
        });
        paint(colors,bands);
      }
    }
    label();
  }
  return {accept,refresh:label};
}

const liveView=createLiveView({
  paint(frame,bands){
    const strip=$('live-strip');
    if(strip.children.length!==frame.length){
      strip.replaceChildren();
      frame.forEach(()=>strip.append(document.createElement('span')));
    }
    frame.forEach((color,i)=>strip.children[i].style.background=color);
    [...$('live-bands').children].forEach((bar,i)=>{
      bar.style.transform='scaleX('+bands[i]+')';
    });
  },
  note(text){$('live-note').textContent=text;}
});
function acceptLive(live){liveView.accept(live);}
setInterval(()=>{if(!document.hidden)liveView.refresh();},250);

let statusPollGeneration=0;
function stopStatusPoll(){
  statusPollGeneration++;
  clearTimeout(poll);
}
function startStatusPoll(){
  stopStatusPoll();
  const generation=statusPollGeneration;
  async function next(){
    if(generation!==statusPollGeneration)return;
    try{
      // No background polling, no overlap and no request queue growth.
      if(!document.hidden&&!busy)await status();
    }finally{
      if(generation===statusPollGeneration)poll=setTimeout(next,500);
    }
  }
  poll=setTimeout(next,500);
}
