# Contributing to LobsterHub

Thank you for your interest in contributing to LobsterHub! This document provides guidelines for contributing to the project.

## Table of Contents
- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Pull Request Process](#pull-request-process)
- [Code Style](#code-style)
- [Testing](#testing)
- [Documentation](#documentation)

## Code of Conduct

All contributors are expected to adhere to the project's code of conduct. Please be respectful and constructive in all interactions.

## Getting Started

### Prerequisites
- Go 1.25 or later
- Git
- Docker (optional, for containerized development)

### Setup
1. Fork the repository
2. Clone your fork: `git clone https://github.com/your-username/lobsterhub.git`
3. Navigate to the project directory: `cd lobsterhub`
4. Install dependencies: `go mod tidy`
5. Build the project: `go build -v ./...`
6. Run the project: `./lobsterhub`

## Development Workflow

1. Create a new branch for your feature or bug fix: `git checkout -b feature/your-feature-name`
2. Make your changes
3. Run tests: `go test -v ./...`
4. Run code quality checks: `staticcheck ./...`
5. Commit your changes with a descriptive commit message
6. Push your branch: `git push origin feature/your-feature-name`
7. Create a pull request

## Pull Request Process

1. Ensure your code passes all tests and code quality checks
2. Update the documentation if your changes affect user-facing functionality
3. Add a clear description of your changes in the pull request
4. Reference any related issues in the pull request description
5. Wait for review and address any feedback

## Code Style

- Follow the Go code style guidelines
- Use `go fmt` to format your code
- Use `staticcheck` to check for code quality issues
- Write clear, concise comments

## Testing

- Write unit tests for new functionality
- Ensure existing tests pass
- Test edge cases

## Documentation

- Update the README.md if your changes affect installation or usage
- Add documentation for new features
- Keep API documentation up to date

## Thank You

Your contributions are greatly appreciated! Thank you for helping make LobsterHub better.