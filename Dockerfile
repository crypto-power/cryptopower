FROM golang 

#Set build arguments
ARG BUILDOS
ARG BUILDARCH
ARG BUILDNAME

RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.35.2

ENV GO111MODULE on

RUN go version

RUN apt-get update

RUN apt-get install libwayland-dev libx11-dev libx11-xcb-dev libxkbcommon-x11-dev libgles2-mesa-dev libegl1-mesa-dev libffi-dev libxcursor-dev libvulkan-dev --yes

# RUN go install -v github.com/onsi/ginkgo/ginkgo@latest

# RUN go install -v github.com/onsi/gomega@latest

WORKDIR /app

#Copy all files from root into the container
COPY . ./

RUN go mod download

#Use go mod tidy to handle dependencies
RUN go mod tidy

#Compile the binary
RUN env GOOS=$BUILDOS GOARCH=$BUILDARCH go build -o $BUILDNAME -trimpath -ldflags=-buildid=
# ENTRYPOINT [ "go", "test"]
