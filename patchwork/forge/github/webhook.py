# Patchwork - automated patch tracking system
# Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
#
# SPDX-License-Identifier: GPL-2.0-or-later

from patchwork.forge import ForgeEvent
from patchwork.forge import ForgeUser
from patchwork.forge import ReviewComment


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


def parse_issue_comment(payload):
    if payload.get('action') != 'created':
        return None
    issue = payload.get('issue', {})
    if 'pull_request' not in issue:
        return None
    comment = payload.get('comment', {})
    return ForgeEvent(
        type='issue_comment',
        repo_key=get_repo_key(payload),
        pr_number=issue.get('number', 0),
        author=parse_user(comment.get('user')),
        body=comment.get('body', ''),
    )


def parse_review(self, payload):
    if payload.get('action') != 'submitted':
        return None
    review = payload.get('review', {})
    pr = payload.get('pull_request', {})
    comments = []
    for c in review.get('comments', []):
        comments.append(
            ReviewComment(
                path=c.get('path', ''),
                diff_hunk=c.get('diff_hunk', ''),
                body=c.get('body', ''),
            )
        )
    return ForgeEvent(
        type='review',
        repo_key=get_repo_key(payload),
        pr_number=pr.get('number', 0),
        author=parse_user(review.get('user')),
        body=review.get('body', ''),
        review_state=review.get('state', ''),
        review_comments=comments,
    )
