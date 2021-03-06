default: build

prepare:
	go get ./...

test: prepare
	go test ./...

fmt:
	go fmt .

build: fmt test
	go build -v

install: build
	mkdir -p ~/.packer.d/plugins
	install ./packer-provisioner-puppet-bolt ~/.packer.d/plugins/

release: test
	mkdir -p releases
	go get github.com/mitchellh/gox
	gox -osarch="darwin/amd64 darwin/386 linux/amd64 linux/386 windows/amd64 windows/386" --output 'dist/{{.OS}}_{{.Arch}}/{{.Dir}}'
	zip -j releases/packer-provisioner-puppet-bolt_darwin_386.zip    dist/darwin_386/packer-provisioner-puppet-bolt
	zip -j releases/packer-provisioner-puppet-bolt_darwin_amd64.zip  dist/darwin_amd64/packer-provisioner-puppet-bolt
	zip -j releases/packer-provisioner-puppet-bolt_linux_386.zip     dist/linux_386/packer-provisioner-puppet-bolt
	zip -j releases/packer-provisioner-puppet-bolt_linux_amd64.zip   dist/linux_amd64/packer-provisioner-puppet-bolt
	zip -j releases/packer-provisioner-puppet-bolt_windows_386.zip   dist/windows_386/packer-provisioner-puppet-bolt.exe
	zip -j releases/packer-provisioner-puppet-bolt_windows_amd64.zip dist/windows_amd64/packer-provisioner-puppet-bolt.exe

clean:
	rm -rf dist/
	rm -f releases/*.zip

.PHONY: default prepare test build install release clean
