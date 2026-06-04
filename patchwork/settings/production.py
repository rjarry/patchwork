"""
Patchwork production settings for forge sync deployment.
"""

import os

from .base import *  # noqa

DEFAULT_FROM_EMAIL = os.environ.get("FROM_EMAIL", DEFAULT_FROM_EMAIL)
SERVER_EMAIL = DEFAULT_FROM_EMAIL
NOTIFICATION_FROM_EMAIL = DEFAULT_FROM_EMAIL
EMAIL_SUBJECT_PREFIX = "[patchwork] "

SECRET_KEY = os.environ["DJANGO_SECRET_KEY"]

ALLOWED_HOSTS = os.environ.get("ALLOWED_HOSTS", "localhost").split(",")

DATABASES = {
    "default": {
        "ENGINE": "django.db.backends.postgresql_psycopg2",
        "HOST": os.environ.get("DATABASE_HOST", "localhost"),
        "PORT": os.environ.get("DATABASE_PORT", ""),
        "NAME": os.environ.get("DATABASE_NAME", "patchwork"),
        "USER": os.environ.get("DATABASE_USER", "patchwork"),
        "PASSWORD": os.environ.get("DATABASE_PASSWORD", ""),
    },
}

STATIC_ROOT = os.environ.get("STATIC_ROOT", "/var/www/patchwork")

STATICFILES_STORAGE = "django.contrib.staticfiles.storage.ManifestStaticFilesStorage"

TIME_ZONE = "UTC"

LOGGING = {
    "version": 1,
    "disable_existing_loggers": False,
    "formatters": {
        "syslog": {
            "format": "%(name)s: %(message)s",
        },
    },
    "handlers": {
        "syslog": {
            "class": "logging.handlers.SysLogHandler",
            "address": "/dev/log",
            "facility": "daemon",
            "formatter": "syslog",
        },
    },
    "root": {
        "handlers": ["syslog"],
        "level": "INFO",
    },
}

ENABLE_FORGE = True
FORGE_BACKENDS = ["patchwork.forge.github"]
FORGE_WEBHOOK_SECRETS = {
    "github": os.environ.get("GH_WEBHOOK_SECRET", ""),
}
FORGE_GIT_MIRROR_PATH = "/var/cache/patchwork/mirrors"
FORGE_AUTH = {
    "github": {
        "token": os.environ.get("GH_TOKEN", ""),
    },
}
