FROM golang:1.26.4-alpine3.22 AS build
WORKDIR /src
ARG BUILD_VERSION=dev
ARG BUILD_COMMIT=dev
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build \
      -ldflags="-X main.version=${BUILD_VERSION} -X main.commit=${BUILD_COMMIT}" \
      -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build \
      -ldflags="-X main.version=${BUILD_VERSION} -X main.commit=${BUILD_COMMIT}" \
      -o /out/worker ./cmd/worker

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/api /api
COPY --from=build /out/worker /worker
COPY migrations /migrations
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/api"]
