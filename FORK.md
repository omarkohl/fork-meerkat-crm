# Fork releases

This fork of [fbuchner/meerkat-crm](https://github.com/fbuchner/meerkat-crm)
publishes its own images to `ghcr.io/omarkohl/meerkat-crm{,-backend,-frontend}`.

## Versions

`vX.Y.Z-omk.N`: `X.Y.Z` is the upstream tag the release is based on, `N`
counts fork releases on that base (e.g. `v1.7.0-omk.1`, `v1.7.0-omk.2`,
`v1.8.0-omk.1`). Fork tags never collide with upstream tags. There is no
`latest` or `X.Y` image tag.

## Integrate upstream

```sh
jj git fetch --remote upstream
jj rebase -b main -d 'tags(exact:"v1.8.0")'
```

## Release

```sh
git tag v1.8.0-omk.1 <commit>
git push origin v1.8.0-omk.1
```

The tag push runs `.github/workflows/docker-publish.yml`.
