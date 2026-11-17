# schema

The schema for templates on [Runway](https://www.runway.horse), and a tool to check them.

A template describes ready-made software (a database, a cache, a search engine)
that Runway deploys to an app. Each template is one `template.yaml` in its own
repository in the [runway-templates](https://github.com/runway-templates) organization.

- [Templates guide](https://www.runway.horse/docs/guides/templates/): how to deploy and use templates
- [`v1alpha1.json`](./v1alpha1.json): the JSON schema
- [Interpolation](./docs/interpolation/interpolation.md): the `${{ ... }}` expressions

## Example

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/runway-templates/schema/refs/heads/main/v1alpha1.json
apiVersion: schema.runway.horse/templates/v1alpha1
kind: Template

metadata:
  name: valkey
  version: 0.2.0
  displayName: Valkey
  description: In-memory key-value store.
  category: cache

inputs:
  image_tag:
    type: string
    default: "9.1.2-alpine"

services:
  - name: valkey
    image: "valkey/valkey:${{ inputs.image_tag }}"
    env:
      PORT: "6379"
      VALKEY_PASSWORD:
        value: "${{ runway.secret(32) }}"
    volume:
      mountPath: /data
    outputs:
      DSN: "redis://default:${{ env.VALKEY_PASSWORD }}@${{ runway.internal_address }}"
    settings:
      route: false
    minPlan: free-launch
```

The first line gives you completion and checks in editors that support the
YAML language server. See the [valkey](https://github.com/runway-templates/valkey)
repository for a complete template.

## Tool

The tool checks a template against the schema and the lint rules. Runway
rejects templates that fail these checks.

```sh
go install github.com/runway-templates/schema/cmd/tool@latest

tool validate template.yaml # schema only
tool lint template.yaml     # schema and lint rules
```

In GitHub Actions:

```yaml
steps:
  - uses: actions/checkout@v7
  - uses: runway-templates/schema/.github/actions/setup-tool@main
  - run: tool lint template.yaml
```

## Contributing

Issues for all templates, the schema and the tool are tracked in
[this repository](https://github.com/runway-templates/schema/issues).

### Existing templates

Contributions to existing templates are very welcome: new versions, new
inputs, better defaults, fixes and docs.

- [Open an "Existing template" issue](https://github.com/runway-templates/schema/issues/new?template=existing-template.yml) to report a problem or suggest a change.
- Open a pull request in the repository of the template.

### New templates

[Open a "New template" issue](https://github.com/runway-templates/schema/issues/new?template=new-template.yml)
to request a template, or to offer to build one.

If you want to build it, we create a repository in this organization and give
you access. A template repository contains:

- `template.yaml`: the template
- `README.md`: inputs, outputs and notes on how to connect
- `.github/workflows/ci.yml`: runs `tool lint`

### Schema and tool

Open an [issue](https://github.com/runway-templates/schema/issues) or a pull request in this repository.

```sh
make test
make lint
make run-dev # lint the valid test templates
```

## License

[MPL-2.0](./LICENSE)
