# Teams (Office 365 Connectors)

Microsoft [retired Office 365 Connectors within Teams](https://devblogs.microsoft.com/microsoft365dev/retirement-of-office-365-connectors-within-microsoft-teams/) on May 22, 2026. The connector endpoints are no longer operational, so the `teams` notification service is no longer supported.

Use [Teams Workflows](./teams-workflows.md) to send notifications to Microsoft Teams. See its [migration instructions](./teams-workflows.md#migration-from-office-365-connectors) to update service settings, templates, and subscriptions.

Existing `service.teams` entries and `teams` template sections are ignored with migration warnings so other notification services can continue to load.
