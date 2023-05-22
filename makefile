# This how we want to name the binary output
BINARY=cryptopower
BUILDNAME=cryptopower

VERSION="1.7.0"
BUILD=`date -u +"%Y-%m-%dT%H:%M:%SZ"`
# dev or prod
BuildEnv="prod" 

LDFLAGS=-ldflags "-w -s -X main.Version=${VERSION} -X main.BuildDate=${BUILD} -X main.BuildEnv=${BuildEnv}"
# LDFLAGSWIN adds the -H=windowsgui flag to windows build to prevent cli from starting alongside godcr
LDFLAGSWIN= -ldflags "-H=windowsgui -w -s -X main.Version=${VERSION} -X main.BuildDate=${BUILD} -X main.BuildEnv=${BuildEnv}"

all: clean macos windows linux freebsd

freebsd:
	GOOS=freebsd GOARCH=amd64 go build -trimpath ${LDFLAGS} -o ${BINARY}-freebsd-${GOARCH}
	GOOS=freebsd GOARCH=arm64 go build -trimpath ${LDFLAGS} -o ${BINARY}-freebsd-${GOARCH}

linux-binary:
	#Build the linux-binary image
	docker build -t linux-binary-image --build-arg BUILDOS=linux --build-arg BUILDARCH=amd64 --build-arg BUILDNAME=$(BUILDNAME) --no-cache .

	#Run the linux-binary-image in a new container called linux-binary
	docker run --name linux-binary  linux-binary-image

	#Copy the compiled linux binary to the host machine
	docker cp linux-binary:/app/$(BUILDNAME) /
	
	#Remove the linux-binary container
	docker rm linux-binary

	#Remove the linux-binary-image
	docker image rm linux-binary-image

macos:
	GOOS=darwin GOARCH=amd64 go build -trimpath ${LDFLAGS} -o ${BINARY}-darwin-${GOARCH}
	GOOS=darwin GOARCH=arm64 go build -trimpath ${LDFLAGS} -o ${BINARY}-darwin-${GOARCH}

windows:
	GOOS=windows GOARCH=amd64 go build -trimpath ${LDFLAGSWIN} -o ${BINARY}-windows-${GOARCH}.exe
 
# Cleans our project: deletes old binaries
clean:
	-rm -f ${BINARY}-*

.PHONY: clean darwin windows linux freebsd