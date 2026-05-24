# Patchwork - automated patch tracking system
# Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
#
# SPDX-License-Identifier: GPL-2.0-or-later

from rest_framework.generics import ListCreateAPIView
from rest_framework.generics import RetrieveUpdateDestroyAPIView
from rest_framework import permissions
from rest_framework.serializers import ModelSerializer

from patchwork.models import Event
from patchwork.models import Project
from patchwork.models import Webhook


class IsProjectMaintainer(permissions.BasePermission):
    def has_permission(self, request, view):
        if not request.user or not request.user.is_authenticated:
            return False
        project_id = view.kwargs.get('project_id')
        return request.user.profile.maintainer_projects.filter(
            pk=project_id
        ).exists()


class WebhookSerializer(ModelSerializer):
    class Meta:
        model = Webhook
        fields = (
            'id',
            'url',
            'secret',
            'events',
            'active',
            'creator',
            'created',
        )
        read_only_fields = ('id', 'creator', 'created')
        extra_kwargs = {
            'secret': {'write_only': True},
        }

    def validate_events(self, value):
        if value == '*':
            return value
        valid = {c[0] for c in Event.CATEGORY_CHOICES}
        for cat in value.split(','):
            cat = cat.strip()
            if cat not in valid:
                from rest_framework.exceptions import ValidationError

                raise ValidationError(
                    "Invalid event category '%s'. Valid categories: %s"
                    % (cat, ', '.join(sorted(valid)))
                )
        return value


class WebhookList(ListCreateAPIView):
    permission_classes = (IsProjectMaintainer,)
    serializer_class = WebhookSerializer

    def get_queryset(self):
        return Webhook.objects.filter(project_id=self.kwargs['project_id'])

    def perform_create(self, serializer):
        project = Project.objects.get(pk=self.kwargs['project_id'])
        serializer.save(project=project, creator=self.request.user)


class WebhookDetail(RetrieveUpdateDestroyAPIView):
    permission_classes = (IsProjectMaintainer,)
    serializer_class = WebhookSerializer

    def get_queryset(self):
        return Webhook.objects.filter(project_id=self.kwargs['project_id'])
