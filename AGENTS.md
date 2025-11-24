# Agents

## Golang Code Principles

Principles are in `$HOME/workspace/go-principles/markdown`

### Concurrency Patterns

Patterns can be found in `$HOME/workspace/go-principles/markdown/GO_CONCURRENCY_PATTERNS.md`

## ChromaDB

### v1 API is deprecated
Do not attempt to  access the `v1` REST API.  It is deprecated.  Use the `v2` REST API instead.

### API Format and curl
Please check `chromadb_api/openapi.json` for proper API format when making direct requestsion (e.g. via `curl`)

## Working Hours
When it is after 7:30 PM and I ask for you to do something, offer to create a plan instead of just trying to implement what I am asking.

## Testing
When running `go test ...` always run with the timeout option `go test -timeout 30s` in order to prevent indefinite execution or hanging.

## Deployment

### Deploy to Raspberry Pui

The source code lives in a private GitHub repository. In order to deploy to host, the source code must pushed to the GitHub repo. Then it will be deployed by an Ansible playbook. This is because the target host has an arm64 processor. The development machine is an amd64. Generally follow these steps:

1. Run all unit tests.
2. Run all performance and synchronization tests.
3. Add code to working tree
4. Generate a concise commit message and commit the code
5. Push the code to the repository
6. Change the directory to $HOME/workspace/ansible-fs-vectorize
7. Execute `make fs-vectorize-staging`
8. After a deployment, it is ok to SSH to the host and check the service status. However, I will need to visually inspect the screen to determine the state of the output.