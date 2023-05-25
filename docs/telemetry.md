# Telemetry

AnyCable v1.4+ collects **anonymous usage** information. Users are notified on the server start.

Why collecting telemetry? One of the biggest issues in the open-source development is the lack of feedback (especially, when everything works as expected). Getting more insights on how AnyCable is used in the wild will help us to prioritize work on new features and improvements.

## Opting out

You can disable telemetry by setting the `ANYCABLE_DISABLE_TELEMETRY` environment variable to `true`.

## What is collected

We collect the following information:

- AnyCable version.
- OS name.
- Specific features enabled (e.g., JWT identification, signed streams, etc.).
- Max observed amount of RAM used by the process.
- Max observed number of concurrent connections (this helps us to distinguish development/test runs from production ones).

We **do not collect** personally-identifiable or sensitive information, such as: hostnames, file names, environment variables, or IP addresses.

We use [Posthog](https://posthog.com/) to store and visualize data.
