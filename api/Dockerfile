FROM golang:1.25.10-alpine3.22 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build -o /out/worker ./cmd/worker

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/api /api
COPY --from=build /out/worker /worker
COPY migrations /migrations
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/api"]
