#!/bin/bash

# Colors
GREEN='\033[0;32m'
NC='\033[0m'

echo -e "${GREEN}🚀 Starting Traffic Generator for Observability Check...${NC}"

# 1. Build the CLI
echo "Building CLI..."
go build -o velocity-cli ./cmd/velocity

# 2. Define the Config Template
# We use a function to regenerate the config easily
create_config() {
  cat <<EOF > velocity.yml
version: 1
project_id: "observability-test"
remote:
  enabled: true
  url: "http://localhost:8080"
  token: "secret123"

pipeline:
  build:
    # FIXED: Create the directory before writing the file
    command: "mkdir -p dist && echo 'Artifact Content' > dist/output.txt"
    inputs: ["velocity.yml"]
    outputs: ["dist"]
    depends_on: []
EOF
}

# 3. Initialize
create_config
mkdir -p dist

# 4. The Loop
COUNT=0
while true; do
  COUNT=$((COUNT+1))
  echo -e "\n[Run $COUNT] -----------------------------"

  # --- SCENARIO A: FORCE CACHE MISS (UPLOAD) ---
  echo ">> Simulating Code Change (Cache Miss)..."
  
  # Modifying the config file changes the input hash
  echo "# Run $COUNT" >> velocity.yml 
  
  # Clean up so the build has to run
  rm -rf .velocity dist
  
  # Run build (This will now succeed and Upload)
  ./velocity-cli run build
  
  # --- SCENARIO B: FORCE CACHE HIT (DOWNLOAD) ---
  echo ">> Simulating CI Pull (Cache Hit)..."
  
  # Clean up local state to force remote download
  rm -rf dist .velocity 
  
  # Run build (This should Download)
  ./velocity-cli run build

  echo ">> Sleeping 1s..."
  sleep 1
done