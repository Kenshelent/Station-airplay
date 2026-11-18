const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict');
const source=fs.readFileSync(process.argv[2],'utf8');
// Accept either the built page or the bare module: the page embeds the
// module inside its single inline <script>, after the HTML.
const scriptStart=source.lastIndexOf('<script');
const body=scriptStart>=0?source.slice(source.indexOf('>',scriptStart)+1):source;
// Only the module is under test: the rest of the page script needs a DOM.
const marker='\nconst liveView=';
const from=body.indexOf('function createLiveView(');
const to=body.includes(marker)?body.indexOf(marker):body.length;
const code=body.slice(from>=0?from:0,to);
const sandbox={performance,Number,Array,Math};
vm.createContext(sandbox);
vm.runInContext(code+'\nglobalThis.create=createLiveView;',sandbox);
let time=0,draws=[],label='';
const view=sandbox.create({
  now:()=>time,paint:(colors,bands)=>draws.push({colors,bands}),note:value=>label=value
});
const frame={sequence:5,state:'sent',age_ms:100,colors:['#112233'],bands:[.2,.4,.6,.8]};
view.accept(frame);
assert.equal(draws.length,1);
assert.match(label,/№5/);
time=5000;
view.refresh();
assert.equal(draws.length,1,'timer must not replay buffered animation');
assert.match(label,/устарели/);
view.accept({...frame,age_ms:5100});
assert.equal(draws.length,1,'duplicate frames must not be played again');
view.accept({...frame,sequence:4,colors:['#FFFFFF']});
assert.equal(draws.length,1,'older response rolled back the picture');
view.accept({...frame,sequence:6,colors:['#AABBCC']});
assert.equal(draws.length,2);
view.accept({...frame,sequence:6,state:'error',error:'offline'});
assert.match(label,/offline/);
assert.equal(draws.length,2,'error manufactured a new frame');
view.accept({...frame,sequence:7,state:'released',colors:null});
assert.match(label,/возвращено Luna/);
assert.equal(draws.length,2,'handover invented a black LED frame');
view.accept({...frame,sequence:8,colors:['not-a-color']});
assert.equal(draws.length,2);
console.log('LIVE_LAST_ACK_SEQUENCE_STALENESS_NO_REPLAY_PASS');
