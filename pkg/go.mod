// The shared module. Cross-cutting helpers ONLY — no business logic, no
// service-specific code. Services depend on pkg; pkg depends on nothing of
// ours. That one-way arrow is the whole point (see deep-dive: dependency
// direction).
module github.com/menawar/ecommerce-platform/pkg

go 1.25.0

require (
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/google/uuid v1.6.0
	golang.org/x/crypto v0.53.0
	google.golang.org/grpc v1.81.1
)

require (
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/text v0.38.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260226221140-a57be14db171 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
