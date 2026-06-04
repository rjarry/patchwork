# Patchwork - automated patch tracking system
# Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
#
# SPDX-License-Identifier: GPL-2.0-or-later

import logging

from django.conf import settings
from django.http import HttpResponse
from django.http import HttpResponseNotAllowed
from django.http import HttpResponseNotFound
from django.http import JsonResponse
from django.views.decorators.csrf import csrf_exempt

from patchwork.forge import get_backend
from patchwork.models import ForgeConfig

logger = logging.getLogger(__name__)


@csrf_exempt
def forge_webhook(request, backend_name):
    """
    Receive and process a webhook from a forge backend.

    Verifies the request signature, parses the event payload and
    dispatches it to the appropriate backend sync handler.
    """
    if request.method != 'POST':
        return HttpResponseNotAllowed(['POST'])

    backend = get_backend(backend_name)
    if backend is None:
        return HttpResponseNotFound()

    secret = settings.FORGE_WEBHOOK_SECRETS.get(backend_name, '')
    if not backend.verify_webhook_signature(
        request.body, request.headers, secret
    ):
        return HttpResponse(status=403)

    try:
        event = backend.parse_webhook_event(request.body, request.headers)
    except Exception:
        logger.exception('failed to parse %s webhook', backend_name)
        return HttpResponse(status=400)

    if event is None:
        return JsonResponse({'status': 'ignored'})

    try:
        forge_config = ForgeConfig.objects.select_related('project').get(
            backend=backend_name, repo=event.repo_key
        )
    except ForgeConfig.DoesNotExist:
        logger.warning(
            '%s webhook: no project for repo %s',
            backend_name,
            event.repo_key,
        )
        return JsonResponse({'status': 'unlinked'})

    logger.info(
        '%s webhook: type=%s repo=%s pr=%s project=%s',
        backend_name,
        event.type,
        event.repo_key,
        event.pr_number,
        forge_config.project.linkname,
    )

    try:
        backend.process_webhook_event(forge_config, event)
    except Exception:
        logger.exception(
            'failed to handle %s event for %s',
            event.type,
            forge_config.project.linkname,
        )
        return HttpResponse(status=500)

    return JsonResponse({'status': 'ok'})
