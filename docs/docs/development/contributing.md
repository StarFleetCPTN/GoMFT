---
sidebar_position: 2
title: Contributing
---

# Contributing to GoMFT

Thank you for your interest in contributing to GoMFT! This guide will help you get started with contributing to the project.

## Code of Conduct

By participating in this project, you agree to abide by our Code of Conduct. Please read it before contributing.

## Getting Started

### Prerequisites

Before you begin, ensure you have the following installed:

- **Go** (version 1.20 or later)
- **Node.js** (version 18 or later)
- **Git**
- **Docker** (optional, for container-based development)

### Setting Up the Development Environment

1. Fork the repository on GitHub.

2. Clone your forked repository:

```bash
git clone https://github.com/YOUR_USERNAME/GoMFT.git
cd GoMFT
```

3. Add the original repository as an upstream remote:

```bash
git remote add upstream https://github.com/StarFleetCPTN/GoMFT.git
```

4. Install Go dependencies:

```bash
go mod download
```

5. Install Node.js dependencies:

```bash
npm install
```

6. Install the Templ compiler:

```bash
go install github.com/a-h/templ/cmd/templ@latest
```

7. Install Air for live reloading during development:

```bash
go install github.com/cosmtrek/air@latest
```

### Development Workflow

1. Create a new branch for your feature or bug fix:

```bash
git checkout -b feature/your-feature-name
```

2. Make your changes to the codebase.

3. Compile the Templ templates:

```bash
templ generate
```

4. Run the development server with Air:

```bash
air
```

This will start the application with hot reloading enabled, so changes to Go files will trigger a rebuild.

5. For frontend development, compile the Tailwind CSS and watch for changes:

```bash
npm run dev
```

6. Access the application at `http://localhost:8080`.

## Project Structure

See the [Project Structure](/docs/development/project-structure) page for a detailed overview of the codebase organization.

## Coding Guidelines

### Go Code

- Follow the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments) and [Effective Go](https://golang.org/doc/effective_go) guidelines.
- Format your code with `gofmt` or `go fmt`.
- Ensure your code passes `golint` and `go vet`.
- Write tests for your functionality.
- Add comments to exported functions, types, and packages.

### Frontend Code

- Follow the [Airbnb JavaScript Style Guide](https://github.com/airbnb/javascript) for JavaScript code.
- Use Tailwind CSS for styling.
- Ensure your UI components are responsive.
- Test your UI changes in different browsers.

### Commit Messages

- Use clear and meaningful commit messages.
- Follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:
  - `feat`: A new feature
  - `fix`: A bug fix
  - `docs`: Documentation only changes
  - `style`: Changes that do not affect the meaning of the code
  - `refactor`: A code change that neither fixes a bug nor adds a feature
  - `test`: Adding missing tests or correcting existing tests
  - `chore`: Changes to the build process or auxiliary tools
  - `perf`: Performance improvements

Example: `feat: add email notification for failed transfers`

## Testing

### Running Tests

Run the Go tests:

```bash
go test ./...
```

Run specific tests:

```bash
go test ./internal/api/...
```

### Writing Tests

- Write unit tests for your functions and methods.
- Write integration tests for API endpoints.
- Aim for high test coverage, especially for critical functionality.
- Use table-driven tests where appropriate.

## Pull Request Process

1. Update your branch with the latest changes from upstream:

```bash
git fetch upstream
git rebase upstream/main
```

2. Push your branch to your forked repository:

```bash
git push origin feature/your-feature-name
```

3. Create a pull request from your branch to the main repository.

4. Ensure your PR description clearly describes the changes you've made.

5. Link any relevant issues in your PR description.

6. Wait for code review and address any feedback.

### PR Review Checklist

Before submitting your PR, please ensure:

- [ ] Your code builds without errors or warnings
- [ ] You've added tests for your changes
- [ ] All tests pass
- [ ] Your code follows the project's coding guidelines
- [ ] You've updated documentation as needed
- [ ] You've added appropriate logging
- [ ] You've considered security implications
- [ ] Your changes don't introduce performance regressions

## Development Tips

### Working with Templ

[Templ](https://github.com/a-h/templ) is used for HTML templating in GoMFT. After making changes to `.templ` files, you need to regenerate the Go code:

```bash
templ generate
```

### Working with HTMX

[HTMX](https://htmx.org/) is used for dynamic UI updates. Familiarize yourself with its concepts before making UI changes.

### Working with SQLite

GoMFT uses SQLite for data storage. The database file is located at `data/gomft.db` by default. You can use a tool like [SQLite Browser](https://sqlitebrowser.org/) to inspect the database.

### Debugging

For debugging Go code, you can use:

- `fmt.Printf()` statements for simple debugging
- [Delve](https://github.com/go-delve/delve) for more complex debugging scenarios
- VSCode's integrated Go debugger

## Documentation

### Updating Documentation

Documentation is written in Markdown and stored in the `docs/` directory. To update the documentation:

1. Edit the relevant Markdown files.
2. If you're adding new pages, update the sidebar configuration in `sidebars.ts`.
3. Preview your changes using the documentation development server:

```bash
cd documentation
npm install
npm start
```

4. Access the documentation at `http://localhost:3000`.

### API Documentation

API documentation is generated from Go comments using [Swaggo](https://github.com/swaggo/swag). To update the API documentation:

1. Update the API comments following the Swagger/OpenAPI format.
2. Regenerate the API documentation:

```bash
swag init -g cmd/api/main.go
```

## Release Process

GoMFT follows [Semantic Versioning](https://semver.org/).

### Creating a Release

1. Update the version number in relevant files:
   - `VERSION` file
   - `package.json`
   - `internal/version/version.go`

2. Update the CHANGELOG.md file with the new version and its changes.

3. Create a new tag:

```bash
git tag -a v1.2.3 -m "Release v1.2.3"
git push origin v1.2.3
```

4. The CI/CD pipeline will build and publish the release artifacts.

## Getting Help

If you need help with contributing to GoMFT, you can:

- Open an issue on GitHub with questions
- Discuss in the GitHub Discussions section
- Reach out to the maintainers

Thank you for contributing to GoMFT! 