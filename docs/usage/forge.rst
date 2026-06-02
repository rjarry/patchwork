Forge Integration
=================

Patchwork can synchronize patch workflows with code forges such as GitHub.
When enabled, patch series submitted to a mailing list are automatically
converted into pull requests, and pull requests opened on a forge are converted
into patch series on the mailing list. Comments, reviews, and CI results flow
in both directions.

.. contents::
   :local:

Overview
--------

The forge integration provides bidirectional synchronization:

**Mailing list to forge:**

- A completed patch series creates a pull request on the forge.
- Respin submissions (v2, v3, ...) force-push to the same branch and post an
  update comment on the existing pull request.
- Comments on patches or cover letters are forwarded to the pull request.

**Forge to mailing list:**

- A pull request opened on the forge generates a patch series email sent to the
  mailing list and ingested directly into the Patchwork database.
- Respin (force-push) events generate a new version of the patch series with
  proper version numbering and optional threading to the original.
- Comments and reviews on the pull request are forwarded as email replies to the
  series.
- CI check results are recorded as Patchwork checks on each patch and
  summarized in an email to the list.

Loop prevention ensures that events originating from the sync itself are not
re-synced in the other direction.

Enabling Forge Support
----------------------

Forge support is gated behind the ``ENABLE_FORGE`` setting. To enable it, add
the following to your ``settings.py``:

.. code-block:: python

   ENABLE_FORGE = True
   FORGE_BACKENDS = ['patchwork.forge.github']

Each backend module is only imported when listed in ``FORGE_BACKENDS``.
Backend-specific dependencies (if any) are only required when the backend is
enabled.

Settings Reference
------------------

``ENABLE_FORGE``
~~~~~~~~~~~~~~~~

Set to ``True`` to enable forge integration. When disabled, no webhook
endpoints are registered and no sync signal handlers are active.

Default: ``False``

``FORGE_BACKENDS``
~~~~~~~~~~~~~~~~~~

A list of Python module paths for forge backends to load at startup. Each
module must call ``register_backend()`` to register itself.

Default: ``[]``

Example:

.. code-block:: python

   FORGE_BACKENDS = ['patchwork.forge.github']

``FORGE_WEBHOOK_SECRETS``
~~~~~~~~~~~~~~~~~~~~~~~~~

A dictionary mapping backend names to their webhook secrets. Used for
HMAC-SHA256 signature verification of incoming webhooks. If a backend's secret
is empty, signature verification is skipped.

Default: ``{}``

Example:

.. code-block:: python

   FORGE_WEBHOOK_SECRETS = {
       'github': 'your-webhook-secret',
   }

``FORGE_AUTH``
~~~~~~~~~~~~~~

Authentication credentials for forge backends. Each backend gets a top-level
dictionary with optional per-repository overrides under a ``repos`` key.

Default: ``{}``

Example:

.. code-block:: python

   FORGE_AUTH = {
       'github': {
           'token': 'ghp_default_token',
           'repos': {
               'owner/special-repo': {
                   'token': 'ghp_different_token',
               },
           },
       },
   }

When resolving credentials for a project, the backend-level defaults are merged
with any repository-specific overrides. This allows most projects to share the
same bot account while specific repositories use different credentials.

``FORGE_GIT_MIRROR_PATH``
~~~~~~~~~~~~~~~~~~~~~~~~~

Base directory for bare git mirror clones. One mirror is created per project
at ``{FORGE_GIT_MIRROR_PATH}/{project_linkname}.git``.

Default: ``''``

Example:

.. code-block:: python

   FORGE_GIT_MIRROR_PATH = '/var/cache/patchwork/mirrors'

``FORGE_BRANCH_PREFIX``
~~~~~~~~~~~~~~~~~~~~~~~

Prefix for branches created by patchwork on the forge. Used for loop
prevention: pull requests on branches starting with this prefix are recognized
as patchwork-created and skipped during forge-to-mailing-list sync.

Default: ``'patchwork'``

Per-Project Configuration
-------------------------

Each Patchwork project can be linked to one or more forge repositories through
the ``ForgeConfig`` model, configurable via the Django admin interface as an
inline on the Project admin page.

**Fields:**

``backend``
  The forge backend name (e.g. ``github``). Must match a registered backend.

``repo``
  The repository identifier on the forge (e.g. ``owner/repo`` for GitHub).

``from_email``
  Sender address for forge-originated emails. Falls back to
  ``DEFAULT_FROM_EMAIL`` if empty.

``sync_ml_to_forge``
  Enable creating pull requests from mailing list patch series. Default:
  ``True``.

``sync_forge_to_ml``
  Enable sending patch emails from forge pull requests to the mailing list.
  Default: ``True``.

``thread_respins``
  When enabled, respin series (v2, v3, ...) from the forge are threaded as
  replies to the original version's cover letter. When disabled, each version
  starts its own thread. Default: ``False``.

Webhook Setup
-------------

Each forge backend exposes a webhook endpoint at::

   /webhook/forge/<backend_name>/

For example, with the GitHub backend enabled::

   https://your-patchwork-instance.example.com/webhook/forge/github/

Configure this URL as the webhook endpoint on your forge, selecting the
events you want to synchronize.

GitHub Backend
--------------

The GitHub backend (``patchwork.forge.github``) supports the following webhook
events:

- ``pull_request`` (opened, synchronize)
- ``issue_comment`` (created)
- ``pull_request_review`` (submitted)
- ``check_run`` (created)
- ``check_suite`` (completed)

**Required GitHub webhook configuration:**

- Content type: ``application/json``
- Secret: must match ``FORGE_WEBHOOK_SECRETS['github']``
- Events: select the events listed above, or choose "Send me everything"

**Required permissions** (for a GitHub App or Personal Access Token):

- ``contents:read`` — to fetch pull request refs
- ``pull_requests:write`` — to create pull requests and post comments
- ``checks:read`` — to receive check run/suite events

**Authentication:**

The GitHub backend uses a Personal Access Token configured in ``FORGE_AUTH``:

.. code-block:: python

   FORGE_AUTH = {
       'github': {
           'token': 'ghp_your_token_here',
       },
   }

The token is used for both API calls (creating PRs, posting comments) and git
operations (cloning, fetching, pushing).

Loop Prevention
---------------

Three mechanisms prevent infinite sync loops:

**Branch prefix check:**
  Pull requests on branches starting with ``FORGE_BRANCH_PREFIX`` (default:
  ``patchwork/``) are recognized as patchwork-created and skipped during
  forge-to-mailing-list sync. This check is done at webhook parsing time.

**HTML comment marker:**
  All pull request bodies and comments posted by patchwork contain the
  ``<!-- patchwork -->`` HTML comment. Webhook events containing this marker
  are skipped during parsing.

**Email header check:**
  All emails sent by the forge sync include an ``X-Patchwork-Hint: ignore``
  header. When these emails return through the mail pipeline, ``parsemail``
  skips them. When processing mailing-list-to-forge sync, comments with this
  header are not forwarded to the forge.

Series Metadata
---------------

When a patch series is linked to a forge pull request, two metadata entries are
stored on the series:

- ``{backend}_pr`` — the full URL to the pull request (e.g.
  ``https://github.com/owner/repo/pull/42``)
- ``{backend}_branch`` — the branch name on the forge (e.g.
  ``patchwork/2a/fix-memory-leak``)

This metadata is used to find existing pull requests for respin detection and
to route comments and check results to the correct pull request.

Email Headers
-------------

Emails generated by the forge-to-mailing-list sync include the following
custom headers:

``X-Patchwork-Hint: ignore``
  Tells ``parsemail`` to skip this email when it returns through the mail
  pipeline (since it was already ingested directly).

``Sender``
  The patchwork bot address (from ``ForgeConfig.from_email`` or
  ``DEFAULT_FROM_EMAIL``).

``List-ID``
  The project's list ID, ensuring correct project routing.

``Reply-To``
  The project's mailing list address.

For patch series from pull requests, additional headers are added via
``git format-patch``:

``To``
  The project's mailing list address.

``Cc``
  Addresses extracted from commit trailers (Signed-off-by, Acked-by,
  Reviewed-by, Tested-by, etc.).
