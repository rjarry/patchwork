# Patchwork - automated patch tracking system
# Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
#
# SPDX-License-Identifier: GPL-2.0-or-later

"""
Git mirror management for forge sync operations.
"""

import contextlib
import email
import logging
import os
import re
import subprocess
import tempfile

from django.conf import settings

from patchwork.forge.util import bytes_to_mbox
from patchwork.forge.util import sender_identity

logger = logging.getLogger(__name__)


class GitMirror:
    """
    Bare mirror clone of a forge repository.

    Provides worktree-based operations for patch generation. All git commands
    run with isolated configuration (no user/system gitconfig, no interactive
    prompts) and temporary credential files that are removed after each
    operation.
    """

    def __init__(self, backend, forge_config):
        self.mirror_path = os.path.join(
            settings.FORGE_GIT_MIRROR_PATH,
            f'{forge_config.project.linkname}.git',
        )
        self.backend = backend
        self.forge_config = forge_config
        self.repo_url = backend.repo_url(forge_config)
        self.auth = backend.get_auth(forge_config)
        self.__worktree = None
        self.__credentials = None

    def repo_dir(self):
        if self.__worktree:
            return self.__worktree
        return self.mirror_path

    def git(self, *args, **kwargs):
        env = dict(os.environ)
        env.update(
            {
                'GIT_CONFIG_GLOBAL': '/dev/null',
                'GIT_CONFIG_SYSTEM': '/dev/null',
                'GIT_TERMINAL_PROMPT': '0',
            }
        )
        if self.__credentials:
            env['GIT_CREDENTIAL_HELPER'] = f'store --file={self.__credentials}'
        cmd = ['git']
        if os.path.isdir(self.repo_dir()):
            cmd += ['-C', self.repo_dir()]
        cmd.extend(args)
        logger.debug('+ %s', ' '.join(cmd))
        return subprocess.run(cmd, env=env, text=False, check=True, **kwargs)

    def git_output(self, *args, **kwargs):
        result = self.git(*args, capture_output=True, **kwargs)
        return result.stdout.decode('utf-8', errors='surrogateescape').strip()

    @contextlib.contextmanager
    def credentials(self):
        with tempfile.NamedTemporaryFile(
            prefix='patchwork-cred-', mode='w', delete_on_close=False
        ) as tmp:
            tmp.write(self.backend.git_credentials(self.forge_config))
            tmp.close()
            try:
                self.__credentials = tmp.name
                yield
            finally:
                self.__credentials = None

    def ensure_mirror(self, fetch_refs=None):
        """
        Create the bare mirror clone if it does not exist yet and configure the
        refspec to also fetch pull request heads.
        """
        head_path = os.path.join(self.mirror_path, 'HEAD')
        if not os.path.exists(head_path):
            logger.info('cloning mirror to %s', self.mirror_path)
            os.makedirs(os.path.dirname(self.mirror_path), exist_ok=True)
            with self.credentials():
                self.git('clone', '--mirror', self.repo_url, self.mirror_path)

        if fetch_refs:
            self.git(
                'config', '--replace-all', 'remote.origin.fetch', fetch_refs
            )

    def fetch(self):
        """
        Fetch all remotes and prune stale references.
        """
        logger.info('fetching mirror %s', self.mirror_path)
        with self.credentials():
            self.git('fetch', '--all', '--prune')

    def add_worktree(self, ref, path):
        """
        Create a temporary worktree checked out at the given ref.
        """
        self.git('worktree', 'add', '-fd', '--checkout', path, ref)

    def del_worktree(self, path):
        """
        Remove a previously created worktree.
        """
        self.git('worktree', 'remove', '-ff', path)

    @contextlib.contextmanager
    def worktree(self, ref):
        w = tempfile.mkdtemp(prefix='patchwork-worktree-')
        try:
            self.add_worktree(ref, w)
            self.__worktree = w
            yield
        finally:
            self.__worktree = None
            self.del_worktree(w)

    def commit_count(self, base_ref):
        """
        Return the number of commits in base_ref..HEAD.
        """
        out = self.git_output('rev-list', '--count', f'{base_ref}..HEAD')
        return int(out)

    def ref_exists(self, ref):
        """
        Return True if ref exists in the repository.
        """
        try:
            self.git_output('cat-file', '-t', ref)
            return True
        except subprocess.CalledProcessError:
            return False

    RECIPIENT_RE = re.compile(r'\s*\d+\s+(?P<name>.+)\s+<(?P<email>.+@.+)>')

    def recipients(self, base_ref):
        out = self.git_output(
            'shortlog',
            '-se',
            '-w0',
            '--group=author',
            '--group=committer',
            '--group=trailer:cc',
            '--group=trailer:acked-by',
            '--group=trailer:co-authored-by',
            '--group=trailer:reported-by',
            '--group=trailer:requested-by',
            '--group=trailer:reviewed-by',
            '--group=trailer:signed-off-by',
            '--group=trailer:suggested-by',
            '--group=trailer:tested-by',
            f'{base_ref}..HEAD',
        )
        recipients = {}
        for m in self.RECIPIENT_RE.finditer(out):
            name = m.group('name')
            name = re.sub(r'\w\w+', lambda s: s.group(0).title(), name)
            name = name.strip('"\' \t')
            addr = m.group('email').lower()
            recipients[addr] = name
        for addr, name in recipients.items():
            yield email.utils.formataddr((name, addr))

    def format_patches(
        self,
        base_ref,
        user,
        version=1,
        cover_title=None,
        cover_body=None,
        range_diff_base=None,
        in_reply_to=None,
    ):
        """
        Generate patches for commits in base_ref..HEAD.

        When the series has more than one commit and a cover_title is provided,
        a cover letter is generated. For respins (version > 1), --in-reply-to
        threads the cover letter under the original and --range-diff shows what
        changed since the previous version.

        Returns mailbox.mbox object containing all messages.
        """
        name, addr = sender_identity(user, self.forge_config)
        args = [
            '-c',
            f'user.name={name}',
            '-c',
            f'user.email={addr}',
            'format-patch',
            '--stdout',
            '--thread=shallow',
            f'--subject-prefix=PATCH {self.forge_config.project.linkname}',
            f'--to={self.forge_config.project.listemail}',
        ]

        for cc in self.recipients(base_ref):
            args.append(f'--cc={cc}')

        extra_headers = {
            'Sender': self.forge_config.sender_email,
            'Reply-To': self.forge_config.project.listemail,
            'List-ID': f'<{self.forge_config.project.listid}>',
            'X-Patchwork-Hint': 'ignore',
        }
        for key, value in extra_headers.items():
            args.append(f'--add-header={key}: {value}')

        if in_reply_to:
            args.append(f'--in-reply-to={in_reply_to}')

        if version > 1:
            args.append(f'-v{version}')

        if self.commit_count(base_ref) > 1 and cover_title:
            args.append('--cover-letter')
            if cover_body:
                desc = f'{cover_title}\n\n{cover_body}'
            else:
                desc = cover_title
            desc_file = os.path.join(self.repo_dir(), '.cover-description')
            with open(desc_file, 'w') as f:
                f.write(desc)
            args += [
                '--cover-from-description=subject',
                f'--description-file={desc_file}',
            ]

        if range_diff_base and self.ref_exists(range_diff_base):
            args.append(f'--range-diff={base_ref}..{range_diff_base}')

        args.append(f'{base_ref}..HEAD')

        result = self.git(*args, capture_output=True)
        return bytes_to_mbox(result.stdout)

    def apply_mbox(self, mbox_text):
        """
        Apply patches from an mbox string via git am -3.
        """
        self.git('am', '-3', input=mbox_text)

    def push(self, branch):
        """
        Force-push HEAD to refs/heads/<branch> on the remote.
        """
        with self.credentials():
            self.git('push', '-f', self.repo_url, f'HEAD:refs/heads/{branch}')
