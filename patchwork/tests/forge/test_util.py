# Patchwork - automated patch tracking system
# Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
#
# SPDX-License-Identifier: GPL-2.0-or-later

from unittest.mock import MagicMock
from unittest.mock import patch as mock_patch

from django.test import TestCase

from patchwork.forge import ForgeEvent
from patchwork.forge import ForgeUser
from patchwork.forge.util import bytes_to_mbox
from patchwork.forge.util import ingest_emails
from patchwork.forge.util import next_version
from patchwork.forge.util import reply_to_msgid
from patchwork.forge.util import sanitize_pr_body
from patchwork.forge.util import send_emails
from patchwork.forge.util import sender_identity
from patchwork.models import ForgeConfig
from patchwork.models import SeriesMetadata
from patchwork.tests.utils import create_cover
from patchwork.tests.utils import create_patches
from patchwork.tests.utils import create_project
from patchwork.tests.utils import create_series


class SanitizePRBodyTest(TestCase):
    def test_strip_html_comments(self):
        body = 'Hello\n<!-- this is hidden -->\nWorld'
        self.assertEqual(sanitize_pr_body(body), 'Hello\n\nWorld')

    def test_strip_multiline_html_comment(self):
        body = 'Before\n<!--\nline1\nline2\n-->\nAfter'
        self.assertEqual(sanitize_pr_body(body), 'Before\n\nAfter')

    def test_strip_coderabbit_section(self):
        body = (
            'Real content\n\n'
            '## Summary by CodeRabbit\n\n'
            'AI generated stuff\n'
            'more AI stuff\n'
        )
        self.assertEqual(sanitize_pr_body(body), 'Real content')

    def test_strip_copilot_section(self):
        body = 'Fix bug\n\n## Summary by Copilot\n\nAI stuff'
        self.assertEqual(sanitize_pr_body(body), 'Fix bug')

    def test_strip_walkthrough_section(self):
        body = 'Real\n\n## Walkthrough\n\nAI stuff'
        self.assertEqual(sanitize_pr_body(body), 'Real')

    def test_preserve_normal_headings(self):
        body = '## Description\n\nThis is fine\n\n## Notes\n\nAlso fine'
        self.assertEqual(sanitize_pr_body(body), body)

    def test_empty_body(self):
        self.assertEqual(sanitize_pr_body(''), '')

    def test_none_body(self):
        self.assertEqual(sanitize_pr_body(None), '')


class SenderIdentityTest(TestCase):
    def test_user_with_name_and_email(self):
        user = ForgeUser(login='octocat', name='Octo Cat', email='o@c.com')
        config = ForgeConfig(from_email='pw@example.com')
        self.assertEqual(
            sender_identity(user, config), ('Octo Cat', 'o@c.com')
        )

    def test_user_with_email_only(self):
        user = ForgeUser(login='octocat', name='', email='o@c.com')
        config = ForgeConfig(from_email='pw@example.com')
        self.assertEqual(
            sender_identity(user, config),
            ('octocat', 'o@c.com'),
        )

    def test_user_without_email(self):
        user = ForgeUser(login='octocat', name='Octo Cat', email='')
        config = ForgeConfig(from_email='pw@example.com')
        self.assertEqual(
            sender_identity(user, config),
            ('Octo Cat (via Patchwork)', 'pw@example.com'),
        )

    def test_user_without_email_fallback(self):
        user = ForgeUser(login='octocat', name='', email='')
        config = ForgeConfig(from_email='')
        name, addr = sender_identity(user, config)
        self.assertEqual(name, 'octocat (via Patchwork)')
        self.assertTrue(addr)


class ReplyToMsgidTest(TestCase):
    def test_series_with_cover_letter(self):
        project = create_project()
        series = create_series(project=project)
        cover = create_cover(series=series)
        self.assertEqual(reply_to_msgid(series), cover.msgid)

    def test_series_without_cover_letter(self):
        project = create_project()
        series = create_series(project=project)
        patches = create_patches(count=1, series=series)
        self.assertEqual(reply_to_msgid(series), patches[0].msgid)

    def test_empty_series(self):
        series = create_series()
        self.assertEqual(reply_to_msgid(series), '')


class NextVersionTest(TestCase):
    def test_no_previous_series(self):
        project = create_project()
        backend = MagicMock()
        backend.pr_ref.return_value = 'https://github.com/o/r/pull/1'
        backend.meta_key_pr.return_value = 'github_pr'
        forge_config = MagicMock()
        forge_config.project = project
        event = ForgeEvent(pr_number=1, pr_before='abc123')

        version, in_reply_to, previous_ref = next_version(
            backend, forge_config, event
        )
        self.assertEqual(version, 1)
        self.assertEqual(in_reply_to, '')
        self.assertEqual(previous_ref, '')

    def test_with_previous_series(self):
        project = create_project()
        series_v1 = create_series(project=project, version=1)
        cover_v1 = create_cover(series=series_v1)
        pr_ref = 'https://github.com/o/r/pull/42'
        SeriesMetadata.objects.create(
            series=series_v1, key='github_pr', value=pr_ref
        )

        backend = MagicMock()
        backend.pr_ref.return_value = pr_ref
        backend.meta_key_pr.return_value = 'github_pr'
        forge_config = MagicMock()
        forge_config.project = project
        event = ForgeEvent(pr_number=42, pr_before='def456')

        version, in_reply_to, previous_ref = next_version(
            backend, forge_config, event
        )
        self.assertEqual(version, 2)
        self.assertEqual(in_reply_to, cover_v1.msgid)
        self.assertEqual(previous_ref, 'def456')

    def test_with_multiple_versions(self):
        project = create_project()
        series_v1 = create_series(project=project, version=1)
        cover_v1 = create_cover(series=series_v1)
        series_v2 = create_series(project=project, version=2)
        create_cover(series=series_v2)
        pr_ref = 'https://github.com/o/r/pull/42'
        SeriesMetadata.objects.create(
            series=series_v1, key='github_pr', value=pr_ref
        )
        SeriesMetadata.objects.create(
            series=series_v2, key='github_pr', value=pr_ref
        )

        backend = MagicMock()
        backend.pr_ref.return_value = pr_ref
        backend.meta_key_pr.return_value = 'github_pr'
        forge_config = MagicMock()
        forge_config.project = project
        event = ForgeEvent(pr_number=42, pr_before='ghi789')

        version, in_reply_to, previous_ref = next_version(
            backend, forge_config, event
        )
        self.assertEqual(version, 3)
        self.assertEqual(in_reply_to, cover_v1.msgid)
        self.assertEqual(previous_ref, 'ghi789')


class BytesToMboxTest(TestCase):
    MBOX_DATA = (
        b'From nobody Thu Jan  1 00:00:00 1970\n'
        b'From: Test <test@example.com>\n'
        b'Subject: [PATCH 1/2] first patch\n'
        b'Message-ID: <patch1@example.com>\n'
        b'\n'
        b'First patch body.\n'
        b'\n'
        b'From nobody Thu Jan  1 00:00:00 1970\n'
        b'From: Test <test@example.com>\n'
        b'Subject: [PATCH 2/2] second patch\n'
        b'Message-ID: <patch2@example.com>\n'
        b'\n'
        b'Second patch body.\n'
    )

    def test_parse_messages(self):
        mbox = bytes_to_mbox(self.MBOX_DATA)
        messages = list(mbox)
        self.assertEqual(len(messages), 2)
        self.assertIn('patch1@example.com', messages[0].get('Message-ID'))
        self.assertIn('patch2@example.com', messages[1].get('Message-ID'))

    def test_get_bytes_preserves_content(self):
        mbox = bytes_to_mbox(self.MBOX_DATA)
        for key, msg in mbox.iteritems():
            raw = mbox.get_bytes(key)
            self.assertIn(b'From: Test <test@example.com>', raw)
            self.assertIn(msg.get('Subject').encode(), raw)

    def test_empty_input(self):
        mbox = bytes_to_mbox(b'')
        self.assertEqual(len(list(mbox)), 0)


class IngestEmailsTest(TestCase):
    def _make_mbox(self, messages):
        buf = b''
        for msg in messages:
            buf += b'From nobody Thu Jan  1 00:00:00 1970\n'
            buf += msg + b'\n'
        return bytes_to_mbox(buf)

    def test_ingest_creates_metadata(self):
        project = create_project()
        series = create_series(project=project)
        patch = create_patches(count=1, series=series)[0]

        mbox = self._make_mbox(
            [
                b'From: Test <test@example.com>\n'
                b'Subject: [PATCH] fix thing\n'
                b'Message-ID: <ingest1@example.com>\n'
                b'\nBody\n',
            ]
        )

        backend = MagicMock()
        backend.series_metadata.return_value = {
            'github_pr': 'https://github.com/o/r/pull/1',
            'github_branch': 'fix-thing',
        }
        forge_config = MagicMock()
        forge_config.project = project

        with mock_patch('patchwork.forge.util.parse_mail', return_value=patch):
            ingest_emails(mbox, backend, forge_config, ForgeEvent())

        backend.series_metadata.assert_called_once()
        self.assertTrue(
            SeriesMetadata.objects.filter(
                series=series, key='github_pr'
            ).exists()
        )
        self.assertTrue(
            SeriesMetadata.objects.filter(
                series=series, key='github_branch'
            ).exists()
        )

    def test_ingest_skips_duplicates(self):
        from patchwork.parser import DuplicateMailError

        mbox = self._make_mbox(
            [
                b'From: Test <test@example.com>\n'
                b'Subject: [PATCH] fix thing\n'
                b'Message-ID: <dup1@example.com>\n'
                b'\nBody\n',
            ]
        )

        backend = MagicMock()
        forge_config = MagicMock()
        forge_config.project = create_project()

        with mock_patch(
            'patchwork.forge.util.parse_mail',
            side_effect=DuplicateMailError(msgid='<dup1@example.com>'),
        ):
            ingest_emails(mbox, backend, forge_config, ForgeEvent())

        backend.series_metadata.assert_not_called()

    def test_ingest_skips_value_errors(self):
        mbox = self._make_mbox(
            [
                b'From: Test <test@example.com>\n'
                b'Subject: [PATCH] fix thing\n'
                b'Message-ID: <bad1@example.com>\n'
                b'\nBody\n',
            ]
        )

        backend = MagicMock()
        forge_config = MagicMock()
        forge_config.project = create_project()

        with mock_patch(
            'patchwork.forge.util.parse_mail',
            side_effect=ValueError('bad email'),
        ):
            ingest_emails(mbox, backend, forge_config, ForgeEvent())

        backend.series_metadata.assert_not_called()

    def test_ingest_no_metadata_on_empty_values(self):
        project = create_project()
        series = create_series(project=project)
        patch = create_patches(count=1, series=series)[0]

        mbox = self._make_mbox(
            [
                b'From: Test <test@example.com>\n'
                b'Subject: [PATCH] fix thing\n'
                b'Message-ID: <meta1@example.com>\n'
                b'\nBody\n',
            ]
        )

        backend = MagicMock()
        backend.series_metadata.return_value = {
            'github_pr': 'https://github.com/o/r/pull/1',
            'github_branch': '',
        }
        forge_config = MagicMock()
        forge_config.project = project

        with mock_patch('patchwork.forge.util.parse_mail', return_value=patch):
            ingest_emails(mbox, backend, forge_config, ForgeEvent())

        self.assertTrue(
            SeriesMetadata.objects.filter(
                series=series, key='github_pr'
            ).exists()
        )
        self.assertFalse(
            SeriesMetadata.objects.filter(
                series=series, key='github_branch'
            ).exists()
        )


class SendEmailsTest(TestCase):
    MBOX_DATA = (
        b'From nobody Thu Jan  1 00:00:00 1970\n'
        b'From: Author <author@example.com>\n'
        b'Sender: Bot <bot@example.com>\n'
        b'To: list@example.com\n'
        b'Cc: reviewer@example.com\n'
        b'Subject: [PATCH 1/1] fix thing\n'
        b'Message-ID: <send1@example.com>\n'
        b'\n'
        b'Patch body.\n'
    )

    def test_sends_via_smtp(self):
        mbox = bytes_to_mbox(self.MBOX_DATA)
        forge_config = MagicMock()

        mock_conn = MagicMock()
        mock_conn.connection.sendmail.return_value = {}

        with mock_patch(
            'patchwork.forge.util.get_connection'
        ) as mock_get_conn:
            mock_get_conn.return_value.__enter__ = MagicMock(
                return_value=mock_conn
            )
            mock_get_conn.return_value.__exit__ = MagicMock(return_value=False)
            send_emails(mbox, forge_config)

        mock_conn.connection.sendmail.assert_called_once()
        call_args = mock_conn.connection.sendmail.call_args
        sender = call_args[0][0]
        recipients = call_args[0][1]
        raw_bytes = call_args[0][2]
        self.assertEqual(sender, 'bot@example.com')
        self.assertIn('author@example.com', recipients)
        self.assertIn('list@example.com', recipients)
        self.assertIn('reviewer@example.com', recipients)
        self.assertIn(b'[PATCH 1/1] fix thing', raw_bytes)

    def test_sends_raw_bytes(self):
        mbox = bytes_to_mbox(self.MBOX_DATA)
        forge_config = MagicMock()

        mock_conn = MagicMock()
        mock_conn.connection.sendmail.return_value = {}

        with mock_patch(
            'patchwork.forge.util.get_connection'
        ) as mock_get_conn:
            mock_get_conn.return_value.__enter__ = MagicMock(
                return_value=mock_conn
            )
            mock_get_conn.return_value.__exit__ = MagicMock(return_value=False)
            send_emails(mbox, forge_config)

        raw_bytes = mock_conn.connection.sendmail.call_args[0][2]
        self.assertIn(b'Sender: Bot <bot@example.com>', raw_bytes)
        self.assertIn(b'Message-ID: <send1@example.com>', raw_bytes)
