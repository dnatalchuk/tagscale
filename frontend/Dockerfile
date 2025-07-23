# Frontend Dockerfile
FROM node:18-alpine AS builder
WORKDIR /app

# Install dependencies
COPY package.json .
RUN npm install

# Build argument for API URL
ARG REACT_APP_API_URL
ENV REACT_APP_API_URL=$REACT_APP_API_URL

# Copy source code
COPY . .

# Run build
RUN npm run build

FROM node:18-alpine
WORKDIR /app

# Copy built files
COPY --from=builder /app .

# Set runtime env variable
ENV REACT_APP_API_URL=$REACT_APP_API_URL

CMD ["npm", "start"]
