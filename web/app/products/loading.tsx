// loading.tsx is rendered instantly (via React Suspense) while the Server
// Component above awaits its data — so navigation feels immediate.
export default function Loading() {
  return (
    <main style={{ maxWidth: 1180, margin: "0 auto", padding: "16px 20px 50px" }}>
      {/* Skeleton toolbar */}
      <div
        className="plt-skeleton"
        style={{ height: 20, width: 240, marginBottom: 16 }}
      />
      <div
        style={{
          display: "flex",
          gap: 20,
          alignItems: "flex-start",
          flexWrap: "wrap",
        }}
      >
        {/* Skeleton sidebar */}
        <div
          className="plt-skeleton"
          style={{
            width: 230,
            height: 400,
            flex: "0 0 230px",
            borderRadius: "var(--plt-radius-md)",
          }}
        />
        {/* Skeleton grid */}
        <div style={{ flex: 1, minWidth: 280 }}>
          <div
            className="plt-skeleton"
            style={{ height: 44, marginBottom: 16, borderRadius: "var(--plt-radius-sm)" }}
          />
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "repeat(auto-fill, minmax(190px, 1fr))",
              gap: 16,
            }}
          >
            {Array.from({ length: 8 }).map((_, i) => (
              <div
                key={i}
                className="plt-skeleton"
                style={{
                  height: 280,
                  borderRadius: "var(--plt-radius-md)",
                }}
              />
            ))}
          </div>
        </div>
      </div>
    </main>
  );
}
