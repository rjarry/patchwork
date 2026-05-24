# Patchwork - automated patch tracking system
# Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
#
# SPDX-License-Identifier: GPL-2.0-or-later

import hashlib
import hmac
import json
from http.server import BaseHTTPRequestHandler
from http.server import HTTPServer
import threading

from django.test import override_settings
from django.urls import reverse
from rest_framework import status

from patchwork.tests.api import utils
from patchwork.tests.utils import create_maintainer
from patchwork.tests.utils import create_patch
from patchwork.tests.utils import create_project
from patchwork.tests.utils import create_series
from patchwork.tests.utils import create_user
from patchwork.tests.utils import create_webhook

NO_VALIDATE = {
    'validate_request': False,
    'validate_response': False,
}


@override_settings(ENABLE_REST_API=True)
class TestWebhookAPI(utils.APITestCase):
    @staticmethod
    def api_url(project_id, item=None, version='1.5'):
        kwargs = {'project_id': project_id}
        if version:
            kwargs['version'] = version
        if item is None:
            return reverse('api-webhook-list', kwargs=kwargs)
        kwargs['pk'] = item
        return reverse('api-webhook-detail', kwargs=kwargs)

    def test_list_anonymous(self):
        project = create_project()
        resp = self.client.get(self.api_url(project.id), **NO_VALIDATE)
        self.assertEqual(status.HTTP_403_FORBIDDEN, resp.status_code)

    def test_list_non_maintainer(self):
        project = create_project()
        user = create_user()
        self.client.authenticate(user)
        resp = self.client.get(self.api_url(project.id), **NO_VALIDATE)
        self.assertEqual(status.HTTP_403_FORBIDDEN, resp.status_code)

    def test_list_empty(self):
        project = create_project()
        user = create_maintainer(project=project)
        self.client.authenticate(user)
        resp = self.client.get(self.api_url(project.id), **NO_VALIDATE)
        self.assertEqual(status.HTTP_200_OK, resp.status_code)
        self.assertEqual(0, len(resp.data))

    def test_create(self):
        project = create_project()
        user = create_maintainer(project=project)
        self.client.authenticate(user)
        resp = self.client.post(
            self.api_url(project.id),
            {
                'url': 'http://example.com/hook',
                'secret': 's3cret',
                'events': '*',
            },
            **NO_VALIDATE,
        )
        self.assertEqual(status.HTTP_201_CREATED, resp.status_code)
        self.assertEqual('http://example.com/hook', resp.data['url'])
        self.assertNotIn('secret', resp.data)
        self.assertTrue(resp.data['active'])

    def test_create_specific_events(self):
        project = create_project()
        user = create_maintainer(project=project)
        self.client.authenticate(user)
        resp = self.client.post(
            self.api_url(project.id),
            {
                'url': 'http://example.com/hook',
                'events': 'patch-created,series-completed',
            },
            **NO_VALIDATE,
        )
        self.assertEqual(status.HTTP_201_CREATED, resp.status_code)
        self.assertEqual('patch-created,series-completed', resp.data['events'])

    def test_create_invalid_events(self):
        project = create_project()
        user = create_maintainer(project=project)
        self.client.authenticate(user)
        resp = self.client.post(
            self.api_url(project.id),
            {
                'url': 'http://example.com/hook',
                'events': 'invalid-event',
            },
            **NO_VALIDATE,
        )
        self.assertEqual(status.HTTP_400_BAD_REQUEST, resp.status_code)

    def test_detail(self):
        project = create_project()
        user = create_maintainer(project=project)
        webhook = create_webhook(project=project, creator=user)
        self.client.authenticate(user)
        resp = self.client.get(
            self.api_url(project.id, webhook.id), **NO_VALIDATE
        )
        self.assertEqual(status.HTTP_200_OK, resp.status_code)
        self.assertEqual(webhook.url, resp.data['url'])
        self.assertNotIn('secret', resp.data)

    def test_update(self):
        project = create_project()
        user = create_maintainer(project=project)
        webhook = create_webhook(project=project, creator=user)
        self.client.authenticate(user)
        resp = self.client.patch(
            self.api_url(project.id, webhook.id),
            {'active': False},
            **NO_VALIDATE,
        )
        self.assertEqual(status.HTTP_200_OK, resp.status_code)
        self.assertFalse(resp.data['active'])

    def test_delete(self):
        project = create_project()
        user = create_maintainer(project=project)
        webhook = create_webhook(project=project, creator=user)
        self.client.authenticate(user)
        resp = self.client.delete(
            self.api_url(project.id, webhook.id), **NO_VALIDATE
        )
        self.assertEqual(status.HTTP_204_NO_CONTENT, resp.status_code)

    def test_secret_write_only(self):
        project = create_project()
        user = create_maintainer(project=project)
        webhook = create_webhook(
            project=project, creator=user, secret='mysecret'
        )
        self.client.authenticate(user)
        resp = self.client.get(
            self.api_url(project.id, webhook.id), **NO_VALIDATE
        )
        self.assertEqual(status.HTTP_200_OK, resp.status_code)
        self.assertNotIn('secret', resp.data)


@override_settings(ENABLE_REST_API=True)
class TestWebhookDelivery(utils.APITestCase):
    def test_delivery_on_patch_created(self):
        received = []

        class Handler(BaseHTTPRequestHandler):
            def do_POST(self):
                length = int(self.headers['Content-Length'])
                body = self.rfile.read(length)
                received.append(
                    {
                        'body': body,
                        'headers': dict(self.headers),
                    }
                )
                self.send_response(200)
                self.end_headers()

            def log_message(self, format, *args):
                pass

        server = HTTPServer(('127.0.0.1', 0), Handler)
        port = server.server_address[1]
        thread = threading.Thread(target=server.serve_forever)
        thread.daemon = True
        thread.start()

        try:
            project = create_project()
            secret = 'test-webhook-secret'
            create_webhook(
                project=project,
                url='http://127.0.0.1:%d/' % port,
                secret=secret,
                events='*',
            )
            series = create_series(project=project)
            create_patch(project=project, series=series)
        finally:
            server.shutdown()
            thread.join(timeout=5)

        self.assertTrue(len(received) > 0)

        req = received[0]
        expected_sig = hmac.new(
            secret.encode(), req['body'], hashlib.sha256
        ).hexdigest()
        self.assertEqual(
            req['headers']['X-Patchwork-Signature'],
            'sha256=' + expected_sig,
        )

        payload = json.loads(req['body'])
        self.assertIn('category', payload)
        self.assertIn('payload', payload)

    def test_delivery_filtered_by_events(self):
        received = []

        class Handler(BaseHTTPRequestHandler):
            def do_POST(self):
                length = int(self.headers['Content-Length'])
                body = self.rfile.read(length)
                received.append(json.loads(body))
                self.send_response(200)
                self.end_headers()

            def log_message(self, format, *args):
                pass

        server = HTTPServer(('127.0.0.1', 0), Handler)
        port = server.server_address[1]
        thread = threading.Thread(target=server.serve_forever)
        thread.daemon = True
        thread.start()

        try:
            project = create_project()
            create_webhook(
                project=project,
                url='http://127.0.0.1:%d/' % port,
                events='series-created',
            )
            series = create_series(project=project)
            create_patch(project=project, series=series)
        finally:
            server.shutdown()
            thread.join(timeout=5)

        categories = [r['category'] for r in received]
        self.assertIn('series-created', categories)
        self.assertNotIn('patch-created', categories)
