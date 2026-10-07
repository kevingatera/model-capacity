FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY capacity/ capacity/
COPY cmd/ cmd/
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /model-capacity ./cmd/model-capacity
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /model-capacity /model-capacity
USER 65532:65532
EXPOSE 8388
ENTRYPOINT ["/model-capacity"]
