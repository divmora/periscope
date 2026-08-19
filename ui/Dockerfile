FROM node:22-alpine

WORKDIR /app

# Enable pnpm
RUN corepack enable

# Install dependencies first for layer caching
COPY package.json pnpm-lock.yaml ./
RUN pnpm install

# Copy all source code
COPY . .

EXPOSE 5173

# Run the Vite dev server, bound to all network interfaces so Docker can expose it
CMD ["pnpm", "dev", "--host"]
