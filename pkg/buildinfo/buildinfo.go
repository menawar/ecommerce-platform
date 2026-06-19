// Package buildinfo is a trivial placeholder so the shared module compiles
// and the workspace can be verified end-to-end before any real code exists.
// We'll replace/extend this as pkg grows (observability, auth, events, ...).
package buildinfo

// Module is the import path root for the shared module. Handy for log fields
// and a smoke test that the workspace resolves.
const Module = "github.com/menawar/ecommerce-platform/pkg"
