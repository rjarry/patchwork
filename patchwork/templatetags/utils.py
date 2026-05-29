# Patchwork - automated patch tracking system
# Copyright (C) 2021 Google LLC
#
# SPDX-License-Identifier: GPL-2.0-or-later

from django import template
from django.utils.html import format_html

register = template.Library()


@register.filter
def verbose_name_plural(obj):
    return obj._meta.verbose_name_plural


@register.simple_tag
def is_editable(obj, user):
    return obj.is_editable(user)


@register.filter
def metadata_value(value):
    s = str(value)
    if s.startswith(('http://', 'https://')):
        return format_html('<a href="{}">{}</a>', s, s)
    return format_html('<code>{}</code>', s)
