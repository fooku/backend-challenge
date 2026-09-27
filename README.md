# User Management API

A REST API in Go for managing users. It uses MongoDB for storage, JWT (HS256) for authentication, and bcrypt for password hashing.

## Quick Start

```bash
cp .env.example .env
docker compose up -d --build   # start MongoDB + API on :8080
```

## Design Decisions

### Project Structure

```text
cmd/api/main.go                   wiring, routes, background user counter
internal/config/config.go         loads .env and environment variables
internal/model/user.go            User model
internal/handler/                 HTTP handlers
internal/service/user_service.go  business logic, validation, bcrypt
internal/service/token.go         JWT create and verify (HS256)
internal/repository/              UserInterfaceRepository + MongoDB implementation
internal/middleware/auth.go       JWT middleware
internal/middleware/logger.go     request logging
Dockerfile                        multi-stage build (golang:alpine → alpine, non-root user)
docker-compose.yml                API + MongoDB
```

Every request goes through the same layers:

```text
Handler → Service → Repository → MongoDB
```



### Unique Email

A unique MongoDB index is created for the `email` field.

```text
db.users.createIndex(
  { email: 1 },
  { unique: true }
)
```



### Background User Counter

A background goroutine uses `time.Ticker` to count the total number of users every 10 seconds.

### Password Hashing

Passwords are hashed using `bcrypt` before being stored in MongoDB.

### Validation and Error Handling

Basic validation is performed for required fields, email format, and MongoDB ObjectIDs.

### Testing

Business logic is tested at the service layer using a mock repository

## Requirements

- Docker with Docker Compose, **or**
- Go 1.26+ and MongoDB (installed locally, remote, or started with Docker Compose)



## Run

**Docker Compose** (API + MongoDB):

```bash
cp .env.example .env
docker compose up -d --build   # build and start
docker compose logs -f api     # follow API logs
docker compose down            # stop
docker compose down -v         # stop and delete MongoDB data
```

- The `api` container reads its settings from `.env` (`env_file`), so `.env` must exist.
- `MONGO_URI` is overridden to `mongodb://mongo:27017` inside the container; `localhost` in `.env` is only for running the API on the host.
- The API waits for MongoDB's healthcheck before starting.
- After editing `.env`, run `docker compose up -d` to recreate the container. `docker compose restart` does not reload it.
- `.env` is excluded by `.dockerignore`, so secrets are never baked into the image.

**Local Go** (MongoDB only in Docker):

```bash
cp .env.example .env
docker compose up -d mongo     # start MongoDB only
go run ./cmd/api               # start the API on :8080
```



## Environment Variables


| Variable         | Default                     | Description                                     |
| ---------------- | --------------------------- | ----------------------------------------------- |
| `PORT`           | `8080`                      | HTTP port                                       |
| `MONGO_URI`      | `mongodb://localhost:27017` | MongoDB connection string                       |
| `MONGO_DATABASE` | `user_management`           | Database name                                   |
| `JWT_SECRET`     | *(required)*                | Secret key used to sign and verify HS256 tokens |


```bash
cp .env.example .env
```

`.env` is in `.gitignore`, so the secret isn't committed.

## Run Tests

```bash
go test ./...
```



## API Endpoints


| Method | Path          | Auth | Description                  | Success |
| ------ | ------------- | ---- | ---------------------------- | ------- |
| POST   | `/register`   | No   | Register a new user          | 201     |
| POST   | `/login`      | No   | Log in and get a JWT         | 200     |
| GET    | `/users`      | JWT  | List all users               | 200     |
| GET    | `/users/{id}` | JWT  | Get a user by ID             | 200     |
| PATCH  | `/users/{id}` | JWT  | Update `name` and/or `email` | 200     |
| DELETE | `/users/{id}` | JWT  | Delete a user                | 204     |


Every error response has the same JSON shape:

```json
{ "error": "user not found" }
```


| Status | When                                                        |
| ------ | ----------------------------------------------------------- |
| 400    | Invalid JSON, unknown fields in the body, invalid ObjectID  |
| 401    | Missing, invalid, or expired token; wrong email or password |
| 404    | User not found                                              |
| 409    | Email already exists                                        |
| 500    | Unexpected error                                            |




## Examples



### Register

```bash
curl --request POST \
  --url http://localhost:8080/register \
  --header 'content-type: application/json' \
  --data '{
  "name": "MOCK",
  "email": "mock@example.com",
  "password": "mock-pass"
}'
```

```json
{
  "id": "6ab644634f1783b18f402c5e",
  "name": "MOCK",
  "email": "mock@example.com",
  "created_at": "2026-09-25T09:52:35.662056Z"
}
```

Status: `201`

### Login

```bash
curl --request POST \
  --url http://localhost:8080/login \
  --header 'content-type: application/json' \
  --data '{
  "email": "mock@example.com",
  "password": "mock-pass"
}'
```

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiNmFiNjQ0NjM0ZjE3ODNiMThmNDAyYzVlIiwic3ViIjoiNmFiNjQ0NjM0ZjE3ODNiMThmNDAyYzVlIiwiZXhwIjoxNzkwNDE2NDIwLCJpYXQiOjE3OTAzMzAwMjB9.lHMm2WGnRFd8GTvJ1kIxUWDiEKIyg4-UEi2jO04c3Gg"
}
```

Status: `200`

### Using the JWT Token

The token is signed with HS256 using `JWT_SECRET`. Its payload contains:


| Claim     | Value                     |
| --------- | ------------------------- |
| `user_id` | the user's ID             |
| `sub`     | the user's ID             |
| `iat`     | time the token was issued |
| `exp`     | 24 hours after login      |




### List

```bash
curl http://localhost:8080/users -H "Authorization: Bearer $TOKEN"
```

```json
[
  {
    "id": "6ab644634f1783b18f402c5e",
    "name": "MOCK",
    "email": "mock@example.com",
    "created_at": "2026-09-25T09:52:35.662Z"
  }
]
```

Status: `200`

### Get

```bash
curl http://localhost:8080/users/6ab644634f1783b18f402c5e -H "Authorization: Bearer $TOKEN"
```

```json
{
  "id": "6ab644634f1783b18f402c5e",
  "name": "MOCK",
  "email": "mock@example.com",
  "created_at": "2026-09-25T09:52:35.662Z"
}
```

Status: `200`

### Update

```bash
curl -X PATCH http://localhost:8080/users/6ab644634f1783b18f402c5e \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"new name","email":"new.name@example.com"}'
```

```json
{
  "id": "6ab644634f1783b18f402c5e",
  "name": "new name",
  "email": "new.name@example.com",
  "created_at": "2026-09-25T09:52:35.662Z"
}
```

Status: `200`

### Delete

```bash
curl -X DELETE http://localhost:8080/users/6ab644634f1783b18f402c5e -H "Authorization: Bearer $TOKEN"
```

Status: `204`