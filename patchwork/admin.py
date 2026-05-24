# Patchwork - automated patch tracking system
# Copyright (C) 2008 Jeremy Kerr <jk@ozlabs.org>
#
# SPDX-License-Identifier: GPL-2.0-or-later

from django import forms
from django.contrib import admin
from django.contrib.auth.admin import UserAdmin as BaseUserAdmin
from django.contrib.auth.models import User
from django.db.models import Prefetch

from patchwork.models import Bundle
from patchwork.models import Check
from patchwork.models import Cover
from patchwork.models import CoverComment
from patchwork.models import DelegationRule
from patchwork.models import Event
from patchwork.models import Patch
from patchwork.models import PatchComment
from patchwork.models import PatchRelation
from patchwork.models import Person
from patchwork.models import Project
from patchwork.models import Series
from patchwork.models import SeriesMetadata
from patchwork.models import SeriesReference
from patchwork.models import State
from patchwork.models import Tag
from patchwork.models import UserProfile
from patchwork.models import Webhook


class UserProfileInline(admin.StackedInline):
    model = UserProfile
    can_delete = False
    verbose_name_plural = 'user profile'


class UserAdmin(BaseUserAdmin):
    inlines = (UserProfileInline,)


admin.site.unregister(User)
admin.site.register(User, UserAdmin)


class DelegationRuleInline(admin.TabularInline):
    model = DelegationRule
    fields = ('path', 'user', 'priority')


@admin.register(Project)
class ProjectAdmin(admin.ModelAdmin):
    list_display = ('name', 'linkname', 'listid', 'listemail')
    inlines = [
        DelegationRuleInline,
    ]


@admin.register(Person)
class PersonAdmin(admin.ModelAdmin):
    list_display = ('__str__', 'has_account')
    search_fields = ('name', 'email')

    @admin.display(
        description='Account',
        boolean=True,
        ordering='user',
    )
    def has_account(self, person):
        return bool(person.user)


@admin.register(State)
class StateAdmin(admin.ModelAdmin):
    list_display = ('name', 'action_required')


@admin.register(Cover)
class CoverAdmin(admin.ModelAdmin):
    list_display = ('name', 'submitter', 'project', 'date')
    list_filter = ('project',)
    search_fields = ('name', 'submitter__name', 'submitter__email')
    date_hierarchy = 'date'


@admin.register(Patch)
class PatchAdmin(admin.ModelAdmin):
    list_display = (
        'name',
        'submitter',
        'project',
        'state',
        'date',
        'archived',
        'is_pull_request',
    )
    list_filter = ('project', 'submitter', 'state', 'archived')
    list_select_related = ('submitter', 'project', 'state')
    search_fields = ('name', 'submitter__name', 'submitter__email')
    date_hierarchy = 'date'

    @admin.display(
        description='Pull',
        boolean=True,
        ordering='pull_url',
    )
    def is_pull_request(self, patch):
        return bool(patch.pull_url)


@admin.register(CoverComment)
class CoverCommentAdmin(admin.ModelAdmin):
    list_display = ('cover', 'submitter', 'date')
    search_fields = ('cover__name', 'submitter__name', 'submitter__email')
    date_hierarchy = 'date'


@admin.register(PatchComment)
class PatchCommentAdmin(admin.ModelAdmin):
    list_display = ('patch', 'submitter', 'date')
    search_fields = ('patch__name', 'submitter__name', 'submitter__email')
    date_hierarchy = 'date'


class PatchInline(admin.StackedInline):
    model = Patch
    extra = 0


@admin.register(Series)
class SeriesAdmin(admin.ModelAdmin):
    list_display = (
        'name',
        'submitter',
        'project',
        'date',
        'version',
        'total',
        'received_total',
        'received_all',
    )
    list_filter = ('project', 'submitter')
    list_select_related = ('submitter', 'project')
    readonly_fields = ('received_total', 'received_all')
    search_fields = ('submitter__name', 'submitter__email')
    exclude = ('patches',)
    filter_horizontal = ('dependencies',)
    inlines = (PatchInline,)

    @admin.display(boolean=True)
    def received_all(self, series):
        return series.received_all

    def get_queryset(self, request):
        qs = super(SeriesAdmin, self).get_queryset(request)
        return qs.prefetch_related(
            Prefetch(
                'patches',
                Patch.objects.only(
                    'series',
                ),
            )
        )


@admin.register(SeriesReference)
class SeriesReferenceAdmin(admin.ModelAdmin):
    model = SeriesReference


@admin.register(SeriesMetadata)
class SeriesMetadataAdmin(admin.ModelAdmin):
    model = SeriesMetadata
    list_display = ('series', 'key', 'value')


@admin.register(Check)
class CheckAdmin(admin.ModelAdmin):
    list_display = (
        'patch',
        'user',
        'state',
        'target_url',
        'description',
        'context',
    )
    exclude = ('date',)
    search_fields = ('patch__name', 'project__name')
    date_hierarchy = 'date'


@admin.register(Bundle)
class BundleAdmin(admin.ModelAdmin):
    list_display = ('name', 'owner', 'project', 'public')
    list_filter = ('public', 'project')
    search_fields = ('name', 'owner')


@admin.register(Tag)
class TagAdmin(admin.ModelAdmin):
    list_display = ('name',)


@admin.register(PatchRelation)
class PatchRelationAdmin(admin.ModelAdmin):
    model = PatchRelation


class WebhookForm(forms.ModelForm):
    ALL_EVENTS = '*'

    event_select = forms.MultipleChoiceField(
        choices=Event.CATEGORY_CHOICES,
        widget=forms.CheckboxSelectMultiple,
        required=False,
        label='Events',
        help_text='Select which events trigger this webhook. '
        'Leave empty to receive all events.',
    )

    class Meta:
        model = Webhook
        fields = ('project', 'url', 'secret', 'active')

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        if self.instance.pk:
            events = self.instance.events
            if events != self.ALL_EVENTS:
                self.initial['event_select'] = events.split(',')

    def save(self, commit=True):
        selected = self.cleaned_data.get('event_select', [])
        if selected:
            self.instance.events = ','.join(selected)
        else:
            self.instance.events = self.ALL_EVENTS
        return super().save(commit)


@admin.register(Webhook)
class WebhookAdmin(admin.ModelAdmin):
    form = WebhookForm
    list_display = ('project', 'url', 'events', 'active', 'creator', 'created')
    list_filter = ('project', 'active')
    search_fields = ('url', 'project__name')
    readonly_fields = ('created',)

    def save_model(self, request, obj, form, change):
        if not change:
            obj.creator = request.user
        super().save_model(request, obj, form, change)
