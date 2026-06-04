# Patchwork - automated patch tracking system
# Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
#
# SPDX-License-Identifier: GPL-2.0-or-later


import json
import logging
import urllib.error
import urllib.request

from patchwork.forge import CheckRun
from patchwork.forge import ReviewComment

logger = logging.getLogger(__name__)


class GitHubAPIError(Exception):
    def __init__(self, method, path, code, body):
        super().__init__(f'API {method} {path} failed ({code}): {body}')


def gh_api_request(gh, forge_config, method, path, data=None):
    """
    Make a GitHub API request. Return the parsed JSON response.
    """
    auth = gh.get_auth(forge_config)
    token = auth.get('token', '')
    url = f'https://api.github.com{path}'
    body = json.dumps(data).encode() if data else None
    req = urllib.request.Request(url, data=body, method=method)
    req.add_header('Authorization', f'token {token}')
    req.add_header('Accept', 'application/vnd.github+json')
    if body:
        req.add_header('Content-Type', 'application/json')
    try:
        with urllib.request.urlopen(req) as resp:
            return json.loads(resp.read())
    except urllib.error.HTTPError as e:
        error_body = e.read().decode('utf-8', errors='replace')
        raise GitHubAPIError(method, path, e.code, error_body) from e


def fetch_review_comments(gh, forge_config, pr_number, review_id):
    """
    Fetch inline comments for a review via the GitHub API.
    """
    owner, repo = forge_config.repo.split('/', 1)
    results = gh_api_request(
        gh,
        forge_config,
        'GET',
        f'/repos/{owner}/{repo}/pulls/{pr_number}/reviews/{review_id}/comments',
    )
    comments = []
    for r in results:
        body = r.get('body') or ''
        if body == '':
            continue
        comments.append(
            ReviewComment(
                body=body,
                path=r.get('path', '/dev/null'),
                diff_hunk=r.get('diff_hunk', ''),
            )
        )
    return comments


def fetch_check_runs(gh, forge_config, check_suite_id):
    """
    Fetch check runs for a check suite via the GitHub API.
    """
    owner, repo = forge_config.repo.split('/', 1)
    result = gh_api_request(
        gh,
        forge_config,
        'GET',
        f'/repos/{owner}/{repo}/check-suites/{check_suite_id}/check-runs',
    )
    runs = []
    for run in result.get('check_runs', []):
        status = run.get('conclusion') or run.get('status', '')
        desc = ''
        output = run.get('output')
        if output:
            desc = output.get('summary') or ''
        runs.append(
            CheckRun(
                name=run.get('name', ''),
                status=status,
                url=run.get('html_url', ''),
                description=desc,
            )
        )
    return runs
