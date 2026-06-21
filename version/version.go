package version

// Version is the build version. It defaults to "dev" for builds from source
// and is overridden at release time by goreleaser via -ldflags -X.
var Version = "dev"
