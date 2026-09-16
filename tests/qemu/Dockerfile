FROM golang:1.24-alpine AS build

WORKDIR /app
COPY go.mod ./
COPY . .
ENV CGO_ENABLED=0
RUN go build -o /app/mbwol .

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    qemu-system-x86 \
    ovmf \
    grub-efi-amd64-bin \
    grub-common \
    dosfstools \
    mtools \
    curl \
    python3 \
    iproute2 \
    procps \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /tests/qemu

COPY --from=build /app/mbwol /usr/local/bin/mbwol
COPY tests/qemu/grub.cfg /tests/qemu/grub.cfg
COPY tests/qemu/run_test.sh /tests/qemu/run_test.sh

RUN chmod +x /tests/qemu/run_test.sh

ENTRYPOINT ["/tests/qemu/run_test.sh"]
