# `env.<KEY>`

An env var of the same service: the main container's `env` map, after its
own interpolation. The key must be declared in that map.

Resolution depends on context:

- In `env` values, `command`, and `args`: translated to Kubernetes `$(KEY)`
  expansion. The plain value never appears in the pod spec, so secret-backed
  vars stay inside their Secret object.
- In `outputs`: resolved at render time by the platform from the service's
  stored env. The platform knows all values, including generated secrets.