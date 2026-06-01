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
from patchwork.forge.github.to_ml import handle_pull_request
from patchwork.forge.github.webhook import parse_pull_request

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
            'pull_request': parse_pull_request,
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
            'pull_request': handle_pull_request,
        }
        handler = handlers.get(event.type)
        if handler:
            handler(self, forge_config, event)


register_backend('github', GitHubBackend())
