# Copyright 2020 The BizFly Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

FROM golang:alpine3.11  AS build-env
WORKDIR /app
ADD . /app
RUN cd /app && GO111MODULE=on GOARCH=amd64 go build -o csi-bizflycloud cmd/csi-bizflycloud/main.go

FROM amd64/alpine:3.11

RUN apk add --no-cache ca-certificates

COPY --from=build-env /app/csi-bizflycloud /bin/

CMD ["/bin/csi-bizflycloud"]