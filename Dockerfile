FROM --platform=$BUILDPLATFORM node:20-bookworm AS frontend-builder

WORKDIR /src/frontend

RUN corepack enable

COPY frontend ./ 

RUN pnpm install --frozen-lockfile
RUN pnpm build

FROM --platform=$TARGETPLATFORM golang:1.23-bookworm AS web-builder

WORKDIR /src

RUN apt-get update \
    && apt-get install -y --no-install-recommends build-essential pkg-config \
    && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=frontend-builder /src/frontend/dist ./frontend/dist

RUN CGO_ENABLED=1 go build -tags fts5 -o /out/BiliShareMallWeb ./cmd/web

FROM --platform=$TARGETPLATFORM debian:bookworm-slim AS runtime

ARG DEBIAN_FRONTEND=noninteractive

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /data /app/frontend

COPY --from=web-builder /out/BiliShareMallWeb /app/BiliShareMallWeb
COPY --from=web-builder /src/frontend/dist /app/frontend/dist
COPY dict /app/dict

ENV BSM_BASE_PATH=/app
ENV BSM_DATA_DIR=/data
ENV BSM_HTTP_ADDR=:3754

EXPOSE 3754

CMD ["/app/BiliShareMallWeb"]
