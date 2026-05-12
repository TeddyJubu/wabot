FROM golang:1.25-bookworm AS build
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc libc6-dev \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=1
RUN go build -trimpath -ldflags="-s -w" -o /out/wabot ./cmd/wabot
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/wa ./cmd/wa \
 && go build -trimpath -ldflags="-s -w" -o /out/inbox-echo ./cmd/inbox-echo

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libc6 \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /data
VOLUME ["/data"]
COPY --from=build /out/wabot /out/wa /out/inbox-echo /usr/local/bin/
EXPOSE 7777
# Expect wabot.env and store.db under /data (mount a volume).
CMD ["wabot"]
