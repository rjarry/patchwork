# Patchwork - automated patch tracking system
# Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
#
# SPDX-License-Identifier: GPL-2.0-or-later

from django.urls import path

from patchwork.forge import views

urlpatterns = [
    path(
        '<str:backend_name>/',
        views.forge_webhook,
        name='forge-webhook',
    ),
]
