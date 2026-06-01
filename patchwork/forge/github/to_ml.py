# Patchwork - automated patch tracking system
# Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
#
# SPDX-License-Identifier: GPL-2.0-or-later

from patchwork.forge.git import GitMirror
from patchwork.forge.util import ingest_emails
from patchwork.forge.util import next_version
from patchwork.forge.util import sanitize_pr_body
from patchwork.forge.util import send_emails


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
