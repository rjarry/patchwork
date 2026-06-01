# Patchwork - automated patch tracking system
# Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
#
# SPDX-License-Identifier: GPL-2.0-or-later

from patchwork.forge import ForgeEvent
from patchwork.forge import ForgeUser


def parse_pull_request(payload):
    action = payload.get('action', '')
    if action not in ('opened', 'synchronize'):
        return None
    pr = payload.get('pull_request', {})
    return ForgeEvent(
        type='pull_request',
        repo_key=get_repo_key(payload),
        pr_number=pr.get('number', 0),
        author=parse_user(pr.get('user')),
        pr_title=pr.get('title', ''),
        pr_body=pr.get('body', ''),
        pr_head=f'pull/{pr.get("number", 0)}/head',
        pr_base=pr.get('base', {}).get('sha', ''),
        pr_head_branch=pr.get('head', {}).get('ref', ''),
        pr_action=action,
        pr_before=payload.get('before', ''),
    )


def get_repo_key(payload):
    repo = payload.get('repository', {})
    return repo.get('full_name', '').lower()


def parse_user(user):
    if not user:
        return ForgeUser()
    return ForgeUser(
        login=user.get('login', ''),
        name=user.get('name', ''),
        email=user.get('email', ''),
    )
