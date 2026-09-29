"""Explicit first-session diagnostic; skipped baseline cases stay unattempted."""
import hashlib,os,pathlib,sys
artifact=pathlib.Path(__file__).resolve().parent
os.chdir(artifact/'source');sys.path.insert(0,str(artifact/'source/scripts'))
import desktop_evaluation as evaluation
from model_evaluation import main
original=evaluation.case
def first_session(options,run,entry):
 options.evidence['workload']='first-coding-session-only'
 options.evidence['diagnostic_driver_sha256']=hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest()
 if entry['id']=='coding-1': original(options,run,entry)
evaluation.case=first_session
sys.argv=['model_evaluation.py','--desktop','--launcher',str(artifact/'tofa'),'--codex',str(artifact/'codex'),'--model','zai-org/GLM-5.3-Flash','--guardian-model','zai-org/GLM-5.3-Flash','--campaign',str(artifact/'campaign'),'--qualification-evidence',str(artifact/'controlled.json'),'--diagnostic','first-session-file-details','--output',str(artifact/'diagnostic.json')]
sys.exit(main())
