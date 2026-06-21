module github.com/menawar/ecommerce-platform/services/user

go 1.25.0

// Intra-repo modules. go.work resolves these for workspace builds, but
// `go mod tidy` ignores go.work, so these replaces let tidy (and a standalone
// CI build) find the local pkg/proto modules. Required by these will be filled
// in by tidy as the server code starts importing them.
replace (
	github.com/menawar/ecommerce-platform/pkg => ../../pkg
	github.com/menawar/ecommerce-platform/proto => ../../proto
)
