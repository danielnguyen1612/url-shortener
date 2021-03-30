# URL Shortener with Golang

It's written based on golang with custom mux router

## How to run
1. Start common service (redis, mariadb) by following command
```bash
docker-compose up
```

2. Install dependencies for module
```bash
go get
```

3. Run service
```bash
go run main.go serve
```

Then the service will be ran on address: http://location:8080. For more configurations, please take a look at `config.yaml` file
