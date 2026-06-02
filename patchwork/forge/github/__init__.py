# Patchwork - automated patch tracking system
# Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
#
# SPDX-License-Identifier: GPL-2.0-or-later

"""
GitHub forge backend.
"""

import hashlib
import hmac
import json
import logging

from patchwork.forge import ForgeBackend
from patchwork.forge import register_backend
from patchwork.forge.github.from_ml import create_or_update_pr
from patchwork.forge.github.from_ml import post_pr_comment
from patchwork.forge.github.to_ml import handle_check_pending
from patchwork.forge.github.to_ml import handle_check_result
from patchwork.forge.github.to_ml import handle_issue_comment
from patchwork.forge.github.to_ml import handle_pull_request
from patchwork.forge.github.to_ml import handle_review
from patchwork.forge.github.webhook import parse_check_run
from patchwork.forge.github.webhook import parse_check_suite
from patchwork.forge.github.webhook import parse_issue_comment
from patchwork.forge.github.webhook import parse_pull_request
from patchwork.forge.github.webhook import parse_review

logger = logging.getLogger(__name__)


class GitHubBackend(ForgeBackend):
    def verify_webhook_signature(self, body, headers, secret):
        if not secret:
            return True
        signature = headers.get('X-Hub-Signature-256', '')
        prefix = 'sha256='
        if not signature.startswith(prefix):
            return False
        try:
            sig = bytes.fromhex(signature[len(prefix) :])
        except ValueError:
            return False
        mac = hmac.new(secret.encode(), body, hashlib.sha256)
        return hmac.compare_digest(sig, mac.digest())

    def parse_webhook_event(self, body, headers):
        event_type = headers.get('X-GitHub-Event', '')
        payload = json.loads(body)

        parsers = {
            'check_run': parse_check_run,
            'check_suite': parse_check_suite,
            'issue_comment': parse_issue_comment,
            'pull_request': parse_pull_request,
            'pull_request_review': parse_review,
        }

        parser = parsers.get(event_type)
        if parser is None:
            return None
        return parser(payload)

    def repo_url(self, forge_config):
        return f'https://github.com/{forge_config.repo}.git'

    def pr_ref(self, forge_config, pr_number):
        return f'https://github.com/{forge_config.repo}/pull/{pr_number}'

    def pr_refspec(self, pr_number):
        return f'pull/{pr_number}/head'

    def meta_key_pr(self):
        return 'github_pr'

    def series_metadata(self, forge_config, event):
        return {
            'github_pr': self.pr_ref(forge_config, event.pr_number),
            'github_branch': event.pr_head_branch,
        }

    def git_credentials(self, forge_config):
        auth = self.get_auth(forge_config)
        token = auth.get('token', '')
        return f'https://x-access-token:{token}@github.com\n'

    def process_webhook_event(self, forge_config, event):
        handlers = {
            'issue_comment': handle_issue_comment,
            'pull_request': handle_pull_request,
            'review': handle_review,
            'check_pending': handle_check_pending,
            'check_result': handle_check_result,
        }
        handler = handlers.get(event.type)
        if handler:
            handler(forge_config, event)

    def handle_series_completed(self, forge_config, series):
        create_or_update_pr(self, forge_config, series)

    def handle_comment_created(self, forge_config, comment, series):
        post_pr_comment(self, forge_config, comment, series)


register_backend('github', GitHubBackend())
