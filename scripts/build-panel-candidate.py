import hashlib
import json
import os
import shutil
import subprocess
from pathlib import Path
import yaml
import gzip
import io
import tarfile

root=Path('/mnt/foto/home-lab-dev/build-workspaces')
src=root/'station-airplay-plugin-panel-20261115'
out=root/'airplay-plugin-panel-test-20261115'
yam=root/'yam-registry-v2-20261101'
env=dict(os.environ,PATH='/home/rus/.local/go/bin:/tmp/upx-5.2.1:'+os.environ['PATH'])
version='2.1.8-panel.1'
(out/'bin').mkdir(parents=True,exist_ok=True)
(out/'packages').mkdir(exist_ok=True)
(out/'reports').mkdir(exist_ok=True)
def run(args,cwd=src,log=None,extra=None):
    e=dict(env,**(extra or {}))
    if log:
        with (out/'reports'/log).open('w') as stream:
            subprocess.run(args,cwd=cwd,env=e,stdout=stream,stderr=subprocess.STDOUT,check=True)
    else:
        subprocess.run(args,cwd=cwd,env=e,check=True)
run(['python3','panel/make-page.py'])
run(['node','panel/test-transport.cjs','panel/transport.js'],log='transport.log')
run(['go','vet','./...'],cwd=src/'source',log='vet.log')
run(['go','test','-race','-count=1','./...'],cwd=src/'source',log='race.log')
run(['python3','tests/test_yampkg_integrity.py'],log='integrity.log')
run(['python3','tests/test_package_yampkg.py'],log='matrix.log')
for path,package in [('station-airplay','.'),('panel-command','./cmd/panel-command')]:
    raw=out/'bin'/(path+'.raw')
    run(['go','build','-buildvcs=false','-trimpath','-ldflags=-s -w','-o',str(raw),package],
        cwd=src/'source',extra={'GOOS':'linux','GOARCH':'arm','GOARM':'7','CGO_ENABLED':'0'})
    shutil.copyfile(raw,out/'bin'/path)
    (out/'bin'/path).chmod(0o755)
    run(['upx','--best','--lzma',str(out/'bin'/path)],log=path+'-upx.log')
    run(['upx','-t',str(out/'bin'/path)])
run(['python3','scripts/package_yampkg.py','--base',str(root/'airplay-next-artifacts-20261113/base-2.1.4'),
     '--binary',str(out/'bin/station-airplay'),'--version',version,'--slim-runtime',
     '--upx','/tmp/upx-5.2.1/upx','--out',str(out/'packages'),'--force'],log='airplay-pack.log')
targets=[p['target'] for p in json.loads((out/'packages'/f'provenance-{version}.json').read_text())['packages']]
profiles=out/'pack-profiles'
profiles.mkdir(exist_ok=True)
# The installed host packer predates the aliases metadata field. Aliases
# don't affect any target's arch/triple/paths or this explicit-target pack.
for target in targets:
    profile=yaml.safe_load((yam/'platforms'/(target+'.yaml')).read_text())
    profile.pop('aliases',None)
    (profiles/(target+'.yaml')).write_text(yaml.safe_dump(profile,sort_keys=False))
for name in ['panel-command']:
    stage=out/('stage-'+name)
    plugin=stage/'plugin'
    plugin.mkdir(parents=True,exist_ok=True)
    if name=='panel-command':
        shutil.copyfile(out/'bin/panel-command',plugin/'panel-command')
        (plugin/'panel-command').chmod(0o755)
        shutil.copyfile(src/'panel/command.toml',plugin/'plugin.toml')
        depends=['yam/luna >=0.2.26','airplay/airplay-luna >=2.1.8-panel.1']
    else:
        for file in ['index.html','main.lua']:
            shutil.copyfile(src/'panel'/file,plugin/file)
        shutil.copyfile(src/'panel/web.toml',plugin/'plugin.toml')
        depends=['yam/luna >=0.2.26','airplay/panel-command >=0.1.0-test.1']
    shutil.copyfile(src/'panel/LUNA-LICENSE',plugin/'LUNA-LICENSE')
    manifest={'name':'airplay/'+name,'version':'0.1.0-test.1','license':'MIT','description':
              'Experimental AirPlay plugin panel. Требует Luna 0.2.26+. No Luna patch required.',
              'homepage':'https://github.com/ryysel/Station-airplay',
              'targets':targets,'depends':depends,
              'files':{'share/luna/plugins/airplay.'+name+'/':'plugin/'}}
    (stage/'yam.yaml').write_text(yaml.safe_dump(manifest,allow_unicode=True,sort_keys=False))
    for target in targets:
        destination=out/'packages'/f'airplay-{name}-0.1.0-test.1-{target}.yampkg'
        run([str(yam/'target/release/yam-publish'),'pack','--source',str(stage),'--platform',target,
             '--profiles',str(profiles),'--output',str(destination)],log=f'pack-{name}-{target}.log')
checks=[]
invalid=out/'not-for-install-yam-lua'
invalid.mkdir(exist_ok=True)
for path in (out/'packages').glob('airplay-panel-0.1.0-test.1-*.yampkg'):
    shutil.move(str(path),str(invalid/path.name))
for path in sorted((out/'packages').glob('*.yampkg')):
    checks.append(hashlib.sha256(path.read_bytes()).hexdigest()+'  '+path.name)
(out/'packages/SHA256SUMS').write_text('\n'.join(checks)+'\n')
ui=out/'packages/airplay-panel-0.1.0-test.1.yap'
with ui.open('wb') as stream, gzip.GzipFile(fileobj=stream,mode='wb',filename='',mtime=0) as zipped, tarfile.open(fileobj=zipped,mode='w') as archive:
    for source,name in [('web.toml','plugin.toml'),('main.lua','main.lua'),('index.html','index.html'),('LUNA-LICENSE','LICENSE')]:
        data=(src/'panel'/source).read_bytes()
        info=tarfile.TarInfo(name);info.size=len(data);info.mode=0o644;info.mtime=0
        archive.addfile(info,io.BytesIO(data))
checks.append(hashlib.sha256(ui.read_bytes()).hexdigest()+'  '+ui.name)
(out/'packages/SHA256SUMS').write_text('\n'.join(checks)+'\n')
print('BUILT',len(checks),'packages',flush=True)
