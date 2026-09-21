"""Native Unix terminal smoke tests; synthetic credentials in temporary HOME only."""
import os,pathlib,pty,select,signal,subprocess,sys,tempfile,termios,time,unittest
BINARY=str(pathlib.Path(sys.argv.pop(1)).resolve())
class TerminalTests(unittest.TestCase):
 def start(self,home):
  master,slave=pty.openpty();before=termios.tcgetattr(slave)
  env=dict(os.environ,HOME=home,XDG_CONFIG_HOME=home+'/.config')
  p=subprocess.Popen([BINARY,'auth','login','--storage','file'],stdin=slave,stdout=slave,stderr=slave,env=env)
  return master,slave,before,p
 def read_until(self,fd,word):
  data=b'';end=time.monotonic()+5
  while word not in data and time.monotonic()<end:
   if select.select([fd],[],[],.1)[0]:data+=os.read(fd,4096)
  self.assertIn(word,data);return data
 def test_cancel_restores_echo(self):
  with tempfile.TemporaryDirectory() as home:
   master,slave,before,p=self.start(home)
   try:
    self.read_until(master,b'API key:');time.sleep(.05);p.send_signal(signal.SIGINT);p.wait(timeout=5)
    self.assertEqual(termios.tcgetattr(slave)[3]&termios.ECHO,before[3]&termios.ECHO)
    self.assertFalse(pathlib.Path(home+'/.config/tofa/config.yml').exists())
   finally:
    if p.poll() is None:p.kill();p.wait()
    os.close(master);os.close(slave)
 def test_login_masks_key_and_logout_retains_preferences(self):
  with tempfile.TemporaryDirectory() as home:
   master,slave,before,p=self.start(home)
   try:
    self.read_until(master,b'API key:');time.sleep(.05);os.write(master,b'fixture-token\n')
    output=self.read_until(master,b'Project ID:');self.assertNotIn(b'fixture-token',output)
    os.write(master,b'fixture-project\n');self.read_until(master,b'Credentials saved');self.assertEqual(p.wait(timeout=5),0)
    env=dict(os.environ,HOME=home,XDG_CONFIG_HOME=home+'/.config')
    subprocess.run([BINARY,'auth','logout'],env=env,check=True,capture_output=True)
    config=pathlib.Path(home+'/.config/tofa/config.yml').read_text()
    self.assertIn('fixture-project',config);self.assertNotIn('fixture-token',config)
    self.assertFalse(pathlib.Path(home+'/.config/tofa/credentials.yml').exists())
   finally:
    if p.poll() is None:p.kill();p.wait()
    os.close(master);os.close(slave)
if __name__=='__main__':unittest.main()
