// Cookie names shared by the server code (lib/session, lib/gateway) AND the
// middleware. Kept in their own module — with NO "server-only" — because the
// middleware runs on the Edge runtime and can't import server-only modules.

export const SESSION_COOKIE = "session"; // access token (expires with the token)
export const SESSION_REFRESH_COOKIE = "refresh_session"; // refresh token (7d)

// Match the user service's refresh-token TTL so the cookie doesn't outlive it.
export const REFRESH_MAX_AGE_SECONDS = 7 * 24 * 60 * 60;
