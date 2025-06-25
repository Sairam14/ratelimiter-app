# Ratelimiter Application Service

## Overview
This project is a Go web application that demonstrates scalable, distributed **rate limiting** using both in-memory and Redis-backed algorithms. It supports token bucket and leaky bucket strategies, configurable per-user and per-API-key limits, and is designed for high concurrency and horizontal scalability. The app exposes HTTP endpoints for acquiring tokens, checking rate limit status, Prometheus metrics, and includes an admin UI for visualization. It is structured to separate concerns between the command line interface, service logic, and HTTP handlers, making it easy to extend and integrate into real-world API gateways or backend services.

## Architecture Design
The application is organized into three main directories:
- **cmd/**: Contains the entry point of the application. The `main.go` file initializes the application, sets up routes, and starts the HTTP server.
- **pkg/**: Contains the core service logic. The `service.go` file defines a `Service` struct with methods that encapsulate the business logic.
- **internal/**: Contains the HTTP handlers. The `handler.go` file defines functions that handle incoming requests and responses, utilizing the service methods from the `pkg/service` package.

### Trade-offs Made During Development
1. **Simplicity vs. Scalability**: The application is designed to be simple and easy to understand, which may limit its scalability. Future enhancements could include more complex routing and middleware.
2. **Redis as a Data Store**: Using Redis provides fast access to data but may not be suitable for all use cases, especially where complex queries are needed. The choice was made for speed and simplicity in this example.
3. **Error Handling**: Basic error handling is implemented, but more robust error management could be added to improve reliability.

## API Usage Examples

### Start the Application
To start the application, run the following command:
```
docker-compose up --build
```

### Generate a JWT Token for Testing

You can generate a JWT token using [jwt.io](https://jwt.io/):

1. Go to [https://jwt.io/](https://jwt.io/)
2. In the **Payload** section, enter:
   ```json
   {
     "sub": "user123"
   }
   ```
3. Use any secret and algorithm (signature is not verified in this demo).
4. Copy the resulting token, which will look like:
   ```
   eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyMTIzIn0.dummysignature
   ```

### Example API Endpoints

- **POST /api/acquire**  
  Acquire a token for a user/key.  
  Example:
  ```sh
  curl -X POST \
    -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyMTIzIn0.dummysignature" \
    -H "Content-Type: application/json" \
    -d '{"key":"user123"}' \
    http://localhost:8080/api/acquire
  ```

- **GET /api/status?key=user123**  
  Get the current rate limit status for a user/key.  
  Example:
  ```sh
  curl -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyMTIzIn0.dummysignature" \
    "http://localhost:8080/api/status?key=user123"
  ```

- **GET /metrics**  
  Prometheus metrics endpoint.

- **GET /admin**  
  Launches the admin UI in your browser.

## Testing & Validation

### Run Unit Tests
```sh
go test -v ./pkg/service
```

### Run Integration Tests (with Redis)
Start Redis (if not using Docker Compose):
```sh
docker run --rm -p 6379:6379 redis:7
```
Then run:
```sh
go test -v ./pkg/service -run Redis
```

### Check for Race Conditions
```sh
go test -race ./pkg/service
```

### Run Load Tests
```sh
go test -v ./pkg/service -run HighConcurrency
```
Or use [hey](https://github.com/rakyll/hey) or [wrk](https://github.com/wg/wrk) for HTTP load testing:
```sh
hey -n 10000 -c 100 -m POST -H "Authorization: Bearer <JWT>" -H "Content-Type: application/json" -d '{"key":"user123"}' http://localhost:8080/api/acquire
```

### Launch Admin UI
Open your browser and go to:
```
http://localhost:8080/admin
```
You can view user status, Prometheus metrics, and interact with the rate limiter visually.

## Running Benchmarks

You can run the benchmarks defined in `bench_test.go` to measure the performance of the rate limiter algorithms.

### Run all benchmarks in the service package:
```sh
go test -bench=. ./pkg/service
```

### Run with memory allocation stats:
```sh
go test -bench=. -benchmem ./pkg/service
```

### Run a specific benchmark (replace `BenchmarkYourFunc` with the actual function name):
```sh
go test -bench=BenchmarkYourFunc ./pkg/service
```

> **Note:**  
> Benchmarks are only executed for functions that start with `Benchmark` and have the signature `func(b *testing.B)`.  
> Make sure Redis is running if your benchmarks require it.

---