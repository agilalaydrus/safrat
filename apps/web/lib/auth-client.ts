import { createAuthClient } from "better-auth/react";
import { organizationClient } from "better-auth/client/plugins";

export const authClient = createAuthClient({
  baseURL: process.env.NEXT_PUBLIC_APP_URL!,
  plugins: [organizationClient()],
  fetchOptions: {
    cache: "no-store",
    onError: async (ctx) => {
      // A 401 on sign-in/sign-up itself means "wrong credentials", not "your
      // session expired" — that case must stay on the page so AuthForm can
      // show its own error message. A hard redirect here would reload the
      // page before React ever renders that message, silently wiping the
      // form with no explanation (the actual UX bug reported: login fails
      // and the user sees nothing).
      const path = new URL(ctx.request.url).pathname;
      if (ctx.response.status === 401 && !path.includes("/sign-in") && !path.includes("/sign-up")) {
        window.location.href = "/sign-in";
      }
    },
  },
});
