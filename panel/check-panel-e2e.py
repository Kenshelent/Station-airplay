import base64
import concurrent.futures
import copy
import json
import re
import shlex
import subprocess
import time
import uuid
from pathlib import Path

ROOT=Path('/mnt/foto/home-lab-dev/build-workspaces/airplay-plugin-panel-test-20261115')
ADB='127.0.0.1:15823'
ROOT.mkdir(exist_ok=True)
def api(path,method='GET',data=None,raw=False):
    # File transfer, not adbd stdout: old adbd truncates large plugin lists.
    name=uuid.uuid4().hex
    guest='/tmp/panel-http-'+name
    args=['wget','-S','-O',guest,'--header=Content-Type: application/json','--header=X-Luna-Local: 1']
    if method=='POST': args.append('--post-data='+json.dumps(data))
    args.append('http://127.0.0.1:8099'+path)
    result=subprocess.run(['adb','-s',ADB,'shell',shlex.join(args)],capture_output=True,text=True,timeout=20)
    statuses=re.findall(r'HTTP/1\.[01] (\d+)',result.stdout+result.stderr)
    assert statuses,(result.stdout,result.stderr)
    status=int(statuses[-1])
    if status>=400: raise RuntimeError('HTTP '+str(status)+' '+path)
    local=ROOT/('response-'+name)
    try:
        subprocess.run(['adb','-s',ADB,'pull',guest,str(local)],capture_output=True,check=True,timeout=20)
        text=local.read_text()
        return text if raw else (json.loads(text) if text else None)
    finally:
        if local.exists(): local.unlink()
        subprocess.run(['adb','-s',ADB,'shell','rm','-f',guest],capture_output=True,timeout=10)

def invoke(request):
    attempt=uuid.uuid4().hex
    api('/api/plugins/airplay.panel-command/invoke','POST',{'args':[json.dumps({'request':request,'attempt':attempt},separators=(',',':'))]})
    return attempt
def get_result(identifier,attempt):
    log=api('/api/plugins/airplay.panel-command/logs',raw=True)
    for line in reversed(log.splitlines()):
        if not line.startswith('AP1 '):continue
        try: value=json.loads(base64.b64decode(line[4:]))
        except Exception: continue
        if value['id']==identifier and value['attempt']==attempt:return value
    return None

def rpc(operation,method='GET',body=None,identifier=None,epoch=None):
    request={'id':identifier or uuid.uuid4().hex,'epoch':EPOCH if epoch is None else epoch,'operation':operation,'method':method}
    if body is not None:request['body']=body
    deadline=time.monotonic()+30
    while time.monotonic()<deadline:
        attempt=invoke(request)
        for _ in range(5):
            value=get_result(request['id'],attempt)
            if value is not None and value['status']!=503:
                print(operation,method,value['status'],request['id'],flush=True)
                return value
            time.sleep(.15)
    raise RuntimeError('RPC timeout')

api('/api/plugins/rescan','POST',{})
for identifier in ['airplay.panel','airplay.panel-command']:
    plugin=next(p for p in api('/api/plugins') if p['manifest']['id']==identifier)
    if not plugin['enabled']:api('/api/plugins/'+identifier+'/toggle','POST',{})
api('/api/plugins/airplay.panel/service/enable','POST',{})
for _ in range(40):
    if any(p['id']=='airplay.panel' for p in api('/api/custom-web')):break
    time.sleep(.2)
else:raise RuntimeError('no web registration')
page=api('/web/airplay.panel',raw=True)
assert 'airplay.panel-command/invoke' in page and 'requestID()' in page
assert '<option value="luna">' not in page
print('LUA_YAP_PAGE_OK',flush=True)
EPOCH=''
status=rpc('status');assert status['status']==200
EPOCH=status['epoch']
original=rpc('settings')['body']
changed=copy.deepcopy(original)
changed['timing_mode']='synchronized'
changed['equalizer']={'enabled':True,'preamp_db':-6,'bands_db':[1,0,0,0,0]}
try:
    save=rpc('settings','PUT',changed);assert save['status']==200
    readback=rpc('settings')['body']
    assert readback['equalizer']==changed['equalizer'] and readback['timing_mode']=='synchronized'
    # Replay uses the very same ID and payload, even after a cancelled
    # helper. A conflicting payload must not change persisted settings.
    identifier=uuid.uuid4().hex
    first=rpc('settings','PUT',changed,identifier=identifier)
    second=rpc('settings','PUT',changed,identifier=identifier)
    assert {k:v for k,v in first.items() if k!='attempt'}=={k:v for k,v in second.items() if k!='attempt'}
    bad=copy.deepcopy(changed);bad['timing_mode']='fast_audio'
    assert rpc('settings','PUT',bad,identifier=identifier)['status']==409
    assert rpc('settings')['body']['timing_mode']=='synchronized'
    assert rpc('settings','PUT',changed,epoch='old-epoch')['status']==409
    # Two independent clients contend for the same helper plugin ID.
    with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
        values=list(pool.map(lambda _:rpc('status'),range(2)))
    assert all(v['status']==200 for v in values)
    presets={'test':{'timing_mode':'fast_audio'}}
    assert rpc('presets','PUT',presets)['status']==200
    assert rpc('presets')['body']==presets
    assert rpc('presets','PUT',{'':'invalid'})['status']==400
    count=status['body']['led_count']
    lease=rpc('preview','POST',{'id':'','colors':['#100000']*count})
    assert lease['status']==200
    time.sleep(4.5)
    assert rpc('status')['body']['preview']['active'] is False
finally:
    assert rpc('settings','PUT',original)['status']==200
assert rpc('settings')['body']==original
print('PANEL_OFFICIAL_LUNA_END_TO_END_PASS',flush=True)
