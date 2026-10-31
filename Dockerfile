FROM node:24-bookworm-slim AS web-builder

WORKDIR /build/web

COPY web/ ./
RUN npm ci
RUN npm run build

FROM golang:1.26-bookworm AS go-builder

WORKDIR /src

COPY go.work go.work.sum ./
COPY shared/ ./shared/
COPY api/ ./api/
COPY cmd/ ./cmd/
COPY --from=web-builder /build/web/dist ./api/web_dist

WORKDIR /src/cmd

RUN go mod download \
 && CGO_ENABLED=1 go build -ldflags='-s -w' -o /out/linkomat-server ./linkomat-server

FROM gcr.io/distroless/base-debian12:nonroot

COPY --from=go-builder /out/linkomat-server /linkomat-server

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/linkomat-server", "serve"]
