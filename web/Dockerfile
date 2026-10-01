FROM node:24-alpine AS build
WORKDIR /app
RUN corepack enable
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY . .
RUN pnpm build
FROM nginx:1.27-alpine
ARG FOODFLOW_SOURCE_DIGEST=unbound
LABEL io.foodflow.source-digest=$FOODFLOW_SOURCE_DIGEST
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
