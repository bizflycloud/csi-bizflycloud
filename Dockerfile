# Copyright 2020 The Bizfly Cloud Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

FROM golang:1.24-bookworm AS build-env
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . /app
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o csi-bizflycloud cmd/csi-bizflycloud/main.go

FROM --platform=linux/amd64 debian:bookworm-slim

RUN apt-get update && apt-get install -y ca-certificates e2fsprogs mount xfsprogs udev && rm -rf /var/lib/apt/lists/*

COPY --from=build-env /app/csi-bizflycloud /bin/

CMD ["/bin/csi-bizflycloud"]
