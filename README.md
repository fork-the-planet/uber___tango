# Tango OSS
Tango OSS (Target Analyzer in Go) provides APIs for fetching and comparing target graphs and services. It is a standalone library that can be used and executed independently of the monorepo.

# Install bazelisk
```
curl -fL https://github.com/bazelbuild/bazelisk/releases/download/v1.27.0/bazelisk-linux-amd64 -o /tmp/bazelisk
chmod +x /tmp/bazelisk
mv /tmp/bazelisk /usr/local/bin/bazelisk
ln -sf /usr/local/bin/bazelisk /usr/local/bin/bazel
which bazel
bazel --version
bazel build //...
```

# Updating BUILD files
- Add all direct GO dependencies explicitly to the MODULE.bazel.
- Update BUILD files by running `bazel run //:gazelle`. If an external dependency is added run `bazel mod tidy` to add the dependency to the repo.

# How to generate/regenerate YARPC-compatible Go structs from .proto files using protoc
Install protoc locally https://github.com/protocolbuffers/protobuf?tab=readme-ov-file#protobuf-compiler-installation.
Installation instructions
```
go install github.com/gogo/protobuf/protoc-gen-gogoslick
go install go.uber.org/yarpc/encoding/protobuf/protoc-gen-yarpc-go
go install go.uber.org/yarpc/encoding/protobuf/protoc-gen-yarpc-go
```
Run the following under `tangopb`:
```
protoc --gogoslick_out=. tango.proto
protoc --yarpc-go_out=. tango.proto
```

# Generating mocks with mockgen
To install mockgen, run `go install go.uber.org/mock/mockgen@latest`.
To regenerate mocks for a package run
```
mockgen -destination=<source file> <package path> <Interface><Interface>
# Run in the current package
mockgen -destination=<source file> . <Interface><Interface>
# Example
mockgen -package=tangopbmock  -self_package=tangopbmock  -destination=tangopbmock/tangopbmock.go . TangoServiceGetChangedServicesYARPCServer,TangoServiceGetChangedTargetGraphYARPCServer,TangoServiceGetChangedTargetsYARPCServer,TangoServiceGetTargetGraphYARPCServer
```

## Update new module version
Run
```
git tag <version>          // git tag 3.0.27.35
git commit -m "Initialize new version"
git push origin <version>	// git push origin 3.0.27.35
```
