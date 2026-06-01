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
    pr_body = pr.get('body') or ''
    return ForgeEvent(
        type='pull_request',
        repo_key=get_repo_key(payload),
        pr_number=pr.get('number', 0),
        author=parse_user(pr.get('user')),
        pr_title=pr.get('title', ''),
        pr_body=pr_body,
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
    comment_body = comment.get('body') or ''
    return ForgeEvent(
        type='issue_comment',
        repo_key=get_repo_key(payload),
        pr_number=issue.get('number', 0),
        author=parse_user(comment.get('user')),
        body=comment_body,
    )


def parse_review(payload):
    if payload.get('action') != 'submitted':
        return None
    review = payload.get('review', {})
    pr = payload.get('pull_request', {})
    review_body = review.get('body') or ''
    return ForgeEvent(
        type='review',
        repo_key=get_repo_key(payload),
        pr_number=pr.get('number', 0),
        review_id=review.get('id', 0),
        author=parse_user(review.get('user')),
        body=review_body,
        review_state=review.get('state', ''),
    )


def parse_check_run(payload):
    if payload.get('action') != 'created':
        return None
    run = payload.get('check_run', {})
    prs = run.get('pull_requests', [])
    if not prs:
        return None
    return ForgeEvent(
        type='check_pending',
        repo_key=get_repo_key(payload),
        pr_number=prs[0].get('number', 0),
        check_name=run.get('name', ''),
        check_status='pending',
        check_url=run.get('html_url', ''),
    )


def parse_check_suite(payload):
    if payload.get('action') != 'completed':
        return None
    suite = payload.get('check_suite', {})
    prs = suite.get('pull_requests', [])
    if not prs:
        return None
    app = suite.get('app', {})
    return ForgeEvent(
        type='check_result',
        repo_key=get_repo_key(payload),
        pr_number=prs[0].get('number', 0),
        check_suite_id=suite.get('id', 0),
        check_name=app.get('name', ''),
        check_status=suite.get('conclusion', ''),
    )
