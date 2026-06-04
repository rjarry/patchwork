# Patchwork - automated patch tracking system
# Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
#
# SPDX-License-Identifier: GPL-2.0-or-later


import logging

from patchwork.forge.git import GitMirror
from patchwork.forge.github.api import base_branch
from patchwork.forge.github.api import create_pr
from patchwork.forge.github.api import post_comment
from patchwork.forge.util import build_pr_body
from patchwork.forge.util import forge_branch_name
from patchwork.forge.util import series_from_forge
from patchwork.models import SeriesMetadata
from patchwork.views.utils import series_to_mbox

logger = logging.getLogger(__name__)


def create_or_update_pr(gh, forge_config, series):
    if not forge_config.sync_ml_to_forge:
        return

    if SeriesMetadata.objects.filter(
        series=series, key=gh.meta_key_pr()
    ).exists():
        return

    if series_from_forge(series):
        return

    mirror = GitMirror(gh, forge_config)
    mirror.ensure_mirror()
    mirror.fetch()

    base = base_branch(gh, forge_config)
    pr_number, branch = find_previous_pr(gh, forge_config, series)
    if not branch:
        branch = forge_branch_name(series)

    mbox = series_to_mbox(series)
    with mirror.worktree(base):
        mirror.apply_mbox(mbox)
        mirror.push(branch)

    body = build_pr_body(series)

    if pr_number:
        comment = f'> Series v{series.version} submitted.\n\n{body}'
        post_comment(gh, forge_config, pr_number, comment)
        action = 'updated'
    else:
        title = series.name or 'Untitled series'
        pr_number = create_pr(gh, forge_config, title, body, branch, base)
        action = 'created'

    pr_ref = gh.pr_ref(forge_config, pr_number)
    store_series_metadata(gh, forge_config, series, pr_ref, branch)
    logger.info(
        '%s PR #%d for series %d (v%d): %s',
        action,
        pr_number,
        series.id,
        series.version,
        pr_ref,
    )


def find_previous_pr(gh, forge_config, series):
    """
    Walk the previous_series chain to find a prior version that
    has an existing PR. Return (pr_number, branch) or (None, None).
    """
    current = series
    while current.previous_series_id:
        current = current.previous_series
        try:
            pr_meta = SeriesMetadata.objects.get(
                series=current, key=gh.meta_key_pr()
            )
        except SeriesMetadata.DoesNotExist:
            continue
        try:
            branch_meta = SeriesMetadata.objects.get(
                series=current,
                key=f'{forge_config.backend}_branch',
            )
        except SeriesMetadata.DoesNotExist:
            continue
        pr_url = pr_meta.value
        pr_number = int(pr_url.rsplit('/', 1)[-1])
        return pr_number, branch_meta.value
    return None, None


def store_series_metadata(gh, forge_config, series, pr_ref, branch):
    SeriesMetadata.objects.update_or_create(
        series=series,
        key=gh.meta_key_pr(),
        defaults={'value': pr_ref},
    )
    SeriesMetadata.objects.update_or_create(
        series=series,
        key=f'{forge_config.backend}_branch',
        defaults={'value': branch},
    )
