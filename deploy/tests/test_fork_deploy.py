"""Exercise deployment/rollback/drain behavior without a Docker daemon."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

SCRIPT = Path(__file__).resolve().parents[1] / "deploy-fork-image.sh"
FAKE_DOCKER = r'''#!/usr/bin/env python3
import json,os,pathlib,re,sys
p=pathlib.Path(os.environ['FORK_TEST_STATE'])
s=json.loads(p.read_text()); a=sys.argv[1:]
def save(): p.write_text(json.dumps(s))
if a[:2] == ['image','inspect']:
    if '--format' in a:
        print('https://github.com/MagicRealms/sub2api' if 'source' in a[-1] else 'a'*40)
elif a[0] == 'inspect':
    if '.Config.Image' in a[-1]: print(s['old'])
    else: print('unhealthy' if s['scenario']=='unhealthy' else 'healthy')
elif a[:2] == ['compose','config']:
    pass
elif a[:2] == ['compose','up']:
    image=re.search(r'^    image: (.+)$',pathlib.Path('docker-compose.override.yml').read_text(),re.M)[1]
    s['deploys'].append(image); save()
elif a[:2] == ['compose','exec']:
    if 'pg_dump' in a: print('database snapshot')
    elif 'tar' in a: print('application snapshot')
    elif 'psql' in a: print('test_admin_key_'+'x'*32)
    elif 'curl' in a:
        s['polls']+=1; save()
        active=1 if s['scenario']=='busy' or s['polls']==1 else 0
        print(json.dumps({'code':0,'data':{'account':{'1':{'current_in_use':active,'waiting_in_queue':0}}}}))
    else: raise SystemExit('unexpected exec '+str(a))
else: raise SystemExit('unexpected docker '+str(a))
'''


class ForkDeployTest(unittest.TestCase):
    def run_scenario(self, scenario, legacy=False):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            old = "weishaw/sub2api:0.1.169" if legacy else "magicrealms/sub2api:old"
            state = root / "state.json"
            state.write_text(json.dumps(dict(old=old, scenario=scenario, polls=0, deploys=[])))
            original = f"services:\n  sub2api:\n    image: {old}\n    mem_limit: 1536m\n  redis:\n    mem_limit: 256m\n"
            (root / "docker-compose.override.yml").write_text(original)
            (root / "docker-compose.yml").write_text("services: {}\n")
            (root / ".env").write_text("TEST_SETTING=retained\n")
            bindir = root / "bin"
            bindir.mkdir()
            (bindir / "docker").write_text(FAKE_DOCKER)
            (bindir / "sleep").write_text("#!/bin/sh\nexit 0\n")
            for p in bindir.iterdir(): p.chmod(0o755)
            env = os.environ | {"PATH": str(bindir) + os.pathsep + os.environ["PATH"], "FORK_TEST_STATE": str(state)}
            env.pop("FORK_ROLLBACK_IMAGE", None)
            result = subprocess.run(["bash", str(SCRIPT), "magicrealms/sub2api:new", tmp], env=env, capture_output=True, text=True, timeout=60)
            current = (root / "docker-compose.override.yml").read_text()
            self.assertIn("mem_limit: 1536m", current)
            self.assertIn("mem_limit: 256m", current)
            info = json.loads(state.read_text())
            if scenario == "healthy" and not legacy:
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(info["deploys"], ["magicrealms/sub2api:new"])
                self.assertGreaterEqual(info["polls"], 2)
                backups = list((root / "backups").glob("*/database.dump"))
                self.assertEqual(len(backups), 1)
                self.assertEqual(backups[0].read_text(), "database snapshot\n")
                self.assertEqual(backups[0].parent.stat().st_mode & 0o777, 0o700)
            else:
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(current, original)
                expected = ["magicrealms/sub2api:new", old] if scenario == "unhealthy" else []
                self.assertEqual(info["deploys"], expected)

    def test_backup_drain_and_deploy(self): self.run_scenario("healthy")
    def test_failed_health_restores_previous_image(self): self.run_scenario("unhealthy")
    def test_busy_requests_leave_deployment_unchanged(self): self.run_scenario("busy")
    def test_legacy_online_updated_image_requires_actual_rollback(self): self.run_scenario("healthy", legacy=True)


if __name__ == "__main__":
    unittest.main()
