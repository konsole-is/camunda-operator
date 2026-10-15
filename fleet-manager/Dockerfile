# The fleet manager image. Build it from the root of the repository:
#   docker build -f fleet-manager/Dockerfile .
# Dockerfile.dockerignore limits the context to the app and the images table.

# The release of fleet-manager/.node-version. engines in package.json pins the
# same release, and pnpm refuses to install with another.
FROM node:26.11.1-trixie-slim AS build
ENV NEXT_TELEMETRY_DISABLED=1
# The layout of the repository, so the app finds ../pkg/images/table.json as
# it does in a checkout.
WORKDIR /src/fleet-manager

# Node.js 26 ships no pnpm. npm installs the pnpm release that packageManager
# names, as the Makefile does.
COPY fleet-manager/package.json fleet-manager/pnpm-lock.yaml fleet-manager/pnpm-workspace.yaml ./
RUN npm install --global --allow-scripts=pnpm "pnpm@$(node -p 'require("./package.json").packageManager.replace(/^pnpm@/, "")')" \
 && pnpm install --frozen-lockfile

COPY pkg/images/table.json /src/pkg/images/table.json
COPY fleet-manager/ ./
# The runtime stage copies public/, which the app does not have yet.
RUN mkdir -p public && pnpm run build

FROM gcr.io/distroless/nodejs26-debian13:nonroot
WORKDIR /app
ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    PORT=3000 \
    HOSTNAME=0.0.0.0 \
    FLEET_IMAGES_TABLE=/app/data/images-table.json \
    FLEET_CATALOG=/app/data/official-plugins.json
COPY --from=build /src/fleet-manager/.next/standalone ./
COPY --from=build /src/fleet-manager/.next/static ./.next/static
COPY --from=build /src/fleet-manager/public ./public
COPY pkg/images/table.json /app/data/images-table.json
COPY fleet-manager/catalog/official-plugins.json /app/data/official-plugins.json
USER 65532:65532
EXPOSE 3000
# The distroless entrypoint is node. server.js is the server that next build
# writes for output: "standalone".
CMD ["server.js"]
