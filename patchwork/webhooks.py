# Patchwork - automated patch tracking system
# Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
#
# SPDX-License-Identifier: GPL-2.0-or-later

import hashlib
import hmac
import json
import logging
import urllib.request

from django.db import transaction

from patchwork.models import Webhook

logger = logging.getLogger(__name__)

REQUEST_TIMEOUT = 10


def _serialize_event(event):
    from patchwork.api.event import EventSerializer

    from django.conf import settings
    from django.contrib.sites.models import Site
    from rest_framework.request import Request
    from rest_framework.test import APIRequestFactory

    site = Site.objects.get_current()
    factory = APIRequestFactory(SERVER_NAME=site.domain)
    request = Request(factory.get('/', secure=settings.FORCE_HTTPS_LINKS))
    request.version = '1.5'
    serializer = EventSerializer(event, context={'request': request})
    return serializer.data


def deliver_webhooks(event):
    transaction.on_commit(lambda: _post_webhooks(event))


def _post_webhooks(event):
    try:
        webhooks = Webhook.objects.filter(project=event.project, active=True)
        if not webhooks:
            return

        payload = None

        for webhook in webhooks:
            if not webhook.matches_event(event.category):
                continue

            if payload is None:
                payload = json.dumps(_serialize_event(event)).encode()

            headers = {
                'Content-Type': 'application/json',
                'X-Patchwork-Event': event.category,
                'X-Patchwork-Delivery': str(event.id),
            }

            if webhook.secret:
                sig = hmac.new(
                    webhook.secret.encode(),
                    payload,
                    hashlib.sha256,
                ).hexdigest()
                headers['X-Patchwork-Signature'] = 'sha256=' + sig

            req = urllib.request.Request(
                webhook.url,
                data=payload,
                headers=headers,
                method='POST',
            )
            try:
                with urllib.request.urlopen(req, timeout=REQUEST_TIMEOUT):
                    pass
            except Exception:
                logger.warning(
                    'webhook delivery failed for %r',
                    webhook,
                    exc_info=True,
                )
    except Exception:
        logger.warning(
            'webhook delivery failed for event %r', event, exc_info=True
        )
