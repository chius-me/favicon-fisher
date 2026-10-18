"""Offline release regression tests. Apple/GitHub are stubbed, hashes and Git are real."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


class ReleaseTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="fvf-release-test-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.bin = self.root / "bin"
        self.bin.mkdir()
        self.assets = self.root / "assets"
        self.assets.mkdir()
        self.output = self.root / "output"
        self.tag = "v1.2.3"
        self.dmg = "Favicon-Fisher-1.2.3-universal.dmg"
        (self.assets / self.dmg).write_bytes(b"unsigned test fixture, not a disk image")
        self.digest = hashlib.sha256((self.assets / self.dmg).read_bytes()).hexdigest()
        (self.assets / "checksums.txt").write_text(f"{self.digest}  {self.dmg}\n")
        self.release = {"tagName": self.tag, "isDraft": False, "isPrerelease": False}
        self.metadata = self.root / "release.json"
        self.metadata.write_text(json.dumps(self.release))
        self.log = self.root / "gh.log"
        self.stub("gh", '''
import json, os, pathlib, shutil, sys
args = sys.argv[1:]
with open(os.environ["TEST_GH_LOG"], "a") as f:
    f.write(json.dumps(args) + "\\n")
if args[:2] == ["release", "view"]:
    print(pathlib.Path(os.environ["TEST_RELEASE"]).read_text())
elif args[:2] == ["release", "download"]:
    for file in pathlib.Path(os.environ["TEST_ASSETS"]).iterdir():
        shutil.copy(file, pathlib.Path(args[args.index("--dir") + 1]) / file.name)
elif args[:2] == ["pr", "list"]:
    print(os.environ.get("TEST_EXISTING_PR", ""))
elif args[:2] == ["pr", "create"]:
    print("https://github.com/chius-me/homebrew-tap/pull/123")
else:
    sys.exit("unexpected gh call")
''')
        self.stub("xcrun", '''
import os, sys
assert sys.argv[1:3] == ["stapler", "validate"], sys.argv
sys.exit(int(os.environ.get("TEST_STAPLER_STATUS", "0")))
''')
        self.stub("hdiutil", '''
import os, sys
assert sys.argv[1] == "verify", sys.argv
sys.exit(int(os.environ.get("TEST_DMG_STATUS", "0")))
''')
        self.env = dict(os.environ, PATH=str(self.bin) + os.pathsep + os.environ["PATH"],
                        TEST_GH_LOG=str(self.log), TEST_RELEASE=str(self.metadata),
                        TEST_ASSETS=str(self.assets), GIT_CONFIG_NOSYSTEM="1",
                        GIT_CONFIG_GLOBAL=os.devnull, GIT_TERMINAL_PROMPT="0")

    def stub(self, name, source):
        path = self.bin / name
        path.write_text(f"#!{sys.executable}\n" + source)
        path.chmod(0o755)

    def run_script(self, name, *args, success=True):
        result = subprocess.run(["bash", str(ROOT / "scripts" / name), *map(str, args)],
                                env=self.env, capture_output=True, text=True)
        if success:
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        else:
            self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        return result

    def prepare(self, success=True, mode="ad-hoc"):
        return self.run_script("prepare-homebrew-cask.sh", self.tag, self.output, mode, success=success)

    def test_verified_asset_generates_actual_checksum(self):
        self.prepare()
        cask = (self.output / "favicon-fisher.rb").read_text()
        self.assertIn(f'sha256 "{self.digest}"', cask)
        self.assertIn('version "1.2.3"', cask)
        self.assertIn('depends_on macos: :sonoma', cask)
        self.assertIn('uninstall quit: "dev.chius.favicon-fisher"', cask)
        self.assertNotIn(":no_check", cask)
        self.assertIn("not notarized by Apple", cask)

    def test_free_mode_does_not_require_apple_credentials(self):
        self.env["TEST_STAPLER_STATUS"] = "1"
        self.prepare()

    def test_notarized_mode_validates_ticket_and_omits_free_caveat(self):
        self.prepare(mode="notarized")
        self.assertNotIn("not notarized", (self.output / "favicon-fisher.rb").read_text())

    def test_invalid_disk_image_rejected(self):
        self.env["TEST_DMG_STATUS"] = "1"
        self.prepare(success=False)
        self.assertFalse(self.output.exists())

    def test_invalid_tags_fail_before_network(self):
        for tag in ["v1.2.3-beta", "v01.2.3", "v1.2", "../main", "v1.2.3;echo bad"]:
            self.run_script("prepare-homebrew-cask.sh", tag, self.output, success=False)
        self.assertFalse(self.log.exists())

    def test_unpublished_and_mismatched_release_rejected(self):
        for override in [{"isDraft": True}, {"isPrerelease": True}, {"tagName": "v9.9.9"}]:
            self.metadata.write_text(json.dumps(dict(self.release, **override)))
            self.prepare(success=False)
        self.assertFalse(self.output.exists())

    def test_missing_duplicate_or_wrong_checksums_rejected(self):
        valid = f"{self.digest}  {self.dmg}\n"
        for contents in ["", valid * 2, f"{'0' * 64}  {self.dmg}\n", f"{self.digest}  ../{self.dmg}\n"]:
            (self.assets / "checksums.txt").write_text(contents)
            self.prepare(success=False)
        self.assertFalse(self.output.exists())

    def test_failed_staple_preserves_existing_output(self):
        self.output.mkdir()
        cask = self.output / "favicon-fisher.rb"
        cask.write_text("existing file")
        self.env["TEST_STAPLER_STATUS"] = "1"
        self.prepare(success=False, mode="notarized")
        self.assertEqual(cask.read_text(), "existing file")

    def git(self, *args, cwd=None):
        result = subprocess.run(["git", *map(str, args)], cwd=cwd, env=self.env,
                                capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        return result.stdout.strip()

    def tap(self, version=None):
        remote = self.root / "tap.git"
        checkout = self.root / "tap"
        self.git("init", "--bare", "--initial-branch=main", remote)
        self.git("clone", remote, checkout)
        (checkout / "README.md").write_text("Test tap\n")
        if version:
            (checkout / "Casks").mkdir()
            (checkout / "Casks/favicon-fisher.rb").write_text(f'cask "favicon-fisher" do\n  version "{version}"\nend\n')
        self.git("add", ".", cwd=checkout)
        self.git("-c", "user.name=Test", "-c", "user.email=test@example.invalid",
                 "commit", "-m", "Initialize", cwd=checkout)
        self.git("push", "origin", "main", cwd=checkout)
        return checkout

    def update(self, checkout, success=True):
        return self.run_script("update-homebrew-tap.sh", self.tag, self.output / "favicon-fisher.rb",
                               checkout, "pr", success=success)

    def test_local_update_needs_no_token_commit_or_push(self):
        self.prepare()
        checkout = self.tap()
        before = self.git("rev-parse", "HEAD", cwd=checkout)
        calls = self.log.read_text()
        self.run_script("update-homebrew-tap.sh", self.tag, self.output / "favicon-fisher.rb", checkout)
        self.assertEqual(self.git("rev-parse", "HEAD", cwd=checkout), before)
        self.assertEqual(self.git("branch", "--show-current", cwd=checkout), "main")
        self.assertEqual(self.log.read_text(), calls)
        self.assertEqual((checkout / "Casks/favicon-fisher.rb").read_bytes(),
                         (self.output / "favicon-fisher.rb").read_bytes())

    def test_tap_pr_creation_and_retry_reuse_branch(self):
        self.prepare()
        checkout = self.tap()
        self.update(checkout)
        first = self.git("rev-parse", "HEAD", cwd=checkout)
        second_checkout = self.root / "retry"
        self.git("clone", self.root / "tap.git", second_checkout)
        self.env["TEST_EXISTING_PR"] = "https://github.com/chius-me/homebrew-tap/pull/123"
        self.update(second_checkout)
        self.assertEqual(first, self.git("rev-parse", "HEAD", cwd=second_checkout))
        calls = [json.loads(line) for line in self.log.read_text().splitlines()]
        self.assertEqual(sum(args[:2] == ["pr", "create"] for args in calls), 1)

    def test_tap_downgrade_rejected(self):
        self.prepare()
        checkout = self.tap("2.0.0")
        self.update(checkout, success=False)
        self.assertEqual(self.git("branch", "--show-current", cwd=checkout), "main")

    def test_existing_version_replacement_rejected(self):
        self.prepare()
        self.update(self.tap("1.2.3"), success=False)

    def test_already_merged_version_is_noop(self):
        self.prepare()
        checkout = self.tap()
        self.update(checkout)
        self.git("switch", "main", cwd=checkout)
        self.git("merge", "--ff-only", "codex/favicon-fisher-1.2.3", cwd=checkout)
        before = self.log.read_text()
        self.update(checkout)
        self.assertEqual(self.log.read_text(), before)

    def test_dirty_checkout_rejected(self):
        self.prepare()
        checkout = self.tap()
        (checkout / "README.md").write_text("Local user changes\n")
        self.update(checkout, success=False)
        self.assertEqual((checkout / "README.md").read_text(), "Local user changes\n")


if __name__ == "__main__":
    unittest.main()
