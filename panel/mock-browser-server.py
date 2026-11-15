import base64
import copy
import json
import threading
import time
import uuid
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

config={'timing_mode':'fast_audio','led_level':200,'spectrum':False,
        'spectrum_gain_r':1,'spectrum_gain_g':1,'spectrum_gain_b':1,
        'spectrum_brightness':1,'spectrum_saturation':1,
        'equalizer':{'enabled':False,'preamp_db':0,'bands_db':[0,0,0,0,0]}}
presets={};log='';records={};expires=0;lease=''
page=Path(__file__).with_name('index.html').read_bytes()
class Handler(BaseHTTPRequestHandler):
    def respond(self,status,value):
        body=value if isinstance(value,bytes) else json.dumps(value).encode()
        self.send_response(status);self.send_header('Content-Type','application/json')
        self.end_headers();self.wfile.write(body)
    def do_GET(self):
        if self.path=='/web/airplay.panel':
            self.send_response(200);self.send_header('Content-Type','text/html; charset=utf-8');self.end_headers();self.wfile.write(page)
        elif self.path=='/api/device':self.respond(200,{'model':'yandexmini','room':'Browser fixture'})
        elif self.path.endswith('/logs'):
            self.send_response(200);self.end_headers();self.wfile.write(log.encode())
        elif self.path=='/favicon.ico':self.respond(204,b'')
        else:self.respond(404,{'error':'not found'})
    def do_POST(self):
        global config,presets,log,expires,lease
        if self.path=='/shutdown':
            self.respond(200,{})
            threading.Thread(target=self.server.shutdown,daemon=True).start()
            return
        invocation=json.loads(self.rfile.read(int(self.headers.get('Content-Length','0'))))
        invocation=json.loads(invocation['args'][0])
        request=invocation['request'];op=request['operation'];method=request['method'];body=request.get('body')
        if request['id'] in records:
            reply=copy.deepcopy(records[request['id']])
        else:
            result={};status=200
            if op=='status':
                result={'model':'yandexmini','led_count':6,'defaults':config,
                        'zones':[{'id':'artwork','count':6,'supported':True}],
                        'preview':{'active':time.monotonic()<expires,'allowed':True}}
            elif op=='settings':
                if method=='PUT':config=body;result={'saved':True,'restart_required':True}
                else:result=config
            elif op=='presets':
                if method=='PUT':presets=body;result={'saved':True}
                else:result=presets
            elif op=='preview':
                if method=='POST':
                    lease=body.get('id') or uuid.uuid4().hex;expires=time.monotonic()+4
                    result={'id':lease,'ttl_ms':4000,'sent':body['colors']}
                else:expires=0;result={'stopped':True}
            reply={'id':request['id'],'epoch':'browser-fixture','status':status,'body':copy.deepcopy(result)}
            records[request['id']]=copy.deepcopy(reply)
        reply['attempt']=invocation['attempt']
        log+='\nAP1 '+base64.b64encode(json.dumps(reply).encode()).decode()+'\n'
        self.respond(202,b'')

server=ThreadingHTTPServer(('127.0.0.1',18760),Handler)
server.serve_forever()
