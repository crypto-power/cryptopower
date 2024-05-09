# This how we want to name the binary output
BUILDNAME=cryptopower

VERSION="v1.1.3"
# dev or prod
BuildEnv="prod" 

build-binary:
	#Build the binary image
	docker build -t binary-image --build-arg BUILDOS=$(BUILDOS) --build-arg BUILDARCH=$(BUILDARCH) --build-arg BUILDNAME=$(BUILDNAME)-$(BUILDOS)-$(BUILDARCH)-$(VERSION) --no-cache .

	#Run the binary-image in a new container called binary
	docker run --name binary binary-image

	#Copy the compiled binary to the host machine
	docker cp binary:/app/$(BUILDNAME)-$(BUILDOS)-$(BUILDARCH)-$(VERSION) ./reproduciblebuilds
	
	#Remove the binary container
	docker rm binary

	#Remove the binary-image
	docker image rm binary-image

linux-amd64-binary: 
	make BUILDOS=linux BUILDARCH=amd64 build-binary

linux-arm64-binary: 
	make BUILDOS=linux BUILDARCH=arm64 build-binary

darwin-amd64-binary: 
	make BUILDOS=darwin BUILDARCH=amd64 build-binary

darwin-arm64-binary: 
	make BUILDOS=darwin BUILDARCH=arm64 build-binary

windows-amd64-binary: 
	make BUILDOS=windows BUILDARCH=amd64 build-binary

windows-386-binary: 
	make BUILDOS=windows BUILDARCH=386 build-binary

freebsd-amd64-binary: 
	make BUILDOS=freebsd BUILDARCH=amd64 build-binary

freebsd-arm-binary: 
	make BUILDOS=freebsd BUILDARCH=arm build-binary
 
# Cleans our project: deletes old binaries
clean:
	-rm -f ${BINARY}-*