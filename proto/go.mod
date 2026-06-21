// The generated-code module. Holds ONLY buf/protoc output (*.pb.go,
// *_grpc.pb.go) — no hand-written logic. Both pkg and every service may import
// it; it imports only the protobuf/grpc runtime. Keeping generated code in its
// own module means a service depends on the contract, not on sibling services.
module github.com/menawar/ecommerce-platform/proto

go 1.25.0

require (
	google.golang.org/grpc v1.81.1
	google.golang.org/protobuf v1.36.11
)

require (
	golang.org/x/net v0.51.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260226221140-a57be14db171 // indirect
)
