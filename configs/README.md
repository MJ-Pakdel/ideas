# IDAES Configuration Files

This directory contains configuration files used by various components of the IDAES system.

## Configuration Files

### Observability and Monitoring

- **`otel-collector-config.yaml`** - OpenTelemetry Collector configuration
  - **Purpose**: Configures telemetry data collection, processing, and export
  - **Used by**: OpenTelemetry Collector service in Docker Compose
  - **Components configured**:
    - Receivers (OTLP, Jaeger, Zipkin, Prometheus)
    - Processors (batch processing, resource detection)
    - Exporters (Zipkin, Prometheus, logging)
  - **Ports configured**:
    - 4317: OTLP gRPC receiver
    - 4318: OTLP HTTP receiver
    - 14250: Jaeger gRPC receiver
    - 9411: Zipkin receiver
    - 8888: Prometheus metrics endpoint

## File Locations in Docker Containers

### OpenTelemetry Collector
The `otel-collector-config.yaml` file is mounted into the collector container at:
```
/etc/otel-collector-config.yaml
```

This is specified in both `docker-compose.yml` and `docker-compose.dev.yml`:
```yaml
volumes:
  - ${PWD}/configs/otel-collector-config.yaml:/etc/otel-collector-config.yaml
```

## Usage in Development

When modifying these configuration files:

1. **For OpenTelemetry changes**: Edit `configs/otel-collector-config.yaml`
2. **Apply changes**: Restart the Docker services
   ```bash
   # Using scripts
   ./scripts/deploy.sh
   # or
   ./scripts/deploy-dev.sh
   
   # Using make
   make docker-deploy
   # or  
   make docker-dev-up
   ```

## Configuration File Validation

The deployment scripts (`scripts/deploy.sh` and `scripts/deploy-dev.sh`) automatically check for the presence of required configuration files before starting services.

## Adding New Configuration Files

When adding new configuration files to this directory:

1. **Place the file** in the `configs/` directory
2. **Update Docker Compose** files to mount the configuration if needed
3. **Update deployment scripts** to validate the file exists
4. **Document the configuration** in this README
5. **Update the main DOCKER_README.md** if the configuration affects deployment

## Integration with Services

### OpenTelemetry Stack
- **Collector**: Uses `otel-collector-config.yaml` for configuration
- **Zipkin**: Receives traces via collector configuration
- **Prometheus**: May scrape metrics from collector endpoints

### IDAES Application
The IDAES application is configured to send telemetry data to the OpenTelemetry Collector, which then processes and forwards the data according to the configuration in this directory.

## Environment-Specific Configurations

Currently, the same configuration files are used for both development and production deployments. If environment-specific configurations are needed in the future, consider:

- `configs/dev/` - Development-specific configurations
- `configs/prod/` - Production-specific configurations
- Environment variable substitution in configuration files