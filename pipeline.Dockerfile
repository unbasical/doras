FROM scratch
ARG TARGETOS
ARG TARGETARCH

ENTRYPOINT ["/doras-server"]
COPY doras-server-${TARGETOS}-${TARGETARCH} /doras-server