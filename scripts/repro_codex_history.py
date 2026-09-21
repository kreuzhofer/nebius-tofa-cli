"""Synthetic two-turn Codex replay: local server, temporary config, dummy key."""
import http.server,json,os,shutil,subprocess,tempfile,threading
model=os.environ.get("TOFA_REPRO_MODEL","moonshotai/Kimi-K3")
requests=[]
class Handler(http.server.BaseHTTPRequestHandler):
 def log_message(self,*args):pass
 def do_POST(self):
  requests.append(json.loads(self.rfile.read(int(self.headers['Content-Length']))))
  part={'type':'output_text','text':'Hi!','annotations':[]}
  message={'id':'msg_fixture','type':'message','role':'assistant','status':'completed','content':[part]}
  response={'id':'resp_fixture_'+str(len(requests)),'object':'response','status':'completed','model':model,'output':[message],'usage':{'input_tokens':10,'output_tokens':2,'total_tokens':12}}
  events=[('response.created',{'response':dict(response,status='in_progress',output=[])}),('response.output_item.added',{'output_index':0,'item':dict(message,status='in_progress',content=[])}),('response.content_part.added',{'item_id':'msg_fixture','output_index':0,'content_index':0,'part':dict(part,text='')}),('response.output_text.delta',{'item_id':'msg_fixture','output_index':0,'content_index':0,'delta':'Hi!'}),('response.output_text.done',{'item_id':'msg_fixture','output_index':0,'content_index':0,'text':'Hi!'}),('response.content_part.done',{'item_id':'msg_fixture','output_index':0,'content_index':0,'part':part}),('response.output_item.done',{'output_index':0,'item':message}),('response.completed',{'response':response})]
  data=''.join('event: '+name+'\ndata: '+json.dumps(dict(body,type=name))+'\n\n' for name,body in events).encode()
  self.send_response(200);self.send_header('Content-Type','text/event-stream');self.send_header('Content-Length',str(len(data)));self.end_headers();self.wfile.write(data)
server=http.server.ThreadingHTTPServer(('127.0.0.1',0),Handler)
threading.Thread(target=server.serve_forever,daemon=True).start()
try:
 with tempfile.TemporaryDirectory(prefix='tofa-codex-repro-') as root:
  env={k:v for k,v in os.environ.items() if k in ('PATH','TMPDIR','SYSTEMROOT','WINDIR')}
  os.mkdir(root+'/codex')
  env.update(HOME=root,CODEX_HOME=root+'/codex',TOFA_FIXTURE_KEY='synthetic-key',OTEL_SDK_DISABLED='true')
  provider='{ name="Fixture", base_url="http://127.0.0.1:'+str(server.server_port)+'/v1", env_key="TOFA_FIXTURE_KEY", wire_api="responses", requires_openai_auth=false, supports_websockets=false }'
  base=[shutil.which('codex'),'-c','model_provider="fixture"','-c','model='+json.dumps(model),'-c','model_providers.fixture='+provider,'-c','web_search="disabled"']
  for args in (['exec','--skip-git-repo-check','--json','Hi'],['exec','resume','--last','--skip-git-repo-check','--json','What is this repository about?']):
   p=subprocess.run(base+args,env=env,cwd=root,capture_output=True,text=True,timeout=40)
   print('Codex exit:',p.returncode);print(p.stdout[-1200:]);print(p.stderr[-1200:])
   if p.returncode:raise SystemExit('fixture did not reach expected response path')
  if len(requests)!=2:raise SystemExit('expected two local requests, got '+str(len(requests)))
  history=[i for i in requests[1]['input'] if isinstance(i,dict) and i.get('role')=='assistant']
  print('Second-turn assistant history:',json.dumps(history,indent=2));assert history,'no assistant history'
  missing=[]
  for item in history:
   if 'status' not in item:missing.append('message.status')
   for content in item.get('content',[]):
    if content.get('type')=='output_text' and 'annotations' not in content:missing.append('output_text.annotations')
  print('Fields required by reported Nebius error but omitted by Codex:',missing)
  assert not missing,'REPRODUCED: '+', '.join(missing)
finally:server.shutdown();server.server_close()
