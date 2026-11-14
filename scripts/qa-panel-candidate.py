import functools
import hashlib
import http.server
import shutil
import subprocess
import threading
from pathlib import Path

root=Path('/mnt/foto/home-lab-dev/build-workspaces')
src=root/'station-airplay-plugin-panel-20261115'
out=root/'airplay-plugin-panel-test-20261115'
original=root/'kuyase-emulator-20261031/kuyase-upstream'
emu=out/'qa/emulator-final'
(emu/'work').mkdir(parents=True,exist_ok=False)
(emu/'kernel').mkdir()
(emu/'guest-init/bin').mkdir(parents=True)
for link,target in [
    (emu/'kernel/Image',original/'kernel/Image'),
    (emu/'guest-init/bin/qemu-hw-bridge',original/'guest-init/bin/qemu-hw-bridge'),
    (emu/'work/yandexmini-system.ext4',root/'kuyase-emulator-20261031/kuyase/work/yandexmini-system.ext4')]:
    link.symlink_to(target)
subprocess.run(['cp','--reflink=auto','--sparse=always',
    str(root/'kuyase-emulator-20261031/kuyase/work/yandexmini-data.ext4'),
    str(emu/'work/yandexmini-data.ext4')],check=True)
public=out/'qa/transfer';public.mkdir(parents=True,exist_ok=True)
for source,name in [
    (out/'packages/station-airplay-2.1.8-panel.1-yandexmini.yampkg','mini-217.yampkg'),
    (out/'packages/airplay-panel-command-0.1.0-test.1-yandexmini.yampkg','command.yampkg'),
    (out/'packages/airplay-panel-0.1.0-test.1.yap','panel.yap'),
    (root/'airplay-next-rebuild-20261113/base-luna-0.2.26/yandexmini.yampkg','luna-official.yampkg')]:
    shutil.copyfile(source,public/name)
guest=(root/'airplay-next-rebuild-20261113/install-217.sh').read_text()
digest=hashlib.sha256((public/'mini-217.yampkg').read_bytes()).hexdigest()
guest=guest.replace('4daffdf35d85a6a023f21d67fb653a8b61d40d12aa8361f4290762c247a24a8f',digest)
guest=guest.replace('2.1.7','2.1.8-panel.1').replace('18749','18753')
marker='stop_host\n/etc/init.d/S99luna start'
assert marker in guest
guest=guest.replace(marker,'''stop_host
wget -q -O /tmp/command.yampkg "$url/command.yampkg"
/data/yam/bin/yam --platform yandexmini --profiles /data/yam/platforms install -y /tmp/command.yampkg
/etc/init.d/S99luna start''')
guest+='''
wget -q -O /tmp/panel.yap "$url/panel.yap"
# Upload the Lua plugin through Luna's supported local-package endpoint.
pidof station-airplay > /tmp/panel-pid-before
wget -q -O /tmp/panel-test "$url/ready"
cat /tmp/panel-test
pidof station-airplay > /tmp/panel-pid-after
cmp /tmp/panel-pid-before /tmp/panel-pid-after
grep -q PANEL_TEST_PASS /tmp/panel-test
echo AIRPLAY_PID_UNCHANGED
'''
(public/'guest.sh').write_text(guest)

class Handler(http.server.SimpleHTTPRequestHandler):
    def do_GET(self):
        if self.path!='/ready':return super().do_GET()
        subprocess.run(['adb','connect','127.0.0.1:15823'],capture_output=True,timeout=10)
        # curl is on the host; multipart reaches the guest via a tiny
        # package-upload helper using its existing wget binary request.
        install=['wget','-S','-O','/tmp/ui-installed','--header=Content-Type: multipart/form-data; boundary=panel-boundary',
                 '--header=X-Luna-Local: 1','--post-file=/tmp/panel-upload','http://127.0.0.1:8099/api/plugins']
        import shlex
        payload=(b'--panel-boundary\r\nContent-Disposition: form-data; name="package"; filename="panel.yap"\r\n'
                 b'Content-Type: application/octet-stream\r\n\r\n'+(public/'panel.yap').read_bytes()+b'\r\n--panel-boundary--\r\n')
        (public/'panel-upload').write_bytes(payload)
        subprocess.run(['adb','-s','127.0.0.1:15823','push',str(public/'panel-upload'),'/tmp/panel-upload'],capture_output=True,check=True)
        uploaded=subprocess.run(['adb','-s','127.0.0.1:15823','shell',shlex.join(install)],
                                 capture_output=True,text=True,timeout=20)
        print('UPLOAD',uploaded.stdout[-600:],uploaded.stderr[-600:],flush=True)
        result=subprocess.run(['python3',str(src/'panel/check-panel-e2e.py')],capture_output=True,text=True,timeout=240)
        (out/'reports/e2e.log').write_text(result.stdout+result.stderr)
        print(result.stdout+result.stderr,flush=True)
        self.send_response(200);self.end_headers()
        self.wfile.write(b'PANEL_TEST_PASS' if result.returncode==0 else b'PANEL_TEST_FAIL')

server=http.server.ThreadingHTTPServer(('127.0.0.1',18753),functools.partial(Handler,directory=str(public)))
threading.Thread(target=server.serve_forever,daemon=True).start()
try:
    with (out/'reports/emulator.log').open('w') as log:
        result=subprocess.run([str(original/'target/release/kuyase'),'run','--headless','--board','yandexmini',
            '--snapshot','--persist-data','--port-offset','268','--exec',
            'wget -q -O /tmp/panel-test.sh http://10.0.2.2:18753/guest.sh && sh /tmp/panel-test.sh'],
            cwd=emu,stdout=log,stderr=subprocess.STDOUT,timeout=480)
    print((out/'reports/emulator.log').read_text()[-3000:])
finally:
    server.shutdown();server.server_close()
raise SystemExit(result.returncode)
