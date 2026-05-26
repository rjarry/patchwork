# Patchwork - automated patch tracking system
# Copyright (C) 2016 Stephen Finucane <stephen@that.guru>
#
# SPDX-License-Identifier: GPL-2.0-or-later

from rest_framework.generics import ListAPIView
from rest_framework.generics import RetrieveUpdateAPIView
from rest_framework.serializers import (
    SerializerMethodField,
    HyperlinkedRelatedField,
)

from patchwork.api.base import BaseHyperlinkedModelSerializer
from patchwork.api.base import PatchworkPermission
from patchwork.api.filters import SeriesFilterSet
from patchwork.api.embedded import CoverSerializer
from patchwork.api.embedded import PatchSerializer
from patchwork.api.embedded import PersonSerializer
from patchwork.api.embedded import ProjectSerializer
from patchwork.models import Series
from patchwork.models import SeriesMetadata


class SeriesSerializer(BaseHyperlinkedModelSerializer):
    web_url = SerializerMethodField()
    project = ProjectSerializer(read_only=True)
    submitter = PersonSerializer(read_only=True)
    mbox = SerializerMethodField()
    cover_letter = CoverSerializer(read_only=True)
    patches = PatchSerializer(read_only=True, many=True)
    dependencies = HyperlinkedRelatedField(
        read_only=True, view_name='api-series-detail', many=True
    )
    dependents = HyperlinkedRelatedField(
        read_only=True, view_name='api-series-detail', many=True
    )
    metadata = SerializerMethodField()
    previous_series = HyperlinkedRelatedField(
        read_only=True, view_name='api-series-detail'
    )
    next_series = HyperlinkedRelatedField(
        read_only=True, view_name='api-series-detail', many=True
    )

    def get_web_url(self, instance):
        request = self.context.get('request')
        return request.build_absolute_uri(instance.get_absolute_url())

    def get_mbox(self, instance):
        request = self.context.get('request')
        return request.build_absolute_uri(instance.get_mbox_url())

    def get_metadata(self, instance):
        return {m.key: m.value for m in instance.metadata.all()}

    def to_representation(self, instance):
        if not instance.project.show_dependencies:
            for field in ('dependencies', 'dependents'):
                if field in self.fields:
                    del self.fields[field]

        return super().to_representation(instance)

    def to_internal_value(self, data):
        ret = super().to_internal_value(data)
        if 'metadata' in data:
            ret['metadata'] = data['metadata']
        return ret

    def update(self, instance, validated_data):
        metadata = validated_data.pop('metadata', None)
        instance = super().update(instance, validated_data)
        if metadata is not None:
            for key, value in metadata.items():
                if value is None:
                    SeriesMetadata.objects.filter(
                        series=instance, key=key
                    ).delete()
                else:
                    SeriesMetadata.objects.update_or_create(
                        series=instance,
                        key=key,
                        defaults={'value': str(value)},
                    )
        return instance

    class Meta:
        model = Series
        fields = (
            'id',
            'url',
            'web_url',
            'project',
            'name',
            'date',
            'submitter',
            'version',
            'total',
            'received_total',
            'received_all',
            'mbox',
            'cover_letter',
            'patches',
            'dependencies',
            'dependents',
            'metadata',
            'previous_series',
            'next_series',
        )
        read_only_fields = (
            'date',
            'submitter',
            'total',
            'received_total',
            'received_all',
            'mbox',
            'cover_letter',
            'patches',
            'dependencies',
            'dependents',
            'previous_series',
            'next_series',
        )
        versioned_fields = {
            '1.1': ('web_url',),
            '1.4': ('dependencies', 'dependents'),
            '1.5': ('metadata', 'previous_series', 'next_series'),
        }
        extra_kwargs = {
            'url': {'view_name': 'api-series-detail'},
        }


class SeriesMixin(object):
    permission_classes = (PatchworkPermission,)
    serializer_class = SeriesSerializer

    def get_queryset(self):
        return (
            Series.objects.all()
            .prefetch_related(
                'patches__project',
                'cover_letter__project',
                'dependencies',
                'dependents',
                'metadata',
                'next_series',
            )
            .select_related('submitter', 'project', 'previous_series')
        )


class SeriesList(SeriesMixin, ListAPIView):
    """List series."""

    filter_class = filterset_class = SeriesFilterSet
    search_fields = ('name',)
    ordering_fields = ('id', 'name', 'date', 'submitter', 'received_all')
    ordering = 'id'


class SeriesDetail(SeriesMixin, RetrieveUpdateAPIView):
    """
    get:
    Show a series.

    patch:
    Update a series.

    put:
    Update a series.
    """

    pass
