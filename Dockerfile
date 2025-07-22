FROM scratch

LABEL org.opencontainers.image.source=https://github.com/scholzj/strimzi-shutdown
LABEL org.opencontainers.image.title="Strimzi Shutdown"
LABEL org.opencontainers.image.description="Simple utility to temporarily stop or restart your Strimzi-based Apache Kafka cluster"

ARG TARGETOS
ARG TARGETARCH

ADD strimzi-shutdown-*-${TARGETOS}-${TARGETARCH} /strimzi-shutdown

CMD ["/strimzi-shutdown"]