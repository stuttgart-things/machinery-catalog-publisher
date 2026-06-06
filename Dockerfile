# Build the publisher. Note: the module imports machinery (module "maschinist")
# via a local replace to ../machinery, so the build context must include both
# repos, OR machinery must be published/vendored and the replace removed.
FROM golang:1.26 AS build
WORKDIR /src

# Expect both repos under the build context: ./machinery and ./machinery-catalog-publisher
COPY machinery/ /machinery/
COPY machinery-catalog-publisher/ /src/

ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
RUN go mod download && \
    CGO_ENABLED=0 go build -trimpath \
      -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
      -o /out/publisher ./cmd/server

FROM gcr.io/distroless/static:nonroot
USER 65532:65532
COPY --from=build /out/publisher /publisher
ENTRYPOINT ["/publisher"]
