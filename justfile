set dotenv-load

# Install cross-platform tools
build-setup:
    docker run --privileged --rm tonistiigi/binfmt --install all

# init dotenv file
init-dotenv:
    echo "C8Y_HOST=$C8Y_HOST" > .env
    c8y microservices getBootstrapUser --id c8y-devmgmt-repo-intgr | c8y template execute --template "std.join('\n', ['C8Y_BOOTSTRAP_TENANT=' + input.value.tenant, 'C8Y_BOOTSTRAP_USER=' + input.value.name, 'C8Y_BOOTSTRAP_PASSWORD=' + input.value.password])" >> .env

# register application for local development
register:
    c8y microservices create --file ./cumulocity.c8y-devmgmt-repo-intgr.json
    [ ! -f .env ] just init-dotenv

# Start local service
start:
    go run ./cmd/main/main.go

# Build microservice
build *ARGS="": build-setup
    goreleaser build --auto-snapshot --clean {{ARGS}}
    just pack

# Build local microservice instance
build-local *ARGS="": build-setup
    goreleaser build --snapshot --clean {{ARGS}}
    just pack

# Package the Cumulocity Microservice as a zip file
pack:
    ./build/microservice.sh pack --name c8y-devmgmt-repo-intgr --manifest cumulocity.c8y-devmgmt-repo-intgr.json --dockerfile Dockerfile

# Deploy microservice
deploy:
    c8y microservices create --file "./c8y-devmgmt-repo-intgr_$(jq -r .version dist/metadata.json).zip" --name c8y-devmgmt-repo-intgr
