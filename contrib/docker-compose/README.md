# Running the API server in Docker with Docker Compose

If you want to run the API server in Docker instead of Kubernetes, you can use the Docker Compose configuration found in this directory.  This is great option if your doing local development and don't want to install Kubernetes.

Firstly you need to start a shell in the `contrib/docker-compose` directory:

```bash
cd contrib/docker-compose
```

Then you need to generate some of the configuration files with:

```bash
./generate-config.sh
```

That wille create the .env and ./volumes directories.  Advanced users can modify the .env file to change the configuration of the API server.

Then you can start the server with:

```bash
docker-compose up -d
```

## Warning: Remote debugging is enabled by default

The apiserver is started with a go debugger listening on port 2345.  You can connect to it with your IDE or a debugger on `localhost:2345`.

To disable the debugger, you can edit the `docker-compose.yml` file and comment out th line that reads: `command: /dlv --continue --listen=:2345 --api-version=2 --only-same-user=false --headless --accept-multiclient exec /apiserver`
