# Use a small Node.js runtime for the test server.
FROM node:22-alpine

# Set the application working directory.
WORKDIR /app

# Copy the manifest before installing production dependencies.
COPY package.json ./
# Install runtime dependencies without generating a lockfile.
RUN npm install --omit=dev --no-package-lock

# Copy the server source into the image.
COPY src ./src

# Start the test server when the container launches.
CMD ["npm", "start"]
