# Use distroless as minimal base image to package the observability-operator binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
ARG TARGETARCH
WORKDIR /

ADD observability-operator-linux-${TARGETARCH} observability-operator

USER 65532:65532

ENTRYPOINT ["/observability-operator"]
