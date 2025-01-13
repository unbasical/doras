# Doras

**Disclaimer:** this software is still **experimental** and for now has no guarantees about API stability.

Doras is a system for delta updates for artifacts stored in OCI registries.
It currently does not support container images, but may in the future.

For now, it primarily targets IoT scenarios where the network bandwidth and storage space are constrained resources.

## General Operation

1. A Doras client requests a delta from one image to another.
2. The Doras server fetches the two images from a registry and constructs an optionally compressed delta.
3. The server tells the client where the delta is stored.
4. The client fetches the delta and applies it.

## Design

- The design aims to be flexible to allow different diffing and compression algorithms.
- We attempt to be compatible with OCI specs and allow for a broader scope of supported artifacts (e.g. container images) in the future.
- The client library aims to address issues that might arise in IoT settings such as:
  - Unreliable network connections while files are downloaded.
  - Robustness against power loss during the update flow.
  - Storage overhead during the update flow.
- The server itself is designed to have very little state, with most of the state being stored in the registry itself. 
  The server's internal state is limited to what is necessary to avoid duplicate requests.


## Documentation

View the [docs directory](./docs) for more information such as specifications.
This includes topics such as:
- How deltas are created, stored and located.
- OpenAPI specification.

## License

This software uses the Apache License 2.0.
