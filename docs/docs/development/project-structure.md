---
sidebar_position: 1
title: Project Structure
---

# Project Structure

This page explains the structure of the GoMFT codebase to help developers understand how the application is organized.

## Overview

GoMFT is built using a combination of Go for the backend and web technologies for the frontend. The application follows a modular architecture to maintain separation of concerns and facilitate testing and maintenance.

## Directory Structure

Here's the high-level directory structure of the GoMFT project:

```
.
├── components/     # Templ components for UI
├── internal/
│   ├── api/        # REST API handlers
│   ├── auth/       # Authentication/authorization
│   ├── config/     # Configuration management
│   ├── db/         # Database models and operations
│   ├── email/      # Email service for notifications and password resets
│   ├── scheduler/  # Job scheduling and execution
│   └── web/        # Web interface handlers
├── static/         # Static assets
│   ├── css/
│   └── js/
└── main.go        # Application entry point
```

## Core Components

### Backend (Go)

GoMFT's backend is written in Go and structured around several key packages:

#### `main.go`

The entry point for the application. It initializes the application, loads configuration, sets up the database, and starts the web server.

#### `internal/`

Contains all internal packages that are not intended to be imported by other applications.

- **`api/`**: REST API implementation
  - `handlers/`: API request handlers
  - `middleware/`: API middleware (authentication, logging, etc.)
  - `routes.go`: API route definitions

- **`auth/`**: Authentication and authorization
  - `providers/`: Authentication providers (local, LDAP, OAuth)
  - `middleware/`: Authentication middleware
  - `rbac/`: Role-based access control

- **`config/`**: Configuration management
  - `config.go`: Application configuration structure
  - `env.go`: Environment variable loading
  - `file.go`: Configuration file loading

- **`db/`**: Database layer
  - `models/`: Database model definitions
  - `migrations/`: Database schema migrations
  - `repositories/`: Data access methods

- **`email/`**: Email functionality
  - `templates/`: Email templates
  - `sender.go`: Email sending service

- **`scheduler/`**: Job scheduling and execution
  - `cron.go`: Cron-based job scheduler
  - `executor.go`: Transfer job executor
  - `queue.go`: Job queue management

- **`web/`**: Web interface
  - `handlers/`: Web request handlers
  - `middleware/`: Web middleware
  - `routes.go`: Web route definitions

#### `components/`

Contains [templ](https://github.com/a-h/templ) components that define the UI. Templ is a Go HTML templating library that provides type-safe templates.

```
components/
├── layouts/       # Page layouts
├── partials/      # Reusable UI components
├── pages/         # Page templates
│   ├── dashboard/
│   ├── transfers/
│   ├── connections/
│   ├── schedules/
│   └── admin/
└── htmx/          # HTMX-specific components
```

### Frontend

The frontend uses a combination of Tailwind CSS for styling and HTMX for dynamic interactions.

#### `static/`

Contains static assets for the web interface:

- **`css/`**: CSS files
  - `main.css`: Main stylesheet (compiled from Tailwind)

- **`js/`**: JavaScript files
  - `htmx.min.js`: HTMX library
  - `alpine.min.js`: Alpine.js for lightweight interactivity
  - `app.js`: Application-specific JavaScript

- **`img/`**: Images and icons

## Build System

GoMFT uses several tools to build and bundle the application:

- **Go Build**: Compiles the Go code
- **Templ**: Compiles templ templates to Go code
- **esbuild**: Bundles JavaScript files
- **Tailwind CSS**: Compiles CSS

The build process is orchestrated by a combination of Go commands and npm scripts defined in `package.json`.

## Configuration Files

- **`.air.toml`**: Configuration for Air, a live reload tool for Go
- **`.env.example`**: Example environment variables configuration
- **`go.mod`**: Go module definition
- **`go.sum`**: Go module checksums
- **`package.json`**: npm package definition for frontend dependencies
- **`Dockerfile`**: Docker container definition
- **`docker-compose.yaml`**: Docker Compose configuration

## Database Structure

GoMFT uses GORM (Go Object Relational Mapper) with SQLite as the default database. The main database models include:

- **`User`**: User account information
- **`Role`**: User roles for RBAC
- **`Permission`**: Individual permissions
- **`Connection`**: File transfer connection configurations
- **`Transfer`**: Transfer definitions
- **`Schedule`**: Transfer schedules
- **`History`**: Transfer execution history
- **`Setting`**: Application settings

## API Structure

The REST API follows a RESTful design with these main endpoints:

- **`/api/auth`**: Authentication endpoints
- **`/api/users`**: User management
- **`/api/connections`**: Connection management
- **`/api/transfers`**: Transfer management
- **`/api/schedules`**: Schedule management
- **`/api/history`**: Transfer history

Each endpoint typically supports standard CRUD operations.

## Web Routes

The web interface is organized around these main routes:

- **`/`**: Dashboard
- **`/connections`**: Connection management
- **`/transfers`**: Transfer management
- **`/schedules`**: Schedule management
- **`/history`**: Transfer history
- **`/admin`**: Administrative functions

## Authentication Flow

The authentication flow in GoMFT works like this:

1. User submits credentials via the login form or API
2. Credentials are validated against the configured authentication provider(s)
3. On success, a session is created for web users or a JWT token is issued for API users
4. The user's permissions are loaded based on their role
5. Requests are then authenticated via session cookie or JWT token

## Transfer Execution Flow

The transfer execution flow is as follows:

1. Transfer job is initiated (manually or via scheduler)
2. Job is added to the execution queue
3. Executor picks up the job and prepares the transfer
4. rclone is invoked with the appropriate parameters
5. Progress is monitored and logged
6. Results are recorded in the history
7. Notifications are sent if configured

## Testing Structure

GoMFT includes several types of tests:

- **Unit Tests**: Test individual functions and methods
- **Integration Tests**: Test interactions between components
- **API Tests**: Test API endpoints
- **End-to-End Tests**: Test complete user flows

Tests are organized alongside the code they're testing, following Go conventions.

## Documentation

Documentation is provided in several formats:

- **Code Comments**: Go doc comments for packages and functions
- **API Documentation**: OpenAPI/Swagger documentation for the REST API
- **User Documentation**: User guides and tutorials (this documentation site)
- **README**: Project overview and quick start instructions 