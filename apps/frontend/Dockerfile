FROM node:24-bookworm AS build
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:24-bookworm-slim
WORKDIR /app
ENV NODE_ENV=production HOST=0.0.0.0 PORT=3000 NITRO_HOST=0.0.0.0 NITRO_PORT=3000
COPY --from=build /src/.output ./.output
USER node
EXPOSE 3000
CMD ["node", ".output/server/index.mjs"]
