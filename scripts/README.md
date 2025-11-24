# IDAES Scripts Directory

This directory contains all shell scripts for the IDAES project. All scripts should be executable and can be run from the project root directory.

## Available Scripts

### Deployment Scripts

- **`deploy.sh`** - Production deployment script that builds and starts the full IDAES stack with telemetry
- **`deploy-dev.sh`** - Development deployment script that builds and starts the stack in development mode with live builds

### Maintenance Scripts

- **`health-check.sh`** - Comprehensive health check script that verifies all services are running correctly
- **`setup-ollama-models.sh`** - Sets up and downloads required Ollama models for the LLM functionality

### Testing Scripts

- **`run-e2e-test.sh`** - End-to-end testing script that validates the complete IDAES workflow

## Usage

All scripts can be run from the project root directory:

```bash
# Production deployment
./scripts/deploy.sh

# Development deployment  
./scripts/deploy-dev.sh

# Health check
./scripts/health-check.sh

# Setup Ollama models
./scripts/setup-ollama-models.sh

# Run end-to-end tests
./scripts/run-e2e-test.sh
```

## Makefile Integration

Most of these scripts are also available through Makefile targets:

```bash
make docker-deploy      # Runs ./scripts/deploy.sh
make docker-health      # Runs ./scripts/health-check.sh
make docker-dev-up      # Alternative to ./scripts/deploy-dev.sh
```

## Contributing

When adding new scripts:
1. Make them executable: `chmod +x scripts/your-script.sh`
2. Add appropriate documentation to this README
3. Consider adding a Makefile target if appropriate
4. Test the script from the project root directory