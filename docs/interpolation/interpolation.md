# Interpolation grammar (v1alpha1)

Templates use one expression syntax: `${{ <expr> }}`. Everything else is
literal text, including `${VAR}`, `$(VAR)`, and `$VAR`. Shell snippets in
`command` and `args` stay as they are.

## Grammar

```ebnf
expression  = "${{" ws expr ws "}}" ;
expr        = input-ref | output-ref | env-ref | runway-ref ;

input-ref   = "inputs." identifier ;
output-ref  = "services." dns-label ".outputs." env-key ;
env-ref     = "env." env-key ;
runway-ref  = "runway." ( identifier | secret-call ) ;
secret-call = "secret(" integer ")" ;

identifier  = [a-zA-Z_][a-zA-Z0-9_]* ;          (* input key pattern *)
dns-label   = [a-z]([a-z0-9-]{0,61}[a-z0-9])? ; (* service name pattern *)
env-key     = [A-Z_][A-Z0-9_]* ;                (* env/output key pattern *)
integer     = [1-9][0-9]* ;
ws          = " "* ;
```

- Spaces inside the braces are optional: `${{inputs.x}}` equals `${{ inputs.x }}`.
- To write a literal `${{`, use `$${{`. There is no other escape.
- A `${{` that is not followed by a valid expression and `}}` is a validation
  error. Broken expressions never pass through as text.
- Expressions can appear anywhere in a string and mix with literal text:
  `redis://default:${{ env.VALKEY_PASSWORD }}@${{ runway.fqdn }}:6379`.
- Expressions do not nest.

## Namespaces

- [inputs](./inputs.md)
- [env](./env.md)
- [services](./services.md)
- [runway](./runway.md)

## Allowed contexts

| Context               | `inputs` | `services.*.outputs` | `env` | `runway.*` | `runway.secret()` |
|-----------------------|:--------:|:--------------------:|:-----:|:--------:|:----------:|
| `service.image`       | yes      | no                   | no    | no       | no         |
| `command` / `args`    | yes      | yes                  | yes   | yes      | no         |
| `env` values          | yes      | yes                  | yes   | yes      | yes        |
| `outputs` values      | yes      | no                   | yes   | yes      | no         |
| healthcheck `path` / `command` | yes | no              | no    | no       | no         |

`init` and `workers` follow the same rules as the parent service's
`command`, `args`, and `env` contexts. Their `env.*` references may use keys
from the parent service's env (inherited) or their own overrides.

`runway.secret()` is limited to env values. This way, generated secrets
always live in a Secret object and can be referenced via `env.*` elsewhere.

## Validation rules

The reference lint in this repo checks these rules. Runway rejects
a deploy that breaks any of them:

1. Every `inputs.<name>` references a declared input.
2. Every `services.<s>.outputs.<K>` references a declared service and a
   declared output key; no self-reference; the cross-service graph is a DAG.
3. Every `env.<KEY>` references a declared env key in scope.
4. Every `runway.<field>` is a known field; `runway.url` only in services
   where `settings.route` is not false.
5. `runway.secret(n)` has `16 <= n <= 128` and appears only in env values.
6. Every expression appears only in a context allowed by the table above.
7. Malformed expressions (`${{` without valid expr + `}}`) are errors.

## Resolution order

How Runway resolves a template at deploy time:

1. Validate template + input values; fill input defaults.
2. Topologically sort services by output references.
3. Per service, in order: generate or load `runway.secret()` values, render
   `env` (keeping `env.*` references as `$(KEY)` for Kubernetes contexts),
   render `outputs` (resolving `env.*` to real values), then render `image`,
   `command`, `args`, and the healthcheck.
4. Any expression left unresolved after rendering is a deploy error.