# Patchwork - automated patch tracking system
# Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
#
# SPDX-License-Identifier: GPL-2.0-or-later

import email
import logging

from django.contrib.auth import get_user_model
from django.db import transaction
from django.utils.text import slugify

from patchwork.forge import ReviewComment
from patchwork.forge.git import GitMirror
from patchwork.forge.github.api import fetch_review_comments
from patchwork.forge.util import bytes_to_mbox
from patchwork.forge.util import find_series_by_pr
from patchwork.forge.util import ingest_emails
from patchwork.forge.util import next_version
from patchwork.forge.util import reply_to_msgid
from patchwork.forge.util import sanitize_pr_body
from patchwork.forge.util import send_emails
from patchwork.forge.util import sender_identity

logger = logging.getLogger(__name__)


def handle_pull_request(gh, forge_config, event):
    if not forge_config.sync_forge_to_ml:
        return

    mirror = GitMirror(gh, forge_config)
    mirror.ensure_mirror()
    mirror.fetch()

    version = 1
    in_reply_to = ''
    range_diff_base = ''
    if event.pr_action == 'synchronize':
        version, reply_msgid, range_diff_base = next_version(
            gh, forge_config, event
        )
        if forge_config.thread_respins:
            in_reply_to = reply_msgid

    pr_url = gh.pr_ref(forge_config, event.pr_number)
    cover_body = sanitize_pr_body(event.pr_body)
    if cover_body:
        cover_body += f'\n\nPull request: {pr_url}'
    else:
        cover_body = f'Pull request: {pr_url}'

    with mirror.worktree(event.pr_head):
        mirror.add_commit_notes(
            event.pr_base,
            lambda sha: f'{pr_url}/commits/{sha}',
        )
        mbox = mirror.format_patches(
            event.pr_base,
            event.author,
            version=version,
            cover_title=event.pr_title,
            cover_body=cover_body,
            range_diff_base=range_diff_base,
            in_reply_to=in_reply_to,
        )

    ingest_emails(mbox, gh, forge_config, event)
    send_emails(mbox, forge_config)


def handle_issue_comment(gh, forge_config, event):
    series = find_series_by_pr(gh, forge_config, event.pr_number).last()
    if not series:
        logger.warning('no series found for PR #%d', event.pr_number)
        return

    subject = f'Re: {series.name} (comment)'
    reply(gh, forge_config, event, series, subject, event.body)


def handle_review(gh, forge_config, event):
    series = find_series_by_pr(gh, forge_config, event.pr_number).last()
    if not series:
        logger.warning('no series found for PR #%d', event.pr_number)
        return

    subject = f'Re: {series.name} (review: {event.review_state})'

    body = ''
    if event.review_state:
        body = f'Review: {event.review_state}\n\n'
    if event.body:
        body += f'{event.body}\n\n'

    comments = fetch_review_comments(
        gh, forge_config, event.pr_number, event.review_id
    )
    for c in comments:
        body += f'--- {c.path}\n'
        if c.diff_hunk:
            for line in c.diff_hunk.split('\n'):
                body += f'> {line}\n'
            body += '\n'
        body += f'{c.body}\n\n'

    if not event.body and not comments:
        return

    reply(gh, forge_config, event, series, subject, body.rstrip())


def reply(gh, forge_config, event, series, subject, body):
    """
    Build a reply email as an mbox, ingest it into the database and
    send it to the mailing list.
    """
    in_reply_to = reply_to_msgid(series)
    if not in_reply_to:
        logger.warning('no message-id for series %d', series.id)
        return

    _, addr = email.utils.parseaddr(forge_config.sender_addr)
    uid, domain = addr.rsplit('@', 1)

    msg = email.mime.text.MIMEText(body)
    msg['From'] = email.utils.formataddr(
        sender_identity(event.author, forge_config)
    )
    msg['Sender'] = email.utils.formataddr(
        email.utils.parseaddr(forge_config.sender_addr)
    )
    msg['To'] = forge_config.project.listemail
    msg['Subject'] = email.header.Header(subject, 'utf-8')
    msg['Date'] = email.utils.formatdate(localtime=True)
    msg['Message-ID'] = email.utils.make_msgid(uid, domain)
    msg['In-Reply-To'] = in_reply_to
    msg['References'] = in_reply_to
    msg['Reply-To'] = forge_config.project.listemail
    msg['List-ID'] = f'<{forge_config.project.listid}>'
    msg['X-Patchwork-Hint'] = 'ignore'

    mbox = bytes_to_mbox(msg.as_bytes(unixfrom=True))
    ingest_emails(mbox, gh, forge_config, event)
    send_emails(mbox, forge_config)
