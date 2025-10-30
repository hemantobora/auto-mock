# Contributing to AutoMock

Thanks for your interest in contributing! This project is early-stage and we welcome issues, feature requests, and PRs.

## Getting Started

- Go 1.22+
- AWS credentials (for infra flows)
- Optional: ANTHROPIC_API_KEY or OPENAI_API_KEY for AI generation

## Build

```bash
# clone
git clone https://github.com/hemantobora/auto-mock.git
cd auto-mock

# build
./build.sh

# run
./automock help
```

## Checks

```bash
# compile
go build ./...

# vet
go vet ./...
```

## Branching
- feature/* for new features
- fix/* for bug fixes

## Pull Requests
- Keep PRs focused and small
- Include a brief description and screenshots/logs where helpful
- Ensure `go build` and `go vet` pass

## License
By contributing, you agree that your contributions will be licensed under the repository’s license.
