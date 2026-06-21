// The shared module. Cross-cutting helpers ONLY — no business logic, no
// service-specific code. Services depend on pkg; pkg depends on nothing of
// ours. That one-way arrow is the whole point (see deep-dive: dependency
// direction).
module github.com/menawar/ecommerce-platform/pkg

go 1.25.0

require golang.org/x/crypto v0.53.0
