# Patchwork - automated patch tracking system
# Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
#
# SPDX-License-Identifier: GPL-2.0-or-later

"""
Forge integration framework.

Provides an abstraction layer for synchronizing patch workflows between
patchwork and code forges (GitHub, GitLab, etc.). Each forge platform is
implemented as a backend that registers itself at import time and is only
loaded when listed in settings.FORGE_BACKENDS.

Backends are responsible for webhook signature verification and event parsing.
Sync logic (creating patches from PRs, forwarding comments) is implemented
per-backend but can reuse generic utilities from patchwork.forge.sync.

Forge-specific dependencies remain entirely optional: a backend module is never
imported unless explicitly enabled in settings.
"""

import importlib
import logging
from abc import ABC
from abc import abstractmethod
from dataclasses import dataclass
from dataclasses import field

from django.conf import settings
from django.db import transaction
from django.db.models.signals import post_save

from patchwork.models import Event
from patchwork.models import ForgeConfig

logger = logging.getLogger(__name__)


@dataclass
class ForgeUser:
    login: str = ''
    name: str = ''
    email: str = ''


@dataclass
class ReviewComment:
    path: str = ''
    diff_hunk: str = ''
    body: str = ''


@dataclass
class CheckRun:
    name: str = ''
    status: str = ''
    url: str = ''
    description: str = ''


@dataclass
class ForgeEvent:
    type: str = ''
    repo_key: str = ''
    pr_number: int = 0
    author: ForgeUser = field(default_factory=ForgeUser)
    body: str = ''

    # review fields
    review_id: int = 0
    review_state: str = ''

    # check fields
    check_suite_id: int = 0
    check_name: str = ''
    check_status: str = ''
    check_url: str = ''
    check_description: str = ''

    # pull request fields
    pr_title: str = ''
    pr_body: str = ''
    pr_head: str = ''
    pr_base: str = ''
    pr_head_branch: str = ''
    pr_action: str = ''
    pr_before: str = ''


class ForgeBackend(ABC):
    """
    Base class for forge backend implementations.

    Each backend handles webhook signature verification and event parsing for
    a specific forge platform (GitHub, GitLab, etc.). Backends register
    themselves at import time via register_backend().
    """

    @abstractmethod
    def verify_webhook_signature(self, body, headers, secret):
        """
        Verify the HMAC signature of an incoming webhook request.

        Args:
            body (bytes): Raw request body.
            headers (dict): HTTP headers.
            secret (str): Shared secret for signature verification.
                If empty, verification is skipped.

        Returns:
            True if the signature is valid or no secret is configured.
        """
        raise NotImplementedError

    @abstractmethod
    def parse_webhook_event(self, body, headers):
        """
        Parse a webhook payload into a ForgeEvent.

        Args:
            body (bytes): Raw request body.
            headers (dict): HTTP headers.

        Returns:
            A ForgeEvent instance, or None if the event should be ignored.
        """
        raise NotImplementedError

    @abstractmethod
    def process_webhook_event(self, forge_config, event):
        """
        Handle a parsed webhook event for a given project.

        Called by the webhook view after signature verification, event
        parsing and project routing. The backend decides which event
        types to act on and what sync actions to perform.
        """
        raise NotImplementedError

    @abstractmethod
    def series_metadata(self, forge_config, event):
        """
        Return a dict of SeriesMetadata key-value pairs to associate
        with a series created from a forge event.

        Used after ingesting patches to link the series back to its
        forge pull request and branch.
        """
        raise NotImplementedError

    def get_auth(self, forge_config):
        """
        Resolve authentication credentials for a forge config.

        Merges backend-level defaults from FORGE_AUTH[backend] with per-repo
        overrides from FORGE_AUTH[backend]["repos"][repo].
        """
        backend_auth = settings.FORGE_AUTH.get(forge_config.backend, {})
        auth = {k: v for k, v in backend_auth.items() if k != 'repos'}
        repo_overrides = backend_auth.get('repos', {}).get(
            forge_config.repo, {}
        )
        auth.update(repo_overrides)
        return auth

    def git_credentials(self, forge_config):
        """
        Return git credential store content as a string for the given
        project. Written to a temporary file and passed to git via
        GIT_CREDENTIAL_HELPER during clone and fetch operations.
        """
        raise NotImplementedError

    @abstractmethod
    def handle_series_completed(self, forge_config, series):
        """
        Handle a completed patch series from the mailing list.

        Called by the series-completed signal handler. The backend
        decides whether to create a pull request from the series.
        """
        raise NotImplementedError

    @abstractmethod
    def handle_comment_created(self, forge_config, comment, series):
        """
        Handle a comment on a patch or cover letter.

        Called by the comment-created signal handler. The backend
        decides whether to forward the comment to the forge PR.
        """
        raise NotImplementedError


_backends = {}


def register_backend(name, backend):
    _backends[name] = backend


def get_backend(name):
    return _backends.get(name)


def _on_series_completed(sender, instance, raw, **kwargs):
    if raw or instance.category != Event.CATEGORY_SERIES_COMPLETED:
        return

    series = instance.series
    if not series:
        return

    def do_sync():
        for forge_config in ForgeConfig.objects.filter(project=series.project):
            backend = get_backend(forge_config.backend)
            if not backend:
                continue
            try:
                backend.handle_series_completed(forge_config, series)
            except Exception:
                logger.exception(
                    'forge sync failed for series %d on %s',
                    series.id,
                    forge_config.backend,
                )

    transaction.on_commit(do_sync)


def _on_comment_created(sender, instance, raw, **kwargs):
    if raw:
        return

    if instance.category == Event.CATEGORY_PATCH_COMMENT_CREATED:
        comment = instance.patch_comment
        if not comment:
            return
        series = comment.patch.series
    elif instance.category == Event.CATEGORY_COVER_COMMENT_CREATED:
        comment = instance.cover_comment
        if not comment:
            return
        series = comment.cover.series
    else:
        return

    if not series:
        return

    def do_sync():
        for forge_config in ForgeConfig.objects.filter(project=series.project):
            backend = get_backend(forge_config.backend)
            if not backend:
                continue
            try:
                backend.handle_comment_created(forge_config, comment, series)
            except Exception:
                logger.exception(
                    'forge comment sync failed for series %d on %s',
                    series.id,
                    forge_config.backend,
                )

    transaction.on_commit(do_sync)


def load_backends():
    for module_path in settings.FORGE_BACKENDS:
        importlib.import_module(module_path)

    post_save.connect(_on_series_completed, sender=Event)
    post_save.connect(_on_comment_created, sender=Event)
