---
sidebar_position: 3
title: Release Process
---

# Release Process

This document outlines the process for creating and publishing new releases of GoMFT.

## Version Numbering

GoMFT follows [Semantic Versioning](https://semver.org/) (SemVer) for version numbering:

- **Major version** (X.0.0): Incompatible API changes or significant architectural changes
- **Minor version** (0.X.0): New features added in a backward-compatible manner
- **Patch version** (0.0.X): Backward-compatible bug fixes and minor improvements

## Release Cycle

GoMFT follows a time-based release cycle:

- **Major releases**: Approximately once per year
- **Minor releases**: Every 2-3 months
- **Patch releases**: As needed for bug fixes and security updates

## Release Preparation

### 1. Feature Freeze

One week before a planned release:

- No new features are merged into the main branch
- Only bug fixes, documentation updates, and release preparation are allowed
- All tests must pass on the main branch

### 2. Update Documentation

- Ensure all new features are properly documented
- Update the changelog with all changes since the last release
- Review and update installation and upgrade instructions

### 3. Version Update

Update version numbers in:

- `VERSION` file in the project root
- `package.json` for frontend dependencies
- `internal/version/version.go` for the Go application

### 4. Create Release Branch

For minor and major releases, create a release branch:

```bash
git checkout -b release/vX.Y.Z
```

This branch will be used for final testing and preparation.

### 5. Update Changelog

Update the `CHANGELOG.md` file with all changes since the last release:

```markdown
# Changelog

## [X.Y.Z] - YYYY-MM-DD

### Added
- New feature 1
- New feature 2

### Changed
- Change 1
- Change 2

### Fixed
- Bug fix 1
- Bug fix 2

### Security
- Security fix 1
```

## Release Process

### 1. Final Testing

Perform the following tests on the release branch:

- Run the full test suite
- Test installation from scratch
- Test upgrading from the previous version
- Test all major features manually
- Test on different platforms (Linux, macOS, Windows)

### 2. Create Release Commit

Once testing is complete, commit the version updates:

```bash
git add VERSION package.json internal/version/version.go CHANGELOG.md
git commit -m "Release vX.Y.Z"
```

### 3. Tag the Release

Create an annotated Git tag for the release:

```bash
git tag -a vX.Y.Z -m "Release vX.Y.Z"
```

### 4. Merge to Main

If using a release branch, merge it back to main:

```bash
git checkout main
git merge release/vX.Y.Z
git push origin main
```

### 5. Push the Tag

Push the tag to trigger the CI/CD release pipeline:

```bash
git push origin vX.Y.Z
```

## Release Artifacts

The CI/CD pipeline automatically builds the following artifacts upon tagging:

1. **Docker Images**:
   - `gomft/gomft:vX.Y.Z` - Specific version
   - `gomft/gomft:latest` - Updated for stable releases only

2. **Binary Distributions**:
   - Linux (amd64, arm64)
   - macOS (amd64, arm64)
   - Windows (amd64)

3. **Documentation**:
   - Updated documentation website with the new version

## Post-Release Tasks

### 1. Create GitHub Release

Create a new release on GitHub:

1. Navigate to the repository's "Releases" page
2. Click "Draft a new release"
3. Select the tag you just pushed
4. Title the release "GoMFT vX.Y.Z"
5. Copy the changelog entry for this version
6. Attach the built artifacts
7. Publish the release

### 2. Announce the Release

Announce the new release through:

- Project website
- GitHub Discussions
- Relevant community forums or mailing lists

### 3. Update Demo Environment

Update the demo/staging environment to the new version to showcase the latest features.

### 4. Version Bump for Development

Create a commit on the main branch that bumps the version to the next anticipated version with a `-dev` suffix:

```bash
# Update versions in files to X.Y.(Z+1)-dev
git add VERSION package.json internal/version/version.go
git commit -m "Bump version to vX.Y.(Z+1)-dev"
git push origin main
```

## Hotfix Releases

For critical issues that need immediate fixes:

1. Create a hotfix branch from the release tag:

```bash
git checkout -b hotfix/vX.Y.(Z+1) vX.Y.Z
```

2. Make the necessary fixes

3. Update version numbers and changelog

4. Follow the standard release process from the "Final Testing" step

## Long-Term Support (LTS)

- Major versions may be designated as LTS releases
- LTS releases receive security updates and critical bug fixes for 12 months after the next major version is released
- Only the most recent major version receives new features

## Release Checklist

Use this checklist for each release:

- [ ] All tests pass on the main branch
- [ ] Documentation is up-to-date
- [ ] CHANGELOG.md is updated
- [ ] Version numbers are updated in all files
- [ ] Release branch created (for minor/major versions)
- [ ] Final testing completed successfully
- [ ] Release committed and tagged
- [ ] Tag pushed to trigger build pipeline
- [ ] GitHub release created with changelog and artifacts
- [ ] Release announced to the community
- [ ] Demo environment updated
- [ ] Development version bumped on main branch 