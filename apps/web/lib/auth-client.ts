import { createAuthClient } from "better-auth/react";
import { organizationClient, twoFactorClient } from "better-auth/client/plugins";

export const authClient = createAuthClient({
  baseURL: process.env.NEXT_PUBLIC_APP_URL!,
  plugins: [
    organizationClient(),
    // No onTwoFactorRedirect here: the sign-in form handles the challenge in
    // place rather than navigating away, so the email and password the user
    // just typed are not lost if they mistype the code.
    twoFactorClient(),
  ],
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
      if (
        ctx.response.status === 401 &&
        !path.includes("/sign-in") &&
        !path.includes("/sign-up") &&
        !path.includes("/two-factor/")
      ) {
        window.location.href = "/sign-in";
      }
    },
  },
});
