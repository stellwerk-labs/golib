# hservicejwt

This package provides some common utilities for encoding, decoding, and working with service JWTs that may be passed
between services to indicate an authenticated or delegated set of permissions.

This may be used in the following use cases:

## Delegating permissions to a system user

When Platform Orchestrator needs to take action in a customers organisation with a limited set of permissions it may use a system
user. This is different to a _service_ user which is a user that belongs to an Organization and whos permissions are
managed dynamically by administrators and managers in that Organization but are not very accessible to developers.

Instead:

1. The users service creates a system user for the target service, eg: "Platform Orchestrator Pipelines System User". The user id
   looks like a service user `s-01234567-89ab-cdef-0123-456789abcdef`.
2. The service using the system user collects a set of object roles from the customer when the customer creates a resource
    or link. For example: `/orgs/my-org: member`, `/orgs/my-org/apps/my-app: developer`.
3. The service then generates a Service JWT using this information.
4. When sending these delegated permissions to the auth enforcer or another service, they should be sent as a JWT in the
    `X-Service-Authorization`.
