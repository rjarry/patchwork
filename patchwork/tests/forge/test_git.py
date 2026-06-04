# Patchwork - automated patch tracking system
# Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
#
# SPDX-License-Identifier: GPL-2.0-or-later

import os
import shutil
import subprocess
import tempfile
import unittest

from django.test import TestCase
from django.test import override_settings

from patchwork.forge import ForgeUser
from patchwork.forge.git import GitMirror


def _has_git():
    try:
        subprocess.run(
            ['git', '--version'],
            capture_output=True,
            check=True,
        )
        return True
    except (FileNotFoundError, subprocess.CalledProcessError):
        return False


def _run_git(*args, cwd=None):
    env = dict(os.environ)
    env.update(
        {
            'GIT_CONFIG_GLOBAL': '/dev/null',
            'GIT_CONFIG_SYSTEM': '/dev/null',
            'GIT_TERMINAL_PROMPT': '0',
            'GIT_AUTHOR_NAME': 'Test Author',
            'GIT_AUTHOR_EMAIL': 'author@example.com',
            'GIT_COMMITTER_NAME': 'Test Author',
            'GIT_COMMITTER_EMAIL': 'author@example.com',
        }
    )
    return subprocess.run(
        ['git'] + list(args),
        cwd=cwd,
        env=env,
        capture_output=True,
        text=True,
        check=True,
    )


@unittest.skipUnless(_has_git(), 'git is not installed')
class GitMirrorTestBase(TestCase):
    """
    Base class that creates a bare "upstream" repo with a few commits
    and a GitMirror clone of it.
    """

    @classmethod
    def setUpClass(cls):
        super().setUpClass()
        cls.tmpdir = tempfile.mkdtemp(prefix='patchwork-test-git-')

        # create upstream bare repo
        cls.upstream_path = os.path.join(cls.tmpdir, 'upstream.git')
        work = os.path.join(cls.tmpdir, 'work')
        os.makedirs(work)
        _run_git('init', cwd=work)
        _run_git('commit', '--allow-empty', '-m', 'initial', cwd=work)

        # tag the base for commit range
        _run_git('tag', 'base', cwd=work)

        # add commits
        with open(os.path.join(work, 'a.txt'), 'w') as f:
            f.write('aaa\n')
        _run_git('add', 'a.txt', cwd=work)
        _run_git(
            'commit',
            '-m',
            'add file a\n\nSigned-off-by: Test Author <author@example.com>',
            cwd=work,
        )

        with open(os.path.join(work, 'b.txt'), 'w') as f:
            f.write('bbb\n')
        _run_git('add', 'b.txt', cwd=work)
        _run_git(
            'commit',
            '-m',
            'add file b\n\nAcked-by: Reviewer <reviewer@example.com>',
            cwd=work,
        )

        # clone as bare
        _run_git('clone', '--mirror', work, cls.upstream_path)
        shutil.rmtree(work)

    @classmethod
    def tearDownClass(cls):
        shutil.rmtree(cls.tmpdir, ignore_errors=True)
        super().tearDownClass()

    def _make_mirror(self):
        from unittest.mock import MagicMock

        backend = MagicMock()
        backend.repo_url.return_value = self.upstream_path
        backend.get_auth.return_value = {}
        backend.git_credentials.return_value = ''

        forge_config = MagicMock()
        forge_config.project.linkname = 'mirror'
        forge_config.project.listemail = 'list@example.com'
        forge_config.project.listid = 'list.example.com'
        forge_config.sender_email = 'patchwork@example.com'

        with override_settings(FORGE_GIT_MIRROR_PATH=self.tmpdir):
            mirror = GitMirror(backend, forge_config)

        return mirror


class CommitCountTest(GitMirrorTestBase):
    def test_count_commits(self):
        mirror = self._make_mirror()
        mirror.ensure_mirror()
        with mirror.worktree('HEAD'):
            count = mirror.commit_count('base')
            self.assertEqual(count, 2)


class RefExistsTest(GitMirrorTestBase):
    def test_existing_ref(self):
        mirror = self._make_mirror()
        mirror.ensure_mirror()
        with mirror.worktree('HEAD'):
            self.assertTrue(mirror.ref_exists('base'))

    def test_missing_ref(self):
        mirror = self._make_mirror()
        mirror.ensure_mirror()
        with mirror.worktree('HEAD'):
            self.assertFalse(mirror.ref_exists('nonexistent'))


class TempWorktreeTest(GitMirrorTestBase):
    def test_creates_and_cleans_up(self):
        mirror = self._make_mirror()
        mirror.ensure_mirror()
        w = None
        with mirror.worktree('HEAD'):
            w = mirror.repo_dir()
            self.assertTrue(os.path.isdir(w))
            self.assertTrue(os.path.exists(os.path.join(w, 'a.txt')))
            self.assertTrue(os.path.exists(os.path.join(w, 'b.txt')))
        self.assertFalse(os.path.isdir(w))


class RecipientsTest(GitMirrorTestBase):
    def test_extracts_recipients(self):
        mirror = self._make_mirror()
        mirror.ensure_mirror()
        with mirror.worktree('HEAD'):
            recipients = list(mirror.recipients('base'))
            self.assertIn('Test Author <author@example.com>', recipients)
            self.assertIn('Reviewer <reviewer@example.com>', recipients)


class FormatPatchesTest(GitMirrorTestBase):
    def test_single_patch(self):
        mirror = self._make_mirror()
        mirror.ensure_mirror()
        with mirror.worktree('HEAD'):
            user = ForgeUser(
                login='author', name='Test Author', email='author@example.com'
            )
            mbox = mirror.format_patches('HEAD~1', user)
            messages = list(mbox)
            self.assertEqual(len(messages), 1)
            self.assertIn('[PATCH', messages[0].get('Subject'))

    def test_multi_patch_with_cover(self):
        mirror = self._make_mirror()
        mirror.ensure_mirror()
        with mirror.worktree('HEAD'):
            user = ForgeUser(
                login='author', name='Test Author', email='author@example.com'
            )
            mbox = mirror.format_patches(
                'base',
                user,
                cover_title='Test series',
                cover_body='This is a test.',
            )
            messages = list(mbox)
            # cover letter + 2 patches
            self.assertEqual(len(messages), 3)
            subjects = [m.get('Subject') for m in messages]
            self.assertTrue(
                any('0/2' in s for s in subjects),
                f'no cover letter: {subjects}',
            )

    def test_version_numbering(self):
        mirror = self._make_mirror()
        mirror.ensure_mirror()
        with mirror.worktree('HEAD'):
            user = ForgeUser(
                login='author', name='Test Author', email='author@example.com'
            )
            mbox = mirror.format_patches('base', user, version=2)
            messages = list(mbox)
            subjects = [m.get('Subject') for m in messages]
            self.assertTrue(
                all('v2' in s for s in subjects), f'no v2: {subjects}'
            )

    def test_extra_headers(self):
        mirror = self._make_mirror()
        mirror.ensure_mirror()
        with mirror.worktree('HEAD'):
            user = ForgeUser(
                login='author', name='Test Author', email='author@example.com'
            )
            mbox = mirror.format_patches('HEAD~1', user)
            messages = list(mbox)
            msg = messages[0]
            self.assertIn('ignore', msg.get('X-Patchwork-Hint', ''))
            self.assertIn('list.example.com', msg.get('List-ID', ''))
            self.assertIn('list@example.com', msg.get('Reply-To', ''))
            self.assertIn('patchwork@example.com', msg.get('Sender', ''))

    def test_in_reply_to(self):
        mirror = self._make_mirror()
        mirror.ensure_mirror()
        with mirror.worktree('HEAD'):
            user = ForgeUser(
                login='author', name='Test Author', email='author@example.com'
            )
            mbox = mirror.format_patches(
                'base',
                user,
                cover_title='Test',
                in_reply_to='<v1-cover@example.com>',
            )
            messages = list(mbox)
            cover = messages[0]
            self.assertIn(
                'v1-cover@example.com',
                cover.get('In-Reply-To', ''),
            )

    def test_cc_from_trailers(self):
        mirror = self._make_mirror()
        mirror.ensure_mirror()
        with mirror.worktree('HEAD'):
            user = ForgeUser(
                login='author', name='Test Author', email='author@example.com'
            )
            mbox = mirror.format_patches('base', user)
            messages = list(mbox)
            # check that reviewer from Acked-by is in Cc
            all_cc = ' '.join(m.get('Cc', '') for m in messages)
            self.assertIn('reviewer@example.com', all_cc)

    def test_to_header(self):
        mirror = self._make_mirror()
        mirror.ensure_mirror()
        with mirror.worktree('HEAD'):
            user = ForgeUser(
                login='author', name='Test Author', email='author@example.com'
            )
            mbox = mirror.format_patches('HEAD~1', user)
            messages = list(mbox)
            self.assertIn('list@example.com', messages[0].get('To', ''))
