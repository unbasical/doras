# Storage

This document specifies how deltas created by Doras are stored in a registry. The scope includes:
- how delta metadata is stored,
- how deltas are located,
- how placeholders are stored.

## Doras Delta Manifest

Deltas will be created per layer and stored as a layer. For now we only support single layer artifacts.
To make sure deltas are pushed as a single layer use the `doras push` wrapper or make sure to use `oras push` with a single directory or file as argument.

Some metadata will be stored in the annotations of the manifest, refer to [the specification on annotations](https://github.com/opencontainers/image-spec/blob/main/annotations.md#rules). 
The metadata always includes the following:
- Source images:
  - Annotation key `com.unbasical.doras.delta.from` stores the image string **from** which the delta was calculated.
  - Annotation key `com.unbasical.doras.delta.to` stores the image string **to** which the delta was calculated.
- Creation timestamp: stored in the annotation key `org.opencontainers.image.created`.
- The delta and compression algorithms are stored in the media type:
  - `application/bsdiff` indicates an uncompressed bsdiff delta.
  - `application/bsdiff+gzip` indicates a bsdiff delta that was compressed with `gzip`. This is in line with [how compression is handled usually](https://github.com/opencontainers/image-spec/blob/main/layer.md#gzip-media-types)

It can also optionally include:
- annotation key `com.unbasical.doras.delta.dummy` to indicate a dummy to communicate that a delta has not been stored yet but will soon be pushed.

A delta between two layers of index `i` is stored in layer `i` that is referenced by the manifest.

An example manifest can look like this:

```json
 {
  "schemaVersion": 2,
  "mediaType": "application/vnd.oci.image.manifest.v1+json",
  "artifactType": "application/vnd.example+type",
  "config": {
    "mediaType": "application/vnd.oci.empty.v1+json",
    "digest": "sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a",
    "size": 2
  },
  "layers": [
    {
      "mediaType": "application/bsdiff",
      "digest": "sha256:d2a84f4b8b650937ec8f73cd8be2c74add5a911ba64df27458ed8229da804a26",
      "size": 12,
      "annotations": {
        "org.opencontainers.image.title": "patch.bsdiff"
      }
    }
  ],
  "annotations": {
    "com.unbasical.doras.delta.from": "registry.example.org/foo@sha256:a...",
    "com.unbasical.doras.delta.to": "registry.example.org/foo@sha256:b...",
    "org.opencontainers.image.created": "2023-08-03T00:21:51Z"
  }
}
```

### Dummy Deltas

The algorithm described in #9 requires a way to store dummies (to indicate that this delta is being created).
A dummy is indicated by an annotation with the key `com.unbasical.doras.delta.dummy`,
timeouts and such are based on the`org.opencontainers.image.created` key.

```json
 {
  "schemaVersion": 2,
  "mediaType": "application/vnd.oci.image.manifest.v1+json",
  "artifactType": "application/vnd.example+type",
  "config": {
    "mediaType": "application/vnd.oci.empty.v1+json",
    "digest": "sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a",
    "size": 2
  },
  "layers": [
    {
      "mediaType": "application/vnd.oci.empty.v1+json",
      "digest": "sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a",
      "size": 2,
      "data": "e30="
    }
  ],
  "annotations": {
    "com.unbasical.doras.delta.from": "registry.example.org/foo@sha256:a...",
    "com.unbasical.doras.delta.to": "registry.example.org/foo@sha256:b...",
    "com.unbasical.doras.delta.algorithm": "bsdiff",
    "com.unbasical.doras.delta.dummy": "true",
    "org.opencontainers.image.created": "2023-08-03T00:21:51Z"
  }
}
```
A dummy manifest must have a single layer with an [empty descriptor](https://github.com/opencontainers/image-spec/blob/main/manifest.md#guidance-for-an-empty-descriptor). This is for portability reasons and in line with [the specification](https://github.com/opencontainers/image-spec/blob/main/manifest.md#guidance-for-an-empty-descriptor).

## Avoiding Collisions (Path/Tagging Mechanism)

We want to avoid collisions regarding delta files. Resolving a delta should always lead to compatible deltas. 
An incompatible delta would be one that uses a different delta algorithm.

To avoid such collisions we need to be able to resolve a tuple of
`(fromDescriptor, toDescriptor, deltaAlgorithm, compressionAlgorithm)`
to a tagged OCI image at which we only ever store a delta between these two source images, using the two algorithms.

We use the following mechanism URI:
```
<repo-name>:_delta-<sha256(<digest-from>|<digest-to>|<delta-algo>_<compression-algo>)>
```

In practice this results in identifiers such as:
```
registry.example.org/foobar:deltas_44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a
```