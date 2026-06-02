# Patchwork - automated patch tracking system
# Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
#
# SPDX-License-Identifier: GPL-2.0-or-later

from django.conf import settings

from patchwork.forge import CheckRun
from patchwork.forge import ForgeEvent
from patchwork.forge import ForgeUser
from patchwork.forge import ReviewComment
from patchwork.forge.util import COMMENT_MARKER


def parse_pull_request(payload):
    action = payload.get('action', '')
    if action not in ('opened', 'synchronize'):
        return None
    pr = payload.get('pull_request', {})
    if COMMENT_MARKER in pr.get('body', ''):
        return None
    branch = pr.get('head', {}).get('ref', '')
    if branch.startswith(f'{settings.FORGE_BRANCH_PREFIX}/'):
        return None
    return ForgeEvent(
        type='pull_request',
        repo_key=get_repo_key(payload),
        pr_number=pr.get('number', 0),
        author=parse_user(pr.get('user')),
        pr_title=pr.get('title', ''),
        pr_body=pr.get('body', ''),
        pr_head=f'pull/{pr.get("number", 0)}/head',
        pr_base=pr.get('base', {}).get('sha', ''),
        pr_head_branch=branch,
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
    if COMMENT_MARKER in comment.get('body', ''):
        return None
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
        if COMMENT_MARKER in c.get('body', ''):
            return None
        comments.append(
            ReviewComment(
                path=c.get('path', ''),
                diff_hunk=c.get('diff_hunk', ''),
                body=c.get('body', ''),
            )
        )
    if COMMENT_MARKER in review.get('body', ''):
        return None
    return ForgeEvent(
        type='review',
        repo_key=get_repo_key(payload),
        pr_number=pr.get('number', 0),
        author=parse_user(review.get('user')),
        body=review.get('body', ''),
        review_state=review.get('state', ''),
        review_comments=comments,
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
    check_runs = []
    for run in suite.get('check_runs', []):
        status = run.get('conclusion') or run.get('status', '')
        desc = ''
        output = run.get('output')
        if output:
            desc = output.get('summary', '')
        check_runs.append(
            CheckRun(
                name=run.get('name', ''),
                status=status,
                url=run.get('html_url', ''),
                description=desc,
            )
        )
    app = suite.get('app', {})
    return ForgeEvent(
        type='check_result',
        repo_key=get_repo_key(payload),
        pr_number=prs[0].get('number', 0),
        check_name=app.get('name', ''),
        check_status=suite.get('conclusion', ''),
        check_runs=check_runs,
    )
