"""Offline installer lifecycle test. Only fixture files under a temporary HOME."""
import hashlib, os, pathlib, subprocess, tempfile, unittest
ROOT = pathlib.Path(__file__).resolve().parent
class InstallerTest(unittest.TestCase):
 def test_invalid_checksum_manifests_preserve_existing_installation(self):
  with tempfile.TemporaryDirectory(prefix='tofa checksum ') as temp:
   home=pathlib.Path(temp); assets=home/'assets'; assets.mkdir()
   version='v0.1.0-rc.1'
   platform=subprocess.check_output(['uname','-s'],text=True).strip().lower()
   arch={'arm64':'arm64','aarch64':'arm64','x86_64':'amd64'}[subprocess.check_output(['uname','-m'],text=True).strip()]
   filename=f'tofa_{version}_{platform}_{arch}'
   binary=assets/filename; binary.write_text('#!/bin/sh\necho original\n')
   checksum=assets/'SHA256SUMS'
   checksum.write_text(hashlib.sha256(binary.read_bytes()).hexdigest()+'  '+filename+'\n')
   install=home/'install'
   env=dict(os.environ,HOME=str(home),TOFA_INSTALL_DIR=str(install),TOFA_RELEASE_BASE_URL=assets.as_uri())
   command=['sh',str(ROOT/'install.sh'),'--version',version,'--no-modify-path']
   subprocess.run(command,env=env,check=True,capture_output=True)
   before={p.relative_to(install):p.read_bytes() for p in install.rglob('*') if p.is_file()}
   binary.write_text('#!/bin/sh\necho replacement\n')
   valid=hashlib.sha256(binary.read_bytes()).hexdigest()+'  '+filename+'\n'
   cases={
    'missing': '',
    'other asset': valid.replace(filename,'another-binary'),
    'duplicate': valid+valid,
    'malformed duplicate': valid+'invalid  '+filename+'\n',
    'incorrect': '0'*64+'  '+filename+'\n',
    'extra fields': valid.rstrip()+' unexpected\n',
   }
   for label,content in cases.items():
    with self.subTest(label=label):
     checksum.write_text(content)
     result=subprocess.run(command,env=env,text=True,capture_output=True)
     self.assertNotEqual(result.returncode,0,result.stdout+result.stderr)
     self.assertIn('checksum',result.stderr.lower())
     self.assertEqual({p.relative_to(install):p.read_bytes() for p in install.rglob('*') if p.is_file()},before)

 def test_install_upgrade_uninstall_preserves_then_purges(self):
  with tempfile.TemporaryDirectory(prefix="tofa test ' ") as temp:
   home=pathlib.Path(temp); assets=home/'assets'; assets.mkdir()
   version='v0.0.0-test'; platform=subprocess.check_output(['uname','-s'],text=True).strip().lower()
   arch={'arm64':'arm64','aarch64':'arm64','x86_64':'amd64'}[subprocess.check_output(['uname','-m'],text=True).strip()]
   filename=f'tofa_{version}_{platform}_{arch}'
   binary=assets/filename; binary.write_text('#!/bin/sh\necho tofa-fixture\n')
   (assets/'SHA256SUMS').write_text(hashlib.sha256(binary.read_bytes()).hexdigest()+'  '+filename+'\n')
   env=dict(os.environ,HOME=str(home),XDG_CONFIG_HOME=str(home/'.config'),SHELL='/bin/zsh',ZDOTDIR=str(home),TOFA_RELEASE_BASE_URL=assets.as_uri())
   def run(script,*args,success=True):
    p=subprocess.run(['sh',str(ROOT/script),*args],env=env,text=True,capture_output=True)
    if success:self.assertEqual(p.returncode,0,p.stdout+p.stderr)
    else:self.assertNotEqual(p.returncode,0)
    return p
   run('install.sh','--version',version)
   installed=home/'.local/share/tofa/bin/tofa'; self.assertTrue(installed.exists())
   activated=subprocess.check_output(['sh','-c','. "$HOME/.zshrc"; command -v tofa'],env=env,text=True).strip()
   self.assertEqual(activated,str(installed))
   run('install.sh','--version',version)
   self.assertEqual((home/'.zshrc').read_text().count('# >>> tofa >>>'),1)
   config=home/'.config/tofa';config.mkdir(parents=True);(config/'config.yml').write_text('version: 1\n')
   (config/'credentials.yml').write_text('synthetic: dummy-key\n')
   (config/'unrelated.txt').write_text('keep')
   # Damaged startup markers must not truncate unrelated shell configuration.
   rc=home/'.zshrc';original=rc.read_text();rc.write_text(original.replace('# <<< tofa <<<','')+'\necho preserve-me\n')
   damaged=rc.read_text()
   run('uninstall.sh',success=False)
   self.assertEqual(rc.read_text(),damaged)
   self.assertTrue(installed.exists())
   rc.write_text(original)
   binary.write_text('corrupt fixture')
   run('install.sh','--version',version,success=False)
   self.assertIn('tofa-fixture',installed.read_text())
   run('uninstall.sh')
   self.assertFalse(installed.exists());self.assertTrue((config/'credentials.yml').exists())
   self.assertNotIn('# >>> tofa >>>',(home/'.zshrc').read_text())
   run('uninstall.sh','--purge')
   self.assertFalse((config/'credentials.yml').exists());self.assertTrue((config/'unrelated.txt').exists())
if __name__=='__main__':unittest.main()
