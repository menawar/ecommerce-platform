# Bundles golang-migrate + every service's migration files into one image. The
# Helm chart runs it as a post-install/pre-upgrade hook Job to bring the DBs up to
# date before the app pods start. Built + pushed by the CD workflow on a tag.
FROM migrate/migrate:v4.17.0

# Database-per-service: one migrations tree per service that owns a DB. (cart is
# Redis-only, so it's absent.)
COPY services/user/migrations         /migrations/user
COPY services/product/migrations      /migrations/product
COPY services/payment/migrations      /migrations/payment
COPY services/order/migrations        /migrations/order
COPY services/notification/migrations /migrations/notification

# Drop root: migrate only reads the (world-readable) migration files and connects
# to the DB, so a non-root numeric UID is fine.
USER 65532:65532

# The Job overrides the entrypoint with a shell loop over the databases; this just
# keeps the image runnable standalone for debugging.
ENTRYPOINT ["migrate"]
