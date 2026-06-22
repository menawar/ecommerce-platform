module github.com/menawar/ecommerce-platform/services/cart

go 1.25.0

// Intra-repo modules resolved locally; go.work handles workspace builds.
replace (
	github.com/menawar/ecommerce-platform/pkg => ../../pkg
	github.com/menawar/ecommerce-platform/proto => ../../proto
)

require (
	github.com/alicebob/miniredis/v2 v2.38.0
	github.com/google/uuid v1.6.0
	github.com/menawar/ecommerce-platform/pkg v0.0.0-00010101000000-000000000000
	github.com/menawar/ecommerce-platform/proto v0.0.0-00010101000000-000000000000
	github.com/prometheus/client_golang v1.23.2
	github.com/redis/go-redis/v9 v9.21.0
	golang.org/x/sync v0.21.0
	google.golang.org/grpc v1.81.1
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.66.1 // indirect
	github.com/prometheus/procfs v0.16.1 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/text v0.38.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260226221140-a57be14db171 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
