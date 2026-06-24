module github.com/menawar/ecommerce-platform/services/gateway

go 1.25.0

// Intra-repo modules resolved locally; go.work handles workspace builds, these
// replaces let `go mod tidy` and standalone CI builds find them.
replace (
	github.com/menawar/ecommerce-platform/pkg => ../../pkg
	github.com/menawar/ecommerce-platform/proto => ../../proto
)

require (
	github.com/go-chi/chi/v5 v5.3.0
	github.com/google/uuid v1.6.0
	github.com/menawar/ecommerce-platform/pkg v0.0.0-00010101000000-000000000000
	github.com/menawar/ecommerce-platform/proto v0.0.0-00010101000000-000000000000
	github.com/prometheus/client_golang v1.23.2
	golang.org/x/sync v0.21.0
	google.golang.org/grpc v1.81.1
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.66.1 // indirect
	github.com/prometheus/procfs v0.16.1 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	golang.org/x/crypto v0.53.0 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/text v0.38.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260226221140-a57be14db171 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
