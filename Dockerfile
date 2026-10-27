# The page is prebuilt into web/dist and committed, so this build needs Go only.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/proofqc-server ./cmd/proofqc-server

# distroless/static has no shell or package manager; nonroot runs as uid 65532.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/proofqc-server /proofqc-server
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/proofqc-server"]
