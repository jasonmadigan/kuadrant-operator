# TokenRateLimitPolicy

## Overview

TokenRateLimitPolicy enables token-based rate limiting for service workloads in a Gateway API network. This policy allows you to create rate limits that are based on tokens extracted from authenticated requests, such as user identity or group membership.

## Key Features

- **Token-based Rate Limiting**: Create rate limits based on authentication tokens and claims
- **CEL Expression Support**: Use CEL expressions for flexible predicate and counter definitions
- **Gateway API Integration**: Targets Gateway and HTTPRoute resources
- **Multiple Time Windows**: Support for various time windows (seconds, minutes, hours, days)

## API Reference

### TokenRateLimitPolicySpec

| Field | Type | Description |
|-------|------|-------------|
| `targetRef` | `LocalPolicyTargetReferenceWithSectionName` | Reference to the Gateway or HTTPRoute to which this policy applies |
| `limit` | `TokenLimit` | The token-based rate limit configuration |

### TokenLimit

| Field | Type | Description |
|-------|------|-------------|
| `rate` | `TokenRate` | Rate limit details including limit and window |
| `predicate` | `string` | CEL expression that determines if this limit applies to the request |
| `counter` | `string` | CEL expression that defines the counter key for rate limiting |

### TokenRate

| Field | Type | Description |
|-------|------|-------------|
| `limit` | `int` | Maximum number of requests allowed in the specified window |
| `window` | `string` | Time window using Gateway API Duration format (e.g., "1h", "30m", "1d") |

## Examples

### Basic Token Rate Limiting

```yaml
apiVersion: kuadrant.io/v1alpha1
kind: TokenRateLimitPolicy
metadata:
  name: token-limit-free
  namespace: kuadrant-system
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: Gateway
    name: my-llm-gateway
  limit:
    rate:
      limit: 20000
      window: 1d
    predicate: 'request.auth.claims["kuadrant.io/groups"].split(",").exists(g, g == "free")'
    counter: auth.identity.userid
```

This example:
- Targets a Gateway named `my-llm-gateway`
- Allows 20,000 requests per day
- Only applies to users in the "free" group (based on JWT claims)
- Uses the user ID as the counter key

### HTTPRoute-specific Rate Limiting

```yaml
apiVersion: kuadrant.io/v1alpha1
kind: TokenRateLimitPolicy
metadata:
  name: api-premium-users
  namespace: api-namespace
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: HTTPRoute
    name: api-route
  limit:
    rate:
      limit: 1000
      window: 1h
    predicate: 'request.auth.claims["subscription"] == "premium"'
    counter: request.auth.claims.sub
```

## CEL Expressions

TokenRateLimitPolicy supports CEL (Common Expression Language) for both predicates and counters:

### Common Predicate Examples

```cel
# Check user group membership
request.auth.claims["groups"].split(",").exists(g, g == "admin")

# Check subscription tier
request.auth.claims["subscription"] == "premium"

# Check request path
request.url_path.startsWith("/api/v1/")

# Combined conditions
request.auth.claims["tier"] == "gold" && request.method == "POST"
```

### Common Counter Examples

```cel
# Use user ID
auth.identity.userid

# Use JWT subject claim
request.auth.claims.sub

# Use organization ID
request.auth.claims["org_id"]

# Composite key
request.auth.claims["org_id"] + ":" + request.auth.claims.sub
```

## Status Conditions

TokenRateLimitPolicy reports status through standard Gateway API conditions:

- **Accepted**: Indicates whether the policy has been accepted by the controller
- **Enforced**: Indicates whether the policy is being actively enforced

## Limitations

- Currently supports Gateway and HTTPRoute targets only
- Requires authentication to be configured for token extraction
- CEL expressions must be valid and compile successfully
- Only one TokenRateLimitPolicy per target resource is supported

## See Also

- [RateLimitPolicy](ratelimitpolicy.md) - For non-token-based rate limiting
- [AuthPolicy](authpolicy.md) - For authentication configuration
- [Gateway API Documentation](https://gateway-api.sigs.k8s.io/)