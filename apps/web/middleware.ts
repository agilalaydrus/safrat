import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

const PUBLIC_PATHS = [
  "/login",
  "/register",
  "/sign-in",
  "/sign-up",
  "/api/auth",
  "/pilgrim",
  "/apply",
  "/firebase-messaging-sw.js",
];

export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;
  const isPublic =
    pathname === "/" || PUBLIC_PATHS.some((path) => pathname.startsWith(path));

  if (isPublic) {
    return NextResponse.next();
  }

  const sessionToken =
    request.cookies.get("better-auth.session_token") ??
    request.cookies.get("__Secure-better-auth.session_token");
  if (!sessionToken) {
    return NextResponse.redirect(new URL("/sign-in", request.url));
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico|icons|manifest.json|sw.js).*)"],
};
