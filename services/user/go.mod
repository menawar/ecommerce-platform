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

require (
	github.com/google/uuid v1.6.0
	github.com/menawar/ecommerce-platform/pkg v0.0.0-00010101000000-000000000000
	github.com/menawar/ecommerce-platform/proto v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.81.1
)

require (
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	golang.org/x/crypto v0.53.0 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/text v0.38.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260226221140-a57be14db171 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
