# Copyright 2020 The BizFly Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

FROM golang:1.14-stretch  AS build-env
WORKDIR /app
ADD . /app
RUN cd /app && GO111MODULE=on GOARCH=amd64 go build -o csi-bizflycloud cmd/csi-bizflycloud/main.go

FROM k8s.gcr.io/debian-base-amd64:1.0.0

RUN clean-install ca-certificates e2fsprogs mount xfsprogs udev

COPY --from=build-env /app/csi-bizflycloud /bin/

CMD ["/bin/csi-bizflycloud"]