# Repository Guidelines — Rokishi Backend

## 1. Project Context

Rokishi is a management system for 3D printing and production involving different types of machines, locations, materials, and production workflows.

The project consists of two independent repositories:

- `rokishi-back`: REST API developed in Go with PostgreSQL.
- `rokishi-front`: React, TypeScript, Vite, Tailwind, and Cloudflare Kumo.

The backend is deployed to Heroku using a Docker image.

Production infrastructure:

- Heroku Basic Dyno runs the Go API.
- Heroku PostgreSQL Essential-0 stores application data.
- Jenkins runs on a separate Raspberry Pi.
- Jenkins builds Docker images, runs migrations, and deploys the API.
- Cloudflare Pages hosts the frontend.

The development environment may not have access to the Raspberry Pi, Jenkins, Docker, or production PostgreSQL.

Do not assume that local development and production infrastructure are accessible from the current execution environment.

## 2. Project Structure & Module Organization

The API entry point is `cmd/api/main.go`.

Application code lives under `internal/`:

- `internal/config`: environment configuration.
- `internal/database`: PostgreSQL connection pool.
- `internal/http/handlers`: HTTP request handling.
- `internal/http/router`: HTTP route definitions.

Inspect the existing repository before introducing additional packages.

Keep package tests beside their implementation as `*_test.go`.

Database migrations belong in `migrations/` as matching files:

- `NNNNNN_description.up.sql`
- `NNNNNN_description.down.sql`

Deployment files are located at the repository root and under `deploy/jenkins/`.

Preserve the separation between HTTP handling, business rules, and persistence.

Do not introduce an ORM, unnecessary interfaces, or speculative abstractions.

Only add packages when the current functionality requires them.

## 3. Development Principles

Follow these rules throughout the project:

1. Inspect the existing implementation before making changes.
2. Work only on the functionality explicitly requested.
3. Do not automatically implement future phases.
4. Preserve existing naming conventions and architecture.
5. Prefer small, understandable changes.
6. Avoid unrelated refactoring.
7. Do not introduce dependencies without a concrete reason.
8. Do not invent database columns, relationships, or API contracts.
9. Reuse existing code when appropriate.
10. Preserve compatibility with existing clients whenever possible.
11. Keep business rules primarily in the API.
12. Use explicit SQL.
13. Use transactions for operations that modify related records.
14. Prefer deactivation over physical deletion when historical data must be preserved.
15. Use timestamps with time zone where appropriate.
16. Do not introduce PostgreSQL triggers, functions, or views unless explicitly requested.

When a decision significantly affects the architecture or database design, request approval before implementing it.

## 4. Build, Test & Development Commands

The following commands describe the project's development workflow.

Run the commands that are supported by the current execution environment.

### Local development

Run the API:

`go run ./cmd/api`

The application requires `DATABASE_URL`.

`HTTP_ADDR` defaults to `:8081`.

### Go verification

Run the complete test suite:

`go test ./...`

Check concurrent code for data races:

`go test -race ./...`

Run standard static analysis:

`go vet ./...`

Verify compilation:

`go build ./cmd/api`

Format modified Go files using `gofmt`.

### Docker

Build the production-style image:

`docker build -t rokishi-api:local .`

### PostgreSQL migrations

Apply pending migrations:

`migrate -path ./migrations -database "$DATABASE_URL" up`

This command is documentation of the migration workflow. Do not execute it against a remote or production database without explicit authorization.

## 5. Environment Limitations

The production infrastructure runs on a separate Raspberry Pi and Heroku.

The current Codex environment may not have:

- Docker Engine.
- Docker Buildx.
- A running PostgreSQL database.
- Access to Jenkins.
- Access to Heroku.
- Production credentials.
- Network access to required services.

Do not assume these resources are available.

### If Go is available

Execute the relevant Go formatting, tests, static analysis, and compilation commands.

If tests require PostgreSQL and no test database is configured, report that limitation.

Do not connect to production PostgreSQL to compensate for a missing local database.

### If Docker is unavailable

Do not attempt to install or configure Docker unless explicitly requested.

Inspect the Dockerfile and relevant build configuration statically.

Report that container compilation could not be verified.

Provide the appropriate command for manual execution on the Raspberry Pi.

### If PostgreSQL is unavailable

Implement database-dependent functionality using the existing repository conventions.

Use unit tests or controlled test doubles where appropriate.

Do not invent successful integration-test results.

Clearly distinguish tests that passed from tests that could not be executed.

### If network access is unavailable

Do not assume that dependency installation, remote Git operations, or external service requests will succeed.

Report the limitation and continue with the checks that are possible.

Never claim that a verification succeeded without executing it successfully.

## 6. Coding Style & Naming Conventions

Write idiomatic Go.

Use tabs generated by `gofmt`.

Package names should be short, lowercase nouns.

Naming conventions:

- Exported identifiers: `PascalCase`.
- Internal identifiers: `camelCase`.
- Initialisms: `HTTP`, `URL`, `ID`.

Use `context.Context` for operations that require cancellation or deadlines.

Handle errors explicitly.

Return consistent JSON error responses.

Use appropriate HTTP status codes.

Validate incoming data before accessing PostgreSQL.

Do not expose internal database errors directly to API consumers.

Use `log/slog` according to the existing logging conventions.

Prefer straightforward implementations over unnecessary abstraction.

## 7. API Design & Contracts

The API is the source of truth for business rules and data contracts.

Before implementing or modifying an endpoint:

1. Inspect the existing router.
2. Inspect the relevant handlers.
3. Inspect the associated data structures.
4. Inspect database queries and migrations.
5. Identify existing consumers when available.

Preserve existing request and response structures unless a change is explicitly required.

When introducing an endpoint:

- Use appropriate HTTP methods.
- Validate request bodies and path parameters.
- Return consistent JSON.
- Handle missing resources.
- Handle database errors.
- Use appropriate HTTP status codes.
- Document the request and response structure.
- Add relevant tests.

Do not change endpoint paths simply to match frontend naming preferences.

If a breaking API change is unavoidable, explain its impact before implementing it.

## 8. Frontend Integration

The backend is consumed by `rokishi-front`.

When implementing functionality that affects the frontend:

1. Inspect the relevant backend implementation.
2. Inspect the frontend integration if the frontend repository is available.
3. Identify the existing HTTP contract.
4. Determine which changes belong to each repository.
5. Prefer backward-compatible API changes.
6. Document any new or modified request and response structures.
7. Identify whether frontend changes are required.

If the frontend repository is unavailable, do not invent its implementation.

Explain what contract the frontend must consume.

Do not duplicate frontend presentation logic inside the API.

Do not modify frontend code unless the requested task includes frontend changes.

### Independent deployments

The backend and frontend deploy independently.

Avoid changes that require both applications to update simultaneously.

When adding API functionality, preserve compatibility with the currently deployed frontend where practical.

If a feature requires coordinated deployment, identify the required order.

Prefer deploying backward-compatible backend changes before frontend changes that depend on them.

## 9. Database & Migration Safety

PostgreSQL is the production database.

Before changing the schema:

1. Inspect the current migration files.
2. Identify the existing tables and columns.
3. Determine whether a schema change is actually necessary.
4. Explain the proposed modification.
5. Consider compatibility with existing application versions.
6. Identify possible data-loss risks.

Create migrations using the established naming convention.

Each migration must have matching `up` and `down` files.

Do not modify migration files that have already been applied to production.

Create a new migration instead.

Review rollback operations carefully because they may delete data.

### Production safety

Never execute database migrations against production without explicit authorization.

Never reset, truncate, or drop production data without explicit authorization.

Never connect to production PostgreSQL merely to run tests.

Never use production credentials to generate test fixtures.

Do not modify production data as part of development or verification.

### Deployment compatibility

The Jenkins pipeline executes database migrations before deploying the new API image.

Therefore, migrations must be designed with compatibility in mind.

Avoid migrations that immediately remove columns or structures required by the currently deployed API.

When necessary, use an incremental approach:

1. Introduce backward-compatible schema changes.
2. Deploy compatible application code.
3. Migrate application behavior.
4. Remove obsolete structures in a later approved change.

Do not implement all these steps automatically.

Explain the required sequence and wait for authorization.

## 10. Testing Guidelines

Use Go's standard `testing` package.

Use `net/http/httptest` for HTTP handler tests.

No external test framework is required unless explicitly approved.

Name tests using the `TestBehavior` convention.

Prefer table-driven subtests when testing related scenarios.

Cover relevant cases such as:

- Successful requests.
- Invalid request bodies.
- Missing required fields.
- Invalid identifiers.
- Duplicate records.
- Missing records.
- Database failures.
- Transaction failures.
- Context cancellation where relevant.

Add tests for functionality introduced or modified by the current task.

Do not create an elaborate testing framework for a small feature.

### Database-dependent tests

Prefer tests that can run without production services.

If integration tests require PostgreSQL, use a dedicated disposable test database when one is available.

Never point automated tests at production.

If integration tests cannot run, document:

- Which tests could not execute.
- Which dependency was unavailable.
- Which behavior remains unverified.
- How to run the tests in an appropriate environment.

Do not replace integration-test evidence with assumptions.

## 11. Manual Verification on the Raspberry Pi

When the current environment cannot execute infrastructure-dependent checks, provide a short manual verification procedure.

Include only commands relevant to the task.

Typical commands include:

`go test ./...`

`go vet ./...`

`go build ./cmd/api`

`docker build -t rokishi-api:local .`

When relevant, include instructions for testing HTTP endpoints against a local development instance.

Do not instruct the user to run destructive production commands as routine verification.

Distinguish local verification from production deployment.

### Docker architecture

Jenkins runs on a Raspberry Pi, while Heroku requires a compatible AMD64 container image.

The Jenkins pipeline uses Docker Buildx to construct the deployment image.

Do not replace the production build with an ARM-only image.

Preserve the existing Docker image export configuration required by Heroku Container Registry.

Do not modify Docker build flags unless the requested change requires it.

## 12. Jenkins & Deployment

Jenkins is responsible for automated backend deployment.

The existing pipeline includes:

- Repository checkout.
- Go tests.
- Docker image construction.
- Database migrations.
- Heroku image publication.
- HTTP health verification.

Production deployment is restricted to the `main` branch.

Other branches should not deploy to production.

Do not modify deployment behavior unless explicitly requested.

Do not remove tests, migration checks, or health verification to make a failing pipeline pass.

If a deployment fails, identify the failing stage and its actual error before proposing a change.

### Heroku

Heroku runs the Go API using a Basic Dyno.

PostgreSQL is provided through Heroku PostgreSQL Essential-0.

The application receives its database connection through the `DATABASE_URL` environment variable.

The API must listen on the port provided by Heroku through `PORT`.

Preserve the existing configuration that supports both local development and Heroku deployment.

### Health check

The deployed API exposes:

`GET /api/health`

The response reports application and database availability.

Do not remove or rename this endpoint without explicit authorization.

Do not hardcode an assumed Heroku application URL when the deployment configuration already provides the correct address.

## 13. Security & Configuration

Never commit:

- Database connection strings containing credentials.
- Heroku API keys.
- Cloudflare tunnel tokens.
- Passwords.
- Private keys.
- Production secrets.

Use environment variables or the existing Jenkins credentials mechanism.

Do not print secrets in logs.

Do not expose internal database errors through HTTP responses.

Do not introduce privileged access merely to simplify development.

Do not disable authentication, authorization, or security checks without explicit approval.

Never place database credentials or Heroku API keys in frontend environment variables.

Remember that variables prefixed with `VITE_` are exposed to browser clients.

## 14. Git & Collaboration

Use concise Conventional Commit subjects when commits are requested.

Examples:

- `feat: add machine listing endpoint`
- `fix: validate location identifiers`
- `docs: document machine API`
- `chore: update development instructions`

Keep commits focused.

Do not commit generated binaries or temporary files.

Do not commit local environment files containing secrets.

Do not push changes without explicit authorization.

Do not deploy to production without explicit authorization.

When a task affects both repositories, keep their Git histories and commits independent.

Identify changes that must be deployed together or in a particular order.

## 15. Development Workflow

For each requested feature:

1. Inspect the relevant source files.
2. Inspect the database schema when applicable.
3. Inspect the frontend contract when applicable.
4. Summarize the current implementation.
5. Identify problems relevant to the task.
6. Propose a small implementation plan.
7. Identify the files that need modification.
8. Request approval for significant design decisions.
9. Implement only the requested functionality.
10. Add or update relevant tests.
11. Run the checks supported by the environment.
12. Review the final changes.
13. Explain any verification limitations.
14. Summarize the implementation.
15. Stop and wait for the next instruction.

Do not implement future phases simply to prepare the architecture.

## 16. Completion Report

After completing a task, provide a concise report containing:

### Changes

Describe the implemented functionality.

### Modified files

List the relevant files and their purpose.

### Verification

Distinguish between:

- Passed checks.
- Failed checks.
- Checks not executed because the environment does not support them.

Never report unexecuted tests as successful.

### Manual verification

Provide the necessary commands or steps for verification on the Raspberry Pi when applicable.

### Deployment considerations

Identify migrations, new environment variables, compatibility issues, or required deployment order.

### Outstanding decisions

Mention anything that requires approval.

Do not automatically begin the next development phase.