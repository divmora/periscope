FROM node:26-alpine

WORKDIR /app

# Enable pnpm
RUN npm install -g pnpm

# Install dependencies first for layer caching
COPY package.json pnpm-lock.yaml ./
RUN pnpm install

# Copy all source code
COPY . .

EXPOSE 5173

# Run the Vite dev server, bound to all network interfaces so Docker can expose it
CMD ["pnpm", "dev", "--host"]
