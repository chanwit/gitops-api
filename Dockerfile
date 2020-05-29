FROM golang:1.13
COPY . /app/
WORKDIR /app
RUN go mod download
RUN go build -o main .

FROM ubuntu:18.04

RUN apt-get update
RUN apt-get install -y curl git openssh-client

RUN git config --global user.email "gitops-api@noreply.flux"
RUN git config --global user.name  "GitOps API"

RUN curl -s --location "https://github.com/mikefarah/yq/releases/download/3.3.0/yq_linux_amd64" > /usr/local/bin/yq \
    && chmod +x /usr/local/bin/yq

COPY --from=0 /app/main /main
RUN  chmod +x /main

ENTRYPOINT ["/main"]
