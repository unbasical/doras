# Doras Delta Creation

## How Deltas are calculated.

The following figure shows how a delta between two OCI artifacts is calculated.
We see:
- Deltas are calculated per layer.
- Input layers can be decompressed, if they are compressed in the first place.
- Deltas can be compressed (Null algorithm possible).
- Information about algorithms is stored in the delta manifest via the `mediaType`.
- Deltas are stored in a way that would produce a file per layer if downloaded via oras pull.

![Delta Calculation](images/doras-delta-calculation-delta-calculation.drawio.svg)

## How Delta Requests are Handled

The following figure shows how the server handles delta requests.
Consider the following notes:
- Deltas are calculated asynchronously from request handling. 
- Requests launch the calculation but never live for the entire duration of the request.

![Delta Requests](images/doras-delta-calculation-delta-creation-server-flow.drawio.svg)


